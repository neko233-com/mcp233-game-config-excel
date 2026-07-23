package configexcel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSearchReplaceAndProjectConfig(t *testing.T) {
	root := t.TempDir()
	excelPath := filepath.Join(root, "nested", "ItemConfig.xlsx")
	if err := os.MkdirAll(filepath.Dir(excelPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CreateI18nTemplate(excelPath, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertRow(excelPath, "", "id", "handbook", map[string]string{"tips_CN": "icon_handbook_sword"}); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "mcp233-game-config-excel.txt")
	if err := os.WriteFile(configPath, []byte("config_dirs = [\"nested\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "~$ItemConfig.xlsx"), []byte("not an xlsx"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Search(SearchOptions{ProjectConfig: configPath, Query: "icon_handbook_.*", Regex: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.FilesScanned != 1 || report.MatchCount != 1 || len(report.UniqueValues) != 1 {
		t.Fatalf("unexpected search report: %+v", report)
	}
	preview, err := Replace(ReplaceOptions{SearchOptions: SearchOptions{ProjectConfig: configPath, Query: "icon_handbook_", Regex: false}, Replacement: "icon_book_"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || preview.ChangeCount != 1 || preview.Changes[0].Before != "icon_handbook_sword" || preview.Changes[0].After != "icon_book_sword" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if err := os.Remove(filepath.Join(root, "nested", "~$ItemConfig.xlsx")); err != nil {
		t.Fatal(err)
	}
	apply, err := Replace(ReplaceOptions{SearchOptions: SearchOptions{ProjectConfig: configPath, Query: "icon_handbook_", Regex: false}, Replacement: "icon_book_", Apply: true})
	if err != nil || !apply.Applied || apply.ChangeCount != 1 {
		t.Fatalf("unexpected apply: %+v, err=%v", apply, err)
	}
	rows, err := ReadRows(excelPath, "", 0)
	if err != nil || rows[1]["tips_CN"] != "icon_book_sword" {
		t.Fatalf("replace did not persist: rows=%+v err=%v", rows, err)
	}
}

func TestDeepValidateAndWriteGuardReportOpenedExcel(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ItemConfig.xlsx")
	if err := CreateI18nTemplate(path, ""); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, "~$ItemConfig.xlsx")
	if err := os.WriteFile(lockPath, []byte("Excel lock marker"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := DeepValidate([]string{root}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.LockedFiles) != 1 || !report.Files[0].Locked {
		t.Fatalf("opened workbook must be reported: %+v", report)
	}
	if _, err := Replace(ReplaceOptions{SearchOptions: SearchOptions{Paths: []string{path}, Query: "示例"}, Replacement: "已修改", Apply: true}); err == nil {
		t.Fatal("opened workbook write must fail")
	}
}

func TestConfigureProjectPreviewsThenWrites(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "BusinessConfig")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "mcp233-game-config-excel.txt")
	preview, err := ConfigureProject(configPath, []string{"BusinessConfig"}, false)
	if err != nil || preview.Applied || preview.Diff == "" {
		t.Fatalf("unexpected preview: %+v err=%v", preview, err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("preview must not write config")
	}
	result, err := ConfigureProject(configPath, []string{"BusinessConfig"}, true)
	if err != nil || !result.Applied {
		t.Fatalf("unexpected write: %+v err=%v", result, err)
	}
	loaded, err := LoadProjectConfig(configPath)
	if err != nil || len(loaded.ConfigDirs) != 1 || loaded.ConfigDirs[0] != directory {
		t.Fatalf("unexpected persisted config: %+v err=%v", loaded, err)
	}
}

func TestCheckTextFormatsDetectsDateRisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "I18nTipsConfig.xlsx")
	if err := CreateI18nTemplate(path, ""); err != nil {
		t.Fatal(err)
	}
	f, err := openWorkbook(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("I18nTipsConfig", "C6", 45292); err != nil {
		t.Fatal(err)
	}
	dateStyle, err := f.NewStyle(&excelize.Style{NumFmt: 14})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellStyle("I18nTipsConfig", "C6", "C6", dateStyle); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	report, err := CheckTextFormats([]string{path}, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.IssueCount != 1 || report.Issues[0].Reason != "字符串字段疑似被 Excel 自动转换为日期" {
		t.Fatalf("unexpected format report: %+v", report)
	}
}
