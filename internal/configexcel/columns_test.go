package configexcel

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestAddColumnPreservesStyleAndUsesTextFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "I18nTipsConfig.xlsx")
	if err := CreateI18nTemplate(path, ""); err != nil {
		t.Fatal(err)
	}
	f, err := openWorkbook(path)
	if err != nil {
		t.Fatal(err)
	}
	styleID, err := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"#ABCDEF"}, Pattern: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellStyle("I18nTipsConfig", "C1", "C6", styleID); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	err = AddColumn(path, "", ColumnDefinition{
		Name: "handbookIconPath", ClientName: "handbookIconPath", DisplayName: "图鉴显示的武器 icon", Type: "string", Comment: "图鉴显示的武器 icon",
	}, "tips_CN")
	if err != nil {
		t.Fatal(err)
	}

	inspection, err := Inspect(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(inspection.Columns), 3; got != want {
		t.Fatalf("column count = %d, want %d", got, want)
	}
	if got, want := inspection.Columns[2].Name, "handbookIconPath"; got != want {
		t.Fatalf("column name = %q, want %q", got, want)
	}
	check, err := CheckColumnFormat(path, "", "handbookIconPath", true)
	if err != nil {
		t.Fatal(err)
	}
	if !check.IsTextFormat || len(check.Issues) != 0 {
		t.Fatalf("column format check = %+v", check)
	}
	f, err = openWorkbook(path)
	if err != nil {
		t.Fatal(err)
	}
	styleID, err = f.GetCellStyle("I18nTipsConfig", "D1")
	if err != nil {
		t.Fatal(err)
	}
	style, err := f.GetStyle(styleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(style.Fill.Color) != 1 || style.Fill.Color[0] != "ABCDEF" {
		t.Fatalf("copied fill = %+v", style.Fill)
	}
	displayName, err := f.GetCellValue("I18nTipsConfig", "D2")
	if err != nil {
		t.Fatal(err)
	}
	if displayName != "图鉴显示的武器 icon" {
		t.Fatalf("display name = %q", displayName)
	}
	_ = f.Close()
}

func TestDeleteColumnRemovesConfig233Field(t *testing.T) {
	path := filepath.Join(t.TempDir(), "I18nTipsConfig.xlsx")
	if err := CreateI18nTemplate(path, ""); err != nil {
		t.Fatal(err)
	}
	if err := AddColumn(path, "", ColumnDefinition{Name: "extra", Type: "string"}, "id"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteColumn(path, "", "extra"); err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(inspection.Columns), 2; got != want {
		t.Fatalf("column count = %d, want %d", got, want)
	}
	if inspection.Columns[0].Name != "id" || inspection.Columns[1].Name != "tips_CN" {
		t.Fatalf("unexpected columns: %+v", inspection.Columns)
	}
}

func TestUpsertKeepsPhysicalExcelColumnWhenServerColumnIsSparse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "I18nTipsConfig.xlsx")
	if err := CreateI18nTemplate(path, ""); err != nil {
		t.Fatal(err)
	}
	f, err := openWorkbook(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("I18nTipsConfig", "D3", "serverValue"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("I18nTipsConfig", "D4", "string"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("I18nTipsConfig", "D5", "serverValue"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("I18nTipsConfig", "D6", "before"); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	if _, err := UpsertRow(path, "", "id", "example_tip", map[string]string{"serverValue": "after"}); err != nil {
		t.Fatal(err)
	}
	f, err = openWorkbook(path)
	if err != nil {
		t.Fatal(err)
	}
	value, err := f.GetCellValue("I18nTipsConfig", "D6")
	if err != nil {
		t.Fatal(err)
	}
	if value != "after" {
		t.Fatalf("physical column value = %q, want after", value)
	}
	wrongValue, err := f.GetCellValue("I18nTipsConfig", "C6")
	if err != nil {
		t.Fatal(err)
	}
	if wrongValue != "示例提示" {
		t.Fatalf("sparse column value = %q, want original value", wrongValue)
	}
	_ = f.Close()
}
