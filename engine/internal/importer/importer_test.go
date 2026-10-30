package importer

import (
	"strings"
	"testing"
)

func parseCSVText(t *testing.T, content string) *Result {
	t.Helper()
	res, err := ParseWithOptions("test.csv", []byte(content), Options{})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	return res
}

func TestJudgeRowWithEmptyOptionColumns(t *testing.T) {
	res := parseCSVText(t, "题目,题型,选项A,选项B,选项C,选项D,答案,解析,难度,标签\n"+
		"电气火灾可以直接用水扑灭,判断,,,,,错误,水导电,2,消防；应急\n")

	if len(res.Rows) != 1 {
		t.Fatalf("期望 1 行，得到 %d", len(res.Rows))
	}
	row := res.Rows[0]
	if row.Err != "" {
		t.Fatalf("判断题不应报错，实际: %s", row.Err)
	}
	if row.Input.Type != "single" {
		t.Errorf("题型应为 single，实际 %q", row.Input.Type)
	}
	if len(row.Input.Options) != 2 ||
		row.Input.Options[0].Content != "正确" || row.Input.Options[1].Content != "错误" {
		t.Errorf("应合成正确/错误选项，实际 %+v", row.Input.Options)
	}
	if len(row.Input.Answer) != 1 || row.Input.Answer[0] != "B" {
		t.Errorf("答案应为 [B]，实际 %v", row.Input.Answer)
	}
	if row.Input.Difficulty != 2 {
		t.Errorf("难度应为 2，实际 %d", row.Input.Difficulty)
	}
	if strings.Join(row.Input.Tags, "|") != "消防|应急" {
		t.Errorf("标签解析错误: %v", row.Input.Tags)
	}
}

func TestSplitOptionColumnsWinOverMergedColumn(t *testing.T) {
	res := parseCSVText(t, "题目,选项,选项A,选项B,答案\n"+
		"题干一,\"A—甲,B—乙\",甲,乙,A\n")

	joined := strings.Join(res.Warnings, "\n")
	if !strings.Contains(joined, "已按分列选项导入") {
		t.Errorf("应提示按分列选项导入，实际警告: %q", joined)
	}
	row := res.Rows[0]
	if row.Err != "" {
		t.Fatalf("行解析不应报错: %s", row.Err)
	}
	if len(row.Input.Options) != 2 || row.Input.Options[0].Key != "A" || row.Input.Options[0].Content != "甲" {
		t.Errorf("选项应取自 A/B 分列，实际 %+v", row.Input.Options)
	}
}

func TestMergedOptionColumnStillParses(t *testing.T) {
	res := parseCSVText(t, "题目,选项,答案\n"+
		"题干一,\"A—甲\nB—乙\",B\n")

	row := res.Rows[0]
	if row.Err != "" {
		t.Fatalf("合并选项列应可解析: %s", row.Err)
	}
	if len(row.Input.Options) != 2 || row.Input.Options[1].Content != "乙" {
		t.Errorf("合并选项解析错误: %+v", row.Input.Options)
	}
	if len(row.Input.Answer) != 1 || row.Input.Answer[0] != "B" {
		t.Errorf("答案应为 [B]，实际 %v", row.Input.Answer)
	}
}

func TestDuplicateHeaderRejected(t *testing.T) {
	_, err := ParseWithOptions("test.csv", []byte("题目,题干,选项A,选项B,答案\n题干一,重复,A,B,A\n"), Options{})
	if err == nil || !strings.Contains(err.Error(), "未识别到表头") {
		t.Fatalf("重复表头应被拒绝，实际: %v", err)
	}
}
