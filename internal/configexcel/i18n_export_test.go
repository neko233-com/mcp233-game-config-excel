package configexcel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExportI18nFullJSONCSVAndTSV(t *testing.T) {
	path := createMultiLanguageExcel(t, t.TempDir(), "I18nTipsConfig.xlsx", [][]string{
		{"welcome", "欢迎", "Welcome"},
		{"network_error", "网络异常", "Network error"},
	})
	jsonDir := filepath.Join(t.TempDir(), "json")
	report, err := ExportI18n(I18nExportOptions{Path: path, OutputDir: jsonDir, Format: I18nExportFormatJSON})
	if err != nil {
		t.Fatalf("export json: %v", err)
	}
	if got, want := len(report.Files), 2; got != want {
		t.Fatalf("json files = %d, want %d", got, want)
	}
	var chinese map[string]string
	jsonPath := filepath.Join(jsonDir, "I18nTipsConfig.CN.json")
	content, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	if err := json.Unmarshal(content, &chinese); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if got, want := chinese["network_error"], "网络异常"; got != want {
		t.Fatalf("Chinese text = %q, want %q", got, want)
	}

	for _, format := range []string{I18nExportFormatCSV, I18nExportFormatTSV} {
		outputDir := filepath.Join(t.TempDir(), format)
		if _, err := ExportI18n(I18nExportOptions{Path: path, OutputDir: outputDir, Format: format}); err != nil {
			t.Fatalf("export %s: %v", format, err)
		}
		content, err := os.ReadFile(filepath.Join(outputDir, "I18nTipsConfig.EN."+format))
		if err != nil {
			t.Fatalf("read %s: %v", format, err)
		}
		if !strings.Contains(string(content), "key") || !strings.Contains(string(content), "Welcome") {
			t.Fatalf("unexpected %s content: %q", format, content)
		}
	}
}

func TestExportI18nIncrementalIncludesUpsertAndDelete(t *testing.T) {
	dir := t.TempDir()
	baseline := createMultiLanguageExcel(t, dir, "baseline.xlsx", [][]string{
		{"welcome", "欢迎", "Welcome"},
		{"removed", "已删除", "Removed"},
	})
	current := createMultiLanguageExcel(t, dir, "current.xlsx", [][]string{
		{"welcome", "欢迎回来", "Welcome"},
		{"new_tip", "新提示", "New tip"},
	})
	outputDir := filepath.Join(dir, "incremental")
	report, err := ExportI18n(I18nExportOptions{
		Path:         current,
		OutputDir:    outputDir,
		Format:       I18nExportFormatJSON,
		Mode:         I18nExportModeDelta,
		BaselinePath: baseline,
	})
	if err != nil {
		t.Fatalf("incremental export: %v", err)
	}
	if got, want := report.Files[0].DeleteCount+report.Files[1].DeleteCount, 2; got != want {
		t.Fatalf("deleted total = %d, want %d", got, want)
	}
	var delta i18nDelta
	content, err := os.ReadFile(filepath.Join(outputDir, "current.CN.json"))
	if err != nil {
		t.Fatalf("read incremental JSON: %v", err)
	}
	if err := json.Unmarshal(content, &delta); err != nil {
		t.Fatalf("decode incremental JSON: %v", err)
	}
	if got, want := delta.Upsert["welcome"], "欢迎回来"; got != want {
		t.Fatalf("changed value = %q, want %q", got, want)
	}
	if got, want := delta.Upsert["new_tip"], "新提示"; got != want {
		t.Fatalf("new value = %q, want %q", got, want)
	}
	if got, want := delta.Delete, []string{"removed"}; !equalStrings(got, want) {
		t.Fatalf("deleted keys = %v, want %v", got, want)
	}
}

func TestExportI18nMultiFieldUsesPrefixedKeys(t *testing.T) {
	path := createCustomI18nExcel(t, t.TempDir(), "multi.xlsx", []string{"id", "name_CN", "tips_CN"}, [][]string{{"hero_1", "小鱼", "出战"}})
	outputDir := filepath.Join(t.TempDir(), "out")
	if _, err := ExportI18n(I18nExportOptions{Path: path, OutputDir: outputDir, Format: I18nExportFormatJSON}); err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	content, err := os.ReadFile(filepath.Join(outputDir, "multi.CN.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &values); err != nil {
		t.Fatal(err)
	}
	if values["hero_1.name"] != "小鱼" || values["hero_1.tips"] != "出战" {
		t.Fatalf("prefixed values = %#v", values)
	}
}

func createMultiLanguageExcel(t *testing.T, dir, name string, data [][]string) string {
	t.Helper()
	return createCustomI18nExcel(t, dir, name, []string{"id", "tips_CN", "tips_EN"}, data)
}

func createCustomI18nExcel(t *testing.T, dir, name string, fields []string, data [][]string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	rows := [][]string{
		append([]string{"注释"}, fields...),
		append([]string{"中文字段名"}, fields...),
		append([]string{"CLIENT"}, fields...),
		append([]string{"TYPE"}, repeatString("string", len(fields))...),
		append([]string{"SERVER"}, fields...),
	}
	for _, dataRow := range data {
		rows = append(rows, append([]string{""}, dataRow...))
	}
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	path := filepath.Join(dir, name)
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func repeatString(value string, count int) []string {
	result := make([]string, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func equalStrings(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}
