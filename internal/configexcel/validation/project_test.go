package validation

import (
	"path/filepath"
	"testing"

	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel"
	"github.com/xuri/excelize/v2"
)

func TestCheckIDRequiresExactUniqueID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "I18nTipsConfig.xlsx")
	if err := configexcel.CreateI18nTemplate(path, ""); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("I18nTipsConfig", "B7", "example_tip"); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	issues, err := CheckID(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Value != "example_tip" {
		t.Fatalf("expected duplicate id issue, got %+v", issues)
	}
}

func TestBattleValueUsesRawHundredForDisplayedOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "BattleConfig.xlsx")
	if err := configexcel.CreateI18nTemplate(path, ""); err != nil {
		t.Fatal(err)
	}
	if err := configexcel.AddColumn(path, "", configexcel.ColumnDefinition{Name: "critRate", ClientName: "critRate", Type: "int"}, "tips_CN"); err != nil {
		t.Fatal(err)
	}
	if _, err := configexcel.UpsertRow(path, "", "id", "battle_1", map[string]string{"critRate": "100"}); err != nil {
		t.Fatal(err)
	}
	config := configexcel.DefaultProjectConfig()
	config.BattleAttributePatterns = []string{"(?i)crit"}
	report, err := CheckBattleValues(path, "", config)
	if err != nil {
		t.Fatal(err)
	}
	if report.IssueCount != 0 || report.DisplayRule != "游戏显示值 = 原始整数 / 100；原始值 100 = 显示 1.00" {
		t.Fatalf("unexpected battle report: %+v", report)
	}
	if _, err := configexcel.UpsertRow(path, "", "id", "battle_1", map[string]string{"critRate": "100.5"}); err != nil {
		t.Fatal(err)
	}
	report, err = CheckBattleValues(path, "", config)
	if err != nil || report.IssueCount != 1 {
		t.Fatalf("decimal raw battle value must fail: %+v err=%v", report, err)
	}
}
