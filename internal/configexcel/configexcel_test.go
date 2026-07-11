package configexcel

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestI18nTemplateInspectValidateReadAndUpsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "I18nTipsConfig.xlsx")
	if err := CreateI18nTemplate(path, ""); err != nil {
		t.Fatalf("create template: %v", err)
	}

	inspection, err := Inspect(path, "")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if got, want := inspection.Sheet, "I18nTipsConfig"; got != want {
		t.Fatalf("sheet = %q, want %q", got, want)
	}
	if got, want := inspection.DataRows, 1; got != want {
		t.Fatalf("data rows = %d, want %d", got, want)
	}
	if got := []string{inspection.Columns[0].Name, inspection.Columns[1].Name}; !reflect.DeepEqual(got, []string{"id", "tips_CN"}) {
		t.Fatalf("server columns = %v", got)
	}

	validation, err := Validate(path, "", []string{"id", "tips_CN"}, "id")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("template should validate: %+v", validation)
	}

	created, err := UpsertRow(path, "", "id", "network_error", map[string]string{"tips_CN": "网络异常，请重试"})
	if err != nil || !created {
		t.Fatalf("create row: created=%v err=%v", created, err)
	}
	created, err = UpsertRow(path, "", "id", "network_error", map[string]string{"tips_CN": "网络连接异常"})
	if err != nil || created {
		t.Fatalf("update row: created=%v err=%v", created, err)
	}
	rows, err := ReadRows(path, "", 0)
	if err != nil {
		t.Fatalf("read rows: %v", err)
	}
	if got, want := len(rows), 2; got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}
	if got, want := rows[1]["tips_CN"], "网络连接异常"; got != want {
		t.Fatalf("updated value = %q, want %q", got, want)
	}
}

func TestValidateRejectsDuplicateUID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "I18nTipsConfig.xlsx")
	if err := CreateI18nTemplate(path, ""); err != nil {
		t.Fatal(err)
	}
	// Direct write retains a valid Excel workbook but introduces invalid config data.
	f, err := openWorkbook(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("I18nTipsConfig", "B7", "example_tip"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("I18nTipsConfig", "C7", "重复"); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	result, err := Validate(path, "", nil, "id")
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid {
		t.Fatalf("duplicate id must fail: %+v", result)
	}
}
