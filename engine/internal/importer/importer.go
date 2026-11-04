package importer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"

	"quizbox/engine/internal/store"
)

type Column struct {
	Field  string `json:"field"`
	Header string `json:"header"`
}

type Row struct {
	Number int
	Input  store.QuestionInput
	Err    string
}

type Result struct {
	Format    string
	Sheet     string
	Sheets    []string
	HeaderRow int
	Mapping   map[string]string
	Headers   []string
	Columns   []Column
	Warnings  []string
	Rows      []Row
}

// Options 允许前端覆盖工作表、表头行与列映射后重新解析。
type Options struct {
	Sheet     string
	HeaderRow int
	Mapping   map[string]string
}

const maxRows = 20000

func Parse(fileName string, data []byte) (*Result, error) {
	return ParseWithOptions(fileName, data, Options{})
}

func ParseWithOptions(fileName string, data []byte, opts Options) (*Result, error) {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".xlsx", ".xlsm":
		return parseXLSX(data, opts)
	case ".csv":
		return parseCSV(data, opts)
	case ".xls":
		return nil, fmt.Errorf("不支持旧版 .xls，请在 Excel/WPS 中另存为 .xlsx")
	case ".zip":
		return nil, fmt.Errorf("暂不支持 zip 压缩包导入，请解压后导入其中的 .xlsx/.csv 题目文件")
	default:
		return nil, fmt.Errorf("仅支持 .xlsx / .xlsm / .csv 文件")
	}
}

func parseXLSX(data []byte, opts Options) (*Result, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("无法解析 Excel 文件：%w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	bestSheet := ""
	var bestRows [][]string
	if opts.Sheet != "" {
		rows, err := f.GetRows(opts.Sheet)
		if err != nil || len(rows) == 0 {
			return nil, fmt.Errorf("工作表「%s」不存在或为空", opts.Sheet)
		}
		bestSheet, bestRows = opts.Sheet, rows
	} else {
		for _, name := range sheets {
			rows, err := f.GetRows(name)
			if err != nil || len(rows) == 0 {
				continue
			}
			if bestSheet == "" || len(rows) > len(bestRows) {
				bestSheet, bestRows = name, rows
			}
		}
		for _, name := range sheets {
			if name == "题目" {
				if rows, err := f.GetRows(name); err == nil && len(rows) > 0 {
					bestSheet, bestRows = name, rows
				}
				break
			}
		}
	}
	if bestSheet == "" {
		return nil, fmt.Errorf("工作簿没有可读取的数据")
	}
	res := &Result{Format: "xlsx", Sheet: bestSheet, Sheets: sheets}
	if pics, err := f.GetPictures(bestSheet, "A1:XFD1048576"); err == nil && len(pics) > 0 {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("检测到 %d 张内嵌图片，暂未自动关联到题目", len(pics)))
	}
	if err := parseTable(bestRows, res, opts); err != nil {
		return nil, err
	}
	return res, nil
}

func parseCSV(data []byte, opts Options) (*Result, error) {
	decoded, err := decodeCSV(data)
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(bytes.NewReader(decoded))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV 解析失败：%w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("CSV 文件为空")
	}
	res := &Result{Format: "csv"}
	if err := parseTable(records, res, opts); err != nil {
		return nil, err
	}
	return res, nil
}

func decodeCSV(data []byte) ([]byte, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(data) {
		return data, nil
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
	if err != nil || !utf8.Valid(decoded) {
		return nil, fmt.Errorf("CSV 编码无法识别，请另存为 UTF-8 编码")
	}
	return decoded, nil
}

func parseTable(rows [][]string, res *Result, opts Options) error {
	var header *headerInfo
	var err error
	switch {
	case opts.HeaderRow > 0:
		if opts.HeaderRow > len(rows) {
			return fmt.Errorf("表头行超出文件范围")
		}
		idx := opts.HeaderRow - 1
		if len(opts.Mapping) > 0 {
			header, err = mappingHeader(rows[idx], idx, opts.Mapping)
		} else {
			header, err = mapHeader(rows[idx], idx)
		}
	case len(opts.Mapping) > 0:
		return fmt.Errorf("请同时指定表头行")
	default:
		header, err = findHeader(rows)
	}
	if err != nil {
		return err
	}
	res.HeaderRow = header.index + 1
	res.Headers = rows[header.index]
	res.Columns = header.columns
	res.Mapping = mappingSnapshot(header)
	if header.warning != "" {
		res.Warnings = append(res.Warnings, header.warning)
	}
	if len(header.ignored) > 0 {
		res.Warnings = append(res.Warnings, "已忽略未识别列："+strings.Join(header.ignored, "、"))
	}
	consecutiveEmpty := 0
	for i := header.index + 1; i < len(rows); i++ {
		rec := rows[i]
		if rowEmpty(rec) {
			consecutiveEmpty++
			if consecutiveEmpty >= 20 {
				break
			}
			continue
		}
		consecutiveEmpty = 0
		if len(res.Rows) >= maxRows {
			return fmt.Errorf("数据行超过 %d 行限制", maxRows)
		}
		res.Rows = append(res.Rows, parseRow(rec, header, i+1))
	}
	if len(res.Rows) == 0 {
		return fmt.Errorf("表头下没有数据行")
	}
	return nil
}

type headerInfo struct {
	index      int
	mapping    map[string]int
	optionCols map[string]int
	columns    []Column
	ignored    []string
	warning    string
}

func findHeader(rows [][]string) (*headerInfo, error) {
	var best *headerInfo
	bestScore := -1
	for i := 0; i < len(rows) && i < 10; i++ {
		info, err := mapHeader(rows[i], i)
		if err != nil {
			continue
		}
		_, hasStem := info.mapping["stem"]
		_, hasAnswer := info.mapping["answer"]
		_, hasOptions := info.mapping["options"]
		if !hasStem || !hasAnswer || (!hasOptions && len(info.optionCols) < 2) {
			continue
		}
		score := len(info.mapping) + len(info.optionCols)
		if score > bestScore {
			best, bestScore = info, score
		}
	}
	if best == nil {
		return nil, fmt.Errorf("未识别到表头：请确保包含“题目/题干”“答案”，以及合并的“选项”列或“选项A…选项H”列")
	}
	return best, nil
}

func mappingHeader(cells []string, index int, spec map[string]string) (*headerInfo, error) {
	info := &headerInfo{index: index, mapping: map[string]int{}, optionCols: map[string]int{}}
	for field, rawCol := range spec {
		col := strings.ToUpper(strings.TrimSpace(rawCol))
		if col == "" {
			continue
		}
		idx := columnIndex(col)
		if idx < 0 {
			return nil, fmt.Errorf("列 %s 无效", col)
		}
		if idx >= len(cells) {
			return nil, fmt.Errorf("列 %s 超出表头范围", col)
		}
		if strings.HasPrefix(field, "option_") {
			key := strings.TrimPrefix(field, "option_")
			if len(key) != 1 || key < "A" || key > "H" {
				continue
			}
			info.optionCols[key] = idx
		} else {
			info.mapping[field] = idx
		}
		info.columns = append(info.columns, Column{Field: field, Header: strings.TrimSpace(cells[idx])})
	}
	if _, ok := info.mapping["stem"]; !ok {
		return nil, fmt.Errorf("缺少题干列映射")
	}
	if _, ok := info.mapping["answer"]; !ok {
		return nil, fmt.Errorf("缺少答案列映射")
	}
	if _, ok := info.mapping["options"]; !ok && len(info.optionCols) < 2 {
		return nil, fmt.Errorf("缺少选项列映射：请选择合并选项列，或至少两个分列选项")
	}
	return info, nil
}

func mappingSnapshot(info *headerInfo) map[string]string {
	out := map[string]string{}
	for field, idx := range info.mapping {
		out[field] = columnName(idx)
	}
	for key, idx := range info.optionCols {
		out["option_"+key] = columnName(idx)
	}
	return out
}

func columnIndex(name string) int {
	n := 0
	for _, ch := range strings.ToUpper(strings.TrimSpace(name)) {
		if ch < 'A' || ch > 'Z' {
			return -1
		}
		n = n*26 + int(ch-'A'+1)
	}
	if n == 0 {
		return -1
	}
	return n - 1
}

func columnName(index int) string {
	n := index + 1
	if n <= 0 {
		return ""
	}
	out := ""
	for n > 0 {
		n--
		out = string(rune('A'+n%26)) + out
		n /= 26
	}
	return out
}

func mapHeader(cells []string, index int) (*headerInfo, error) {
	info := &headerInfo{index: index, mapping: map[string]int{}, optionCols: map[string]int{}}
	seen := map[string]string{}
	for i, raw := range cells {
		h := normalizeHeader(raw)
		if h == "" {
			continue
		}
		field := ""
		if f, ok := optionField(h); ok {
			field = f
			info.optionCols[strings.TrimPrefix(f, "option:")] = i
		} else if f, ok := aliasField(h); ok {
			field = f
			info.mapping[f] = i
		}
		if field == "" {
			info.ignored = append(info.ignored, strings.TrimSpace(raw))
			continue
		}
		if prev, dup := seen[field]; dup {
			return nil, fmt.Errorf("表头“%s”与“%s”重复映射到同一字段", prev, strings.TrimSpace(raw))
		}
		seen[field] = strings.TrimSpace(raw)
		info.columns = append(info.columns, Column{Field: field, Header: strings.TrimSpace(raw)})
	}
	if _, ok := info.mapping["options"]; ok && len(info.optionCols) >= 2 {
		delete(info.mapping, "options")
		filtered := info.columns[:0]
		for _, c := range info.columns {
			if c.Field != "options" {
				filtered = append(filtered, c)
			}
		}
		info.columns = filtered
		info.warning = "同时存在合并选项列与分列选项，已按分列选项导入"
	}
	return info, nil
}

var fieldAliases = map[string][]string{
	"type":       {"题型", "类型", "类别", "题目类型", "type", "questiontype"},
	"stem":       {"题干", "题目", "试题", "问题", "question", "stem"},
	"options":    {"选项", "选项内容", "备选项", "options", "choices", "option"},
	"answer":     {"答案", "正确答案", "参考答案", "answer"},
	"analysis":   {"解析", "答案解析", "试题解析", "analysis", "explain"},
	"difficulty": {"难度", "difficulty", "level"},
	"tags":       {"标签", "知识点", "tags"},
	"stemimage":  {"题干图片", "图片", "图片地址", "图片链接", "stemimage", "image"},
	"source":     {"来源", "出处", "source"},
}

func aliasField(h string) (string, bool) {
	for field, aliases := range fieldAliases {
		for _, a := range aliases {
			if h == a {
				return field, true
			}
		}
	}
	return "", false
}

func optionField(h string) (string, bool) {
	if strings.HasPrefix(h, "选项") {
		rest := strings.TrimPrefix(h, "选项")
		if len([]rune(rest)) == 1 {
			r := []rune(rest)[0]
			switch {
			case r >= 'a' && r <= 'h':
				return "option:" + strings.ToUpper(string(r)), true
			case r >= '1' && r <= '8':
				return "option:" + string(rune('A'+r-'1')), true
			}
		}
		return "", false
	}
	if strings.HasPrefix(h, "option") {
		rest := strings.TrimPrefix(h, "option")
		if len(rest) == 1 && rest[0] >= 'a' && rest[0] <= 'h' {
			return "option:" + strings.ToUpper(rest), true
		}
		return "", false
	}
	if len(h) == 1 && h[0] >= 'a' && h[0] <= 'h' {
		return "option:" + strings.ToUpper(h), true
	}
	return "", false
}

var (
	optionPrefix = regexp.MustCompile(`^([A-Ha-h])\s*[—–\-－、.．)）:：]\s*(.*)$`)
	inlineMarker = regexp.MustCompile(`([A-Ha-h])\s*[—–\-－、.．)）:：]`)
)

func parseRow(rec []string, header *headerInfo, number int) Row {
	get := func(field string) string {
		idx, ok := header.mapping[field]
		if !ok || idx >= len(rec) {
			return ""
		}
		return rec[idx]
	}
	row := Row{Number: number}
	var errs []string

	stem := cleanText(get("stem"))
	rawType := cleanText(get("type"))
	typ, judge, typeErr := parseType(rawType)
	if typeErr != nil {
		errs = append(errs, typeErr.Error())
	}
	keys := parseAnswerKeys(get("answer"))
	if len(keys) == 0 && judge {
		keys = parseJudgeAnswer(get("answer"))
	}
	if len(keys) == 0 {
		errs = append(errs, "答案为空")
	}
	if typ == "" && len(keys) > 0 {
		if len(keys) > 1 {
			typ = "multiple"
		} else {
			typ = "single"
		}
	}

	var options []store.QuestionOption
	if len(header.optionCols) >= 2 {
		options, errs = parseOptionColumns(rec, header.optionCols, errs, judge)
	} else if idx, ok := header.mapping["options"]; ok && idx < len(rec) {
		parsed, err := parseCombinedOptions(rec[idx])
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			options = parsed
		}
	} else if !judge {
		errs = append(errs, "缺少选项列")
	}
	if judge && len(options) == 0 {
		options = []store.QuestionOption{{Key: "A", Content: "正确"}, {Key: "B", Content: "错误"}}
	}

	difficulty := 2
	if raw := cleanText(get("difficulty")); raw != "" {
		switch raw {
		case "简单", "容易":
			difficulty = 1
		case "较易":
			difficulty = 2
		case "中等", "一般":
			difficulty = 3
		case "较难":
			difficulty = 4
		case "困难", "难":
			difficulty = 5
		default:
			if n, err := strconv.Atoi(raw); err == nil && n >= 1 && n <= 5 {
				difficulty = n
			}
		}
	}
	image := cleanText(get("stemimage"))
	if image != "" && !strings.HasPrefix(image, "http://") && !strings.HasPrefix(image, "https://") {
		image = filepath.Base(strings.ReplaceAll(image, "\\", "/"))
	}

	input := store.QuestionInput{
		Type:       typ,
		Stem:       stem,
		StemImage:  image,
		Answer:     keys,
		Analysis:   cleanBlock(get("analysis")),
		Source:     cleanText(get("source")),
		Difficulty: difficulty,
		Tags:       splitTags(get("tags")),
		Options:    options,
	}
	if len(errs) == 0 {
		if err := store.ValidateQuestion(&input); err != nil {
			errs = append(errs, err.Error())
		}
	}
	row.Input = input
	row.Err = strings.Join(errs, "；")
	return row
}

func parseOptionColumns(rec []string, cols map[string]int, errs []string, allowEmpty bool) ([]store.QuestionOption, []string) {
	contents := map[string]string{}
	last := -1
	for r := 'A'; r <= 'H'; r++ {
		key := string(r)
		if idx, ok := cols[key]; ok && idx < len(rec) {
			content := cleanBlock(rec[idx])
			contents[key] = content
			if content != "" {
				last = int(r - 'A')
			}
		}
	}
	if last < 0 {
		if allowEmpty {
			return nil, errs
		}
		return nil, append(errs, "有效选项少于 2 个")
	}
	if last < 1 {
		return nil, append(errs, "有效选项少于 2 个")
	}
	options := make([]store.QuestionOption, 0, last+1)
	for i := 0; i <= last; i++ {
		key := string(rune('A' + i))
		content := contents[key]
		if content == "" {
			return nil, append(errs, fmt.Sprintf("选项 %s 为空（选项需从 A 连续填写）", key))
		}
		options = append(options, store.QuestionOption{Key: key, Content: content})
	}
	return options, errs
}

func parseCombinedOptions(raw string) ([]store.QuestionOption, error) {
	block := cleanBlock(raw)
	if block == "" {
		return nil, fmt.Errorf("选项为空")
	}
	lines := strings.Split(block, "\n")
	var options []store.QuestionOption
	expected := byte('A')
	lineBased := true
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if m := optionPrefix.FindStringSubmatch(t); m != nil {
			key := strings.ToUpper(m[1])
			if key[0] != expected {
				return nil, fmt.Errorf("选项键不连续：此处应为 %c", expected)
			}
			options = append(options, store.QuestionOption{
				Key:     key,
				Content: strings.TrimSpace(m[2]),
			})
			expected++
			continue
		}
		if key, content, ok := looseOptionLine(t, expected); ok {
			options = append(options, store.QuestionOption{Key: key, Content: content})
			expected++
			continue
		}
		if len(options) > 0 {
			last := &options[len(options)-1]
			if last.Content != "" {
				last.Content += "\n"
			}
			last.Content += t
			continue
		}
		lineBased = false
		break
	}
	if !lineBased || len(options) < 2 {
		options = nil
		matches := inlineMarker.FindAllStringSubmatchIndex(block, -1)
		if len(matches) < 2 {
			return nil, fmt.Errorf("无法识别选项（需以 A—/B— 等开头，或使用选项A…选项H 分列）")
		}
		for i, m := range matches {
			key := strings.ToUpper(block[m[2]:m[3]])
			start := m[1]
			end := len(block)
			if i+1 < len(matches) {
				end = matches[i+1][0]
			}
			options = append(options, store.QuestionOption{
				Key:     key,
				Content: strings.TrimSpace(block[start:end]),
			})
		}
	}
	seen := map[string]bool{}
	for i := range options {
		key := options[i].Key
		if seen[key] {
			return nil, fmt.Errorf("选项键 %s 重复", key)
		}
		seen[key] = true
		if want := string(rune('A' + i)); key != want {
			return nil, fmt.Errorf("选项键不连续：第 %d 个选项应为 %s", i+1, want)
		}
	}
	if len(options) < 2 {
		return nil, fmt.Errorf("有效选项少于 2 个")
	}
	return options, nil
}

// looseOptionLine 兼容“A 全局索引…”这类只有字母+空格、没有分隔符的写法。
func looseOptionLine(line string, expected byte) (string, string, bool) {
	runes := []rune(line)
	if len(runes) < 2 || runes[0] < 'A' || runes[0] > 'H' || runes[0] != rune(expected) {
		return "", "", false
	}
	if runes[1] != ' ' && runes[1] != '\t' {
		return "", "", false
	}
	content := strings.TrimSpace(string(runes[1:]))
	if content == "" {
		return "", "", false
	}
	return string(runes[0]), content, true
}

func parseType(raw string) (string, bool, error) {
	if raw == "" {
		return "", false, nil
	}
	t := strings.ToLower(strings.ReplaceAll(cleanText(raw), " ", ""))
	switch t {
	case "单选", "单选题", "单项选择", "单项选择题", "single", "radio":
		return "single", false, nil
	case "多选", "多选题", "多项选择", "多项选择题", "不定项", "不定项选择", "multiple", "multi":
		return "multiple", false, nil
	case "判断", "判断题", "是非题", "truefalse", "judge":
		return "single", true, nil
	default:
		return "", false, fmt.Errorf("无法识别题型“%s”", raw)
	}
}

func splitTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	replacer := strings.NewReplacer("，", ",", "、", ",", "；", ",", ";", ",", "；", ",", "\n", ",", "\t", ",")
	parts := strings.Split(replacer.Replace(raw), ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func parseJudgeAnswer(raw string) []string {
	s := strings.ToUpper(strings.Join(strings.Fields(cleanText(raw)), ""))
	switch s {
	case "对", "正确", "是", "√", "T", "TRUE", "Y", "YES", "A":
		return []string{"A"}
	case "错", "错误", "否", "×", "X", "F", "FALSE", "N", "NO", "B":
		return []string{"B"}
	}
	return nil
}

func parseAnswerKeys(raw string) []string {
	s := strings.ToUpper(toHalfWidth(cleanText(raw)))
	replacer := strings.NewReplacer(
		"，", ",", "、", ",", "；", ",", ";", ",", "／", "/", "/", ",",
		" ", "", "\t", "", "\r", "", "\n", "",
	)
	s = replacer.Replace(s)
	var keys []string
	for _, token := range strings.Split(s, ",") {
		if token == "" {
			continue
		}
		if len(token) > 1 && isLetters(strings.ReplaceAll(token, "-", "")) {
			for _, r := range token {
				if r >= 'A' && r <= 'H' {
					keys = append(keys, string(r))
				}
			}
			continue
		}
		if len(token) == 1 && token[0] >= 'A' && token[0] <= 'H' {
			keys = append(keys, token)
		}
	}
	sort.Strings(keys)
	out := keys[:0]
	for _, k := range keys {
		if len(out) == 0 || out[len(out)-1] != k {
			out = append(out, k)
		}
	}
	return out
}

func isLetters(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'H' {
			return false
		}
	}
	return true
}

func rowEmpty(rec []string) bool {
	for _, cell := range rec {
		if cleanText(cell) != "" {
			return false
		}
	}
	return true
}

var zeroWidth = strings.NewReplacer(
	"\u200b", "", "\u200c", "", "\u200d", "", "\ufeff", "",
	"\u00a0", " ", "\u3000", " ",
)

func cleanText(s string) string {
	s = zeroWidth.Replace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

func cleanBlock(s string) string {
	s = cleanText(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func normalizeHeader(s string) string {
	s = toHalfWidth(cleanText(s))
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), "")
	return s
}

func toHalfWidth(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\u3000':
			b.WriteRune(' ')
		case r >= 0xFF01 && r <= 0xFF5E:
			b.WriteRune(r - 0xFEE0)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
