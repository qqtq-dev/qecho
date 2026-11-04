package exporter

import (
	"fmt"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"quizbox/engine/internal/store"
)

const resultsSheetName = "成绩"

// ExamResultsXLSX 把一场考试的参与记录导出成成绩单。
// 第一行是考试标题，第二行是表头，数据从第三行开始，便于直接排序和做透视。
func ExamResultsXLSX(exam store.Exam, items []store.AttemptSummary, exportedAt time.Time) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	idx, err := f.NewSheet(resultsSheetName)
	if err != nil {
		return nil, err
	}
	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(idx)

	headers := []string{
		"序号", "姓名", "学号/工号", "状态", "得分",
		"作答", "开始时间", "交卷时间", "用时（分钟）",
	}
	lastCol, err := excelize.ColumnNumberToName(len(headers))
	if err != nil {
		return nil, err
	}

	title := exam.Title + " · 成绩"
	if title == " · 成绩" {
		title = "考试成绩"
	}
	if err := f.SetCellValue(resultsSheetName, "A1", title); err != nil {
		return nil, err
	}
	if err := f.MergeCell(resultsSheetName, "A1", lastCol+"1"); err != nil {
		return nil, err
	}

	titleStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 13, Color: "1F2329"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
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
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	centerStyle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}

	if err := f.SetCellStyle(resultsSheetName, "A1", lastCol+"1", titleStyle); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(resultsSheetName, 1, 30); err != nil {
		return nil, err
	}
	if err := f.SetSheetRow(resultsSheetName, "A2", &headers); err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(resultsSheetName, "A2", lastCol+"2", headStyle); err != nil {
		return nil, err
	}
	if err := f.SetRowHeight(resultsSheetName, 2, 24); err != nil {
		return nil, err
	}

	for i, a := range items {
		row := []any{i + 1, a.Name, a.StudentNo, attemptStatusLabel(a.Status)}

		if a.Score != nil {
			row = append(row, *a.Score)
		} else {
			row = append(row, "")
		}
		row = append(row,
			fmt.Sprintf("%d/%d", a.AnsweredCount, a.TotalCount),
			formatResultTime(a.StartedAt),
			formatResultTime(a.SubmittedAt),
		)
		if mins, ok := usedMinutes(a.StartedAt, a.SubmittedAt); ok {
			row = append(row, mins)
		} else {
			row = append(row, "")
		}

		line := i + 3
		cell := "A" + strconv.Itoa(line)
		if err := f.SetSheetRow(resultsSheetName, cell, &row); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(resultsSheetName, cell, lastCol+strconv.Itoa(line), bodyStyle); err != nil {
			return nil, err
		}
		for _, col := range []string{"A", "D", "E", "F", "I"} {
			c := col + strconv.Itoa(line)
			if err := f.SetCellStyle(resultsSheetName, c, c, centerStyle); err != nil {
				return nil, err
			}
		}
	}

	widths := map[string]float64{
		"A": 6, "B": 14, "C": 16, "D": 10, "E": 8,
		"F": 10, "G": 18, "H": 18, "I": 12,
	}
	for col, w := range widths {
		if err := f.SetColWidth(resultsSheetName, col, col, w); err != nil {
			return nil, err
		}
	}
	if err := f.SetPanes(resultsSheetName, &excelize.Panes{
		Freeze: true, YSplit: 2, TopLeftCell: "A3", ActivePane: "bottomLeft",
	}); err != nil {
		return nil, err
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func attemptStatusLabel(status string) string {
	if status == "submitted" {
		return "已交卷"
	}
	return "答题中"
}

// formatResultTime 把库里存的 RFC3339 时间转成 “2026-11-03 00:55”，认不出就原样返回。
func formatResultTime(raw string) string {
	if raw == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Local().Format("2006-01-02 15:04")
		}
	}
	return raw
}

// usedMinutes 计算交卷用时，未交卷或时间不可解析时返回 false。
func usedMinutes(startedAt, submittedAt string) (int, bool) {
	if startedAt == "" || submittedAt == "" {
		return 0, false
	}
	var start, end time.Time
	var okStart, okEnd bool
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, startedAt); err == nil && !okStart {
			start, okStart = t, true
		}
		if t, err := time.Parse(layout, submittedAt); err == nil && !okEnd {
			end, okEnd = t, true
		}
	}
	if !okStart || !okEnd || end.Before(start) {
		return 0, false
	}
	return int(end.Sub(start).Minutes() + 0.5), true
}
