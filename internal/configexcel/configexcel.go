// Package configexcel reads and writes config233-compatible Excel workbooks.
package configexcel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	clientRowNumber = 3
	typeRowNumber   = 4
	serverRowNumber = 5
	dataRowNumber   = 6
)

// Column is one config233 field, sourced from its CLIENT, TYPE and SERVER rows.
type Column struct {
	Name       string `json:"name"`
	ClientName string `json:"clientName"`
	Type       string `json:"type"`
}

// Inspection describes Excel format without exposing arbitrary workbook metadata.
type Inspection struct {
	FilePath string   `json:"filePath"`
	Sheet    string   `json:"sheet"`
	Columns  []Column `json:"columns"`
	DataRows int      `json:"dataRows"`
}

// ValidationResult is deterministic and safe to consume from a CLI or MCP client.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
}

// ResolveSheet selects requested sheet or workbook's first sheet.
func ResolveSheet(f *excelize.File, requested string) (string, error) {
	if requested != "" {
		index, err := f.GetSheetIndex(requested)
		if err != nil || index == -1 {
			return "", fmt.Errorf("sheet not found: %s", requested)
		}
		return requested, nil
	}
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "", fmt.Errorf("workbook has no sheets")
	}
	return sheets[0], nil
}

// Inspect verifies fixed config233 header layout and returns server fields.
func Inspect(path, requestedSheet string) (Inspection, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return Inspection{}, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	sheet, err := ResolveSheet(f, requestedSheet)
	if err != nil {
		return Inspection{}, err
	}
	columns, err := readColumns(f, sheet)
	if err != nil {
		return Inspection{}, err
	}
	rows, err := ReadRows(path, sheet, 0)
	if err != nil {
		return Inspection{}, err
	}
	return Inspection{FilePath: filepath.Clean(path), Sheet: sheet, Columns: columns, DataRows: len(rows)}, nil
}

// ReadRows reads data rows only. All values remain strings so Excel config text is lossless.
func ReadRows(path, requestedSheet string, limit int) ([]map[string]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	sheet, err := ResolveSheet(f, requestedSheet)
	if err != nil {
		return nil, err
	}
	columns, err := readColumns(f, sheet)
	if err != nil {
		return nil, err
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}
	result := make([]map[string]string, 0)
	for rowIndex := dataRowNumber - 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		item := make(map[string]string)
		nonEmpty := false
		for columnIndex, column := range columns {
			value := ""
			cellIndex := columnIndex + 1 // config233 reserves column A as marker.
			if cellIndex < len(row) {
				value = row[cellIndex]
			}
			if strings.TrimSpace(value) != "" {
				nonEmpty = true
			}
			item[column.Name] = value
		}
		if nonEmpty {
			result = append(result, item)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// Validate checks config233 layout, exact expected server fields when given, and UID uniqueness.
func Validate(path, requestedSheet string, expectedColumns []string, uidColumn string) (ValidationResult, error) {
	if uidColumn == "" {
		uidColumn = "id"
	}
	result := ValidationResult{Valid: true, Issues: []string{}, Warnings: []string{}}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return result, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	sheet, err := ResolveSheet(f, requestedSheet)
	if err != nil {
		return result, err
	}
	columns, err := readColumns(f, sheet)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, err.Error())
		return result, nil
	}
	if marker, _ := f.GetCellValue(sheet, "A5"); !strings.EqualFold(strings.TrimSpace(marker), "SERVER") {
		result.Valid = false
		result.Issues = append(result.Issues, "A5 must equal SERVER")
	}
	seenColumns := map[string]bool{}
	actualColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		actualColumns = append(actualColumns, column.Name)
		if seenColumns[column.Name] {
			result.Valid = false
			result.Issues = append(result.Issues, "duplicate server column: "+column.Name)
		}
		seenColumns[column.Name] = true
		if column.Type == "" {
			result.Warnings = append(result.Warnings, "empty type for column: "+column.Name)
		}
	}
	if len(expectedColumns) > 0 && !sameColumns(actualColumns, expectedColumns) {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("server columns mismatch: actual=%v expected=%v", actualColumns, expectedColumns))
	}
	if !seenColumns[uidColumn] {
		result.Valid = false
		result.Issues = append(result.Issues, "uid column not found: "+uidColumn)
		return result, nil
	}
	rows, err := ReadRows(path, sheet, 0)
	if err != nil {
		return result, err
	}
	seenUIDs := map[string]bool{}
	for rowNumber, row := range rows {
		uid := strings.TrimSpace(row[uidColumn])
		if uid == "" {
			result.Valid = false
			result.Issues = append(result.Issues, fmt.Sprintf("empty %s in data row %d", uidColumn, rowNumber+dataRowNumber))
			continue
		}
		if seenUIDs[uid] {
			result.Valid = false
			result.Issues = append(result.Issues, fmt.Sprintf("duplicate %s: %s", uidColumn, uid))
		}
		seenUIDs[uid] = true
	}
	return result, nil
}

// UpsertRow updates one data row identified by uidColumn. Missing row is appended.
func UpsertRow(path, requestedSheet, uidColumn, uid string, values map[string]string) (bool, error) {
	if strings.TrimSpace(uidColumn) == "" {
		uidColumn = "id"
	}
	if strings.TrimSpace(uid) == "" {
		return false, fmt.Errorf("uid is required")
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return false, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	sheet, err := ResolveSheet(f, requestedSheet)
	if err != nil {
		return false, err
	}
	columns, err := readColumns(f, sheet)
	if err != nil {
		return false, err
	}
	columnIndexes := map[string]int{}
	for index, column := range columns {
		columnIndexes[column.Name] = index + 2 // Excel column number, with A marker.
	}
	uidIndex, found := columnIndexes[uidColumn]
	if !found {
		return false, fmt.Errorf("uid column not found: %s", uidColumn)
	}
	for name := range values {
		if _, found := columnIndexes[name]; !found {
			return false, fmt.Errorf("unknown server column: %s", name)
		}
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return false, fmt.Errorf("read rows: %w", err)
	}
	targetRow := 0
	for rowNumber := dataRowNumber; rowNumber <= len(rows); rowNumber++ {
		cell, cellErr := excelize.CoordinatesToCellName(uidIndex, rowNumber)
		if cellErr != nil {
			return false, cellErr
		}
		current, valueErr := f.GetCellValue(sheet, cell)
		if valueErr != nil {
			return false, valueErr
		}
		if strings.TrimSpace(current) == uid {
			targetRow = rowNumber
			break
		}
	}
	created := targetRow == 0
	if created {
		targetRow = len(rows) + 1
		if targetRow < dataRowNumber {
			targetRow = dataRowNumber
		}
	}
	values = cloneValues(values)
	values[uidColumn] = uid
	for name, value := range values {
		cell, cellErr := excelize.CoordinatesToCellName(columnIndexes[name], targetRow)
		if cellErr != nil {
			return false, cellErr
		}
		if setErr := f.SetCellValue(sheet, cell, value); setErr != nil {
			return false, setErr
		}
	}
	if err := f.Save(); err != nil {
		return false, fmt.Errorf("save workbook: %w", err)
	}
	return created, nil
}

// CreateI18nTemplate writes an I18nTipsConfig-compatible sheet.
func CreateI18nTemplate(path, requestedSheet string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return fmt.Errorf("create parent directory: %w", err)
	}
	f := excelize.NewFile()
	defer f.Close()
	sheet := requestedSheet
	if sheet == "" {
		sheet = "I18nTipsConfig"
	}
	defaultSheet := f.GetSheetName(0)
	if err := f.SetSheetName(defaultSheet, sheet); err != nil {
		return err
	}
	rows := [][]string{
		{"注释", "唯一 id", "中文提示"},
		{"中文字段名", "唯一id", "中文提示"},
		{"CLIENT", "id", "tips_CN"},
		{"TYPE", "string", "string"},
		{"SERVER", "id", "tips_CN"},
		{"", "example_tip", "示例提示"},
	}
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return err
			}
		}
	}
	f.SetColWidth(sheet, "A", "A", 12)
	f.SetColWidth(sheet, "B", "B", 28)
	f.SetColWidth(sheet, "C", "C", 48)
	return f.SaveAs(path)
}

// JSONRows creates stable pretty JSON for CLI and MCP responses.
func JSONRows(rows []map[string]string) ([]byte, error) {
	return json.MarshalIndent(rows, "", "  ")
}

func readColumns(f *excelize.File, sheet string) ([]Column, error) {
	serverMarker, err := f.GetCellValue(sheet, "A5")
	if err != nil {
		return nil, fmt.Errorf("read SERVER row: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(serverMarker), "SERVER") {
		return nil, fmt.Errorf("config233 SERVER row missing at A5")
	}
	serverRow, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("read rows: %w", err)
	}
	if len(serverRow) < serverRowNumber {
		return nil, fmt.Errorf("config233 requires at least %d rows", serverRowNumber)
	}
	serverFields := serverRow[serverRowNumber-1]
	clientFields := rowAt(serverRow, clientRowNumber-1)
	types := rowAt(serverRow, typeRowNumber-1)
	columns := make([]Column, 0)
	for index := 1; index < len(serverFields); index++ {
		name := strings.TrimSpace(serverFields[index])
		if name == "" {
			continue
		}
		column := Column{Name: name}
		if index < len(clientFields) {
			column.ClientName = strings.TrimSpace(clientFields[index])
		}
		if index < len(types) {
			column.Type = strings.TrimSpace(types[index])
		}
		columns = append(columns, column)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("SERVER row has no fields")
	}
	return columns, nil
}

func rowAt(rows [][]string, index int) []string {
	if index < len(rows) {
		return rows[index]
	}
	return nil
}

func sameColumns(actual, expected []string) bool {
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

func cloneValues(values map[string]string) map[string]string {
	copyValues := make(map[string]string, len(values)+1)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		copyValues[key] = values[key]
	}
	return copyValues
}
