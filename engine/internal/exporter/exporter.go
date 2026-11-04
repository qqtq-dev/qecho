// Package exporter 把题库题目导出为可再次导入的 Excel 文件。
package exporter

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"quizbox/engine/internal/store"
)

const sheetName = "题目"

// BankXLSX 生成与导入模板列名一致的 .xlsx 内容。
func BankXLSX(questions []store.Question) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	idx, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(idx)

	hasImage := false
	maxOptions := 2
	for _, q := range questions {
		if q.StemImage != "" {
			hasImage = true
		}
		if len(q.Options) > maxOptions {
			maxOptions = len(q.Options)
		}
	}

	headers := []string{"题型", "题目"}
	if hasImage {
		headers = append(headers, "题干图片")
	}
	for i := 0; i < maxOptions; i++ {
		headers = append(headers, "选项"+string(rune('A'+i)))
	}
	headers = append(headers, "答案", "解析", "标签", "难度", "来源")
	if err := f.SetSheetRow(sheetName, "A1", &headers); err != nil {
		return nil, err
	}

	headStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "1F5C43"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E6F2EC"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	bodyStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	if err != nil {
		return nil, err
	}
	centerStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "top"},
	})
	if err != nil {
		return nil, err
	}
	headCount := len(headers)
	lastCol, _ := excelize.ColumnNumberToName(headCount)
	if err := f.SetCellStyle(sheetName, "A1", lastCol+"1", headStyle); err != nil {
		return nil, err
	}

	optIndex := map[string]int{}
	for i := 0; i < maxOptions; i++ {
		optIndex[string(rune('A'+i))] = i
	}
	answerCol, _ := excelize.ColumnNumberToName(headCount - 4)

	for n, q := range questions {
		row := []any{typeLabel(q.Type), q.Stem}
		if hasImage {
			row = append(row, q.StemImage)
		}
		contents := make([]string, maxOptions)
		for _, o := range q.Options {
			if i, ok := optIndex[o.Key]; ok {
				contents[i] = o.Content
			}
		}
		for _, c := range contents {
			row = append(row, c)
		}
		row = append(row, strings.Join(q.Answer, ""), q.Analysis,
			strings.Join(q.Tags, "；"), q.Difficulty, q.Source)

		line := n + 2
		cell := fmt.Sprintf("A%d", line)
		if err := f.SetSheetRow(sheetName, cell, &row); err != nil {
			return nil, err
		}
		last := fmt.Sprintf("%s%d", lastCol, line)
		if err := f.SetCellStyle(sheetName, cell, last, bodyStyle); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(sheetName, fmt.Sprintf("%s%d", answerCol, line),
			fmt.Sprintf("%s%d", answerCol, line), centerStyle); err != nil {
			return nil, err
		}
		lines := 1
		for _, text := range []string{q.Stem, q.Analysis, strings.Join(contents, "\n")} {
			if n := wrapLines(text, 60); n > lines {
				lines = n
			}
		}
		height := math.Min(float64(15*lines+6), 150)
		_ = f.SetRowHeight(sheetName, line, height)
	}

	widths := map[string]float64{"A": 8}
	stemCol := "B"
	widths[stemCol] = 52
	if hasImage {
		widths["C"] = 34
	}
	optionStart := 3
	if hasImage {
		optionStart = 4
	}
	for i := 0; i < maxOptions; i++ {
		name, _ := excelize.ColumnNumberToName(optionStart + i)
		widths[name] = 30
	}
	widths[answerCol] = 8
	analysisCol, _ := excelize.ColumnNumberToName(headCount - 3)
	widths[analysisCol] = 40
	tagsCol, _ := excelize.ColumnNumberToName(headCount - 2)
	widths[tagsCol] = 22
	difficultyCol, _ := excelize.ColumnNumberToName(headCount - 1)
	widths[difficultyCol] = 8
	widths[lastCol] = 24
	for col, width := range widths {
		_ = f.SetColWidth(sheetName, col, col, width)
	}

	_ = f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
	if len(questions) > 0 {
		_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:%s%d", lastCol, len(questions)+1), nil)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// TemplateXLSX 生成供用户填写的标准导入模板，含两行示例数据。
func TemplateXLSX() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	idx, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(idx)

	headers := []string{
		"题型", "题目", "题干图片", "选项A", "选项B", "选项C", "选项D", "选项E",
		"答案", "解析", "难度", "标签", "来源",
	}
	if err := f.SetSheetRow(sheetName, "A1", &headers); err != nil {
		return nil, err
	}
	rows := [][]any{
		{"单选题", "示例：以下哪一项属于绿色出行方式？", "", "乘坐公共交通", "驾驶燃油车",
			"焚烧垃圾", "随意丢弃电池", "", "A", "示例解析，可留空", 2, "示例；基础", "示例题库"},
		{"多选题", "示例：以下哪些属于可回收物？", "", "纸箱", "塑料瓶", "果皮",
			"金属罐", "", "ABD", "示例解析，可留空", 2, "示例；环保", "示例题库"},
	}
	for i, row := range rows {
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(sheetName, cell, &row); err != nil {
			return nil, err
		}
	}

	headStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "1F5C43"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E6F2EC"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	lastCol, _ := excelize.ColumnNumberToName(len(headers))
	if err := f.SetCellStyle(sheetName, "A1", lastCol+"1", headStyle); err != nil {
		return nil, err
	}
	bodyStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheetName, "A2", fmt.Sprintf("%s%d", lastCol, len(rows)+1), bodyStyle); err != nil {
		return nil, err
	}
	centerStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "top"},
	})
	if err != nil {
		return nil, err
	}
	answerCol, _ := excelize.ColumnNumberToName(9)
	if err := f.SetCellStyle(sheetName, answerCol+"2", fmt.Sprintf("%s%d", answerCol, len(rows)+1), centerStyle); err != nil {
		return nil, err
	}

	widths := map[string]float64{"A": 8, "B": 52, "C": 24, "D": 24, "E": 24, "F": 24, "G": 24,
		"H": 24, "I": 8, "J": 36, "K": 8, "L": 22, "M": 24}
	for col, width := range widths {
		_ = f.SetColWidth(sheetName, col, col, width)
	}
	for i := range rows {
		_ = f.SetRowHeight(sheetName, i+2, 36)
	}
	_ = f.SetPanes(sheetName, &excelize.Panes{
		Freeze:      true,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func typeLabel(t string) string {
	if t == "multiple" {
		return "多选"
	}
	return "单选"
}

func wrapLines(text string, perLine int) int {
	if text == "" {
		return 1
	}
	lines := 0
	for _, part := range strings.Split(text, "\n") {
		n := utf8.RuneCountInString(part)
		if n == 0 {
			lines++
			continue
		}
		lines += (n + perLine - 1) / perLine
	}
	return lines
}
