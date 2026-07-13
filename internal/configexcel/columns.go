package configexcel

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

const textNumberFormatCode = "@"

// ColumnDefinition describes one config233 column across the five header rows.
type ColumnDefinition struct {
	Name        string `json:"name"`
	ClientName  string `json:"clientName"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
	Comment     string `json:"comment"`
}

// ColumnFormatCheck reports whether a column keeps its expected config type and Excel text format.
type ColumnFormatCheck struct {
	Column       ColumnDefinition `json:"column"`
	ExcelColumn  string           `json:"excelColumn"`
	IsTextFormat bool             `json:"isTextFormat"`
	Issues       []string         `json:"issues"`
}

// AddColumn inserts a config233 column, copies neighbouring presentation style and defaults to text cells.
func AddColumn(path, requestedSheet string, definition ColumnDefinition, afterColumn string) error {
	definition = normalizeColumnDefinition(definition)
	if definition.Name == "" {
		return fmt.Errorf("column name is required")
	}
	if err := ensureWritableWorkbooks([]string{path}); err != nil {
		return err
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	sheet, err := ResolveSheet(f, requestedSheet)
	if err != nil {
		return err
	}
	columns, err := readColumns(f, sheet)
	if err != nil {
		return err
	}
	insertAt, err := getInsertColumnNumber(columns, definition.Name, afterColumn)
	if err != nil {
		return err
	}
	insertColumnName, err := excelize.ColumnNumberToName(insertAt)
	if err != nil {
		return err
	}
	if err := f.InsertCols(sheet, insertColumnName, 1); err != nil {
		return fmt.Errorf("insert column: %w", err)
	}
	if err := copyColumnStyle(f, sheet, insertAt); err != nil {
		return err
	}
	if err := writeColumnHeaders(f, sheet, insertAt, definition); err != nil {
		return err
	}
	if err := setColumnTextFormat(f, sheet, insertAt); err != nil {
		return err
	}
	if err := f.Save(); err != nil {
		return fmt.Errorf("save workbook: %w", err)
	}
	invalidateWorkbookCache(path)
	return nil
}

// DeleteColumn removes one config233 SERVER column and all of its aligned header/data cells.
func DeleteColumn(path, requestedSheet, columnName string) error {
	columnName = strings.TrimSpace(columnName)
	if columnName == "" {
		return fmt.Errorf("column name is required")
	}
	if err := ensureWritableWorkbooks([]string{path}); err != nil {
		return err
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	sheet, err := ResolveSheet(f, requestedSheet)
	if err != nil {
		return err
	}
	columns, err := readColumns(f, sheet)
	if err != nil {
		return err
	}
	columnNumber, found := getColumnNumber(columns, columnName)
	if !found {
		return fmt.Errorf("server column not found: %s", columnName)
	}
	excelColumnName, err := excelize.ColumnNumberToName(columnNumber)
	if err != nil {
		return err
	}
	if err := f.RemoveCol(sheet, excelColumnName); err != nil {
		return fmt.Errorf("remove column: %w", err)
	}
	if err := f.Save(); err != nil {
		return fmt.Errorf("save workbook: %w", err)
	}
	invalidateWorkbookCache(path)
	return nil
}

// CheckColumnFormat validates a named config233 column and verifies string columns use Excel text formatting.
func CheckColumnFormat(path, requestedSheet, columnName string, requireText bool) (ColumnFormatCheck, error) {
	columnName = strings.TrimSpace(columnName)
	if columnName == "" {
		return ColumnFormatCheck{}, fmt.Errorf("column name is required")
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return ColumnFormatCheck{}, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	sheet, err := ResolveSheet(f, requestedSheet)
	if err != nil {
		return ColumnFormatCheck{}, err
	}
	columns, err := readColumns(f, sheet)
	if err != nil {
		return ColumnFormatCheck{}, err
	}
	columnNumber, column, found := findColumn(columns, columnName)
	if !found {
		return ColumnFormatCheck{}, fmt.Errorf("server column not found: %s", columnName)
	}
	definition := columnDefinitionFromColumn(column)
	excelColumn, err := excelize.ColumnNumberToName(columnNumber)
	if err != nil {
		return ColumnFormatCheck{}, err
	}
	check := ColumnFormatCheck{
		Column:      definition,
		ExcelColumn: excelColumn,
		Issues:      []string{},
	}
	if requireText && !strings.EqualFold(definition.Type, "string") {
		check.Issues = append(check.Issues, "TYPE row must be string for a text column")
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return ColumnFormatCheck{}, fmt.Errorf("read rows: %w", err)
	}
	lastRow := len(rows)
	if lastRow < dataRowNumber {
		lastRow = dataRowNumber
	}
	textFormat := true
	for rowNumber := 1; rowNumber <= lastRow; rowNumber++ {
		cell, cellErr := excelize.CoordinatesToCellName(columnNumber, rowNumber)
		if cellErr != nil {
			return ColumnFormatCheck{}, cellErr
		}
		styleID, styleErr := f.GetCellStyle(sheet, cell)
		if styleErr != nil {
			return ColumnFormatCheck{}, styleErr
		}
		style, styleErr := f.GetStyle(styleID)
		if styleErr != nil {
			return ColumnFormatCheck{}, styleErr
		}
		if style.NumFmt != 49 {
			textFormat = false
			check.Issues = append(check.Issues, fmt.Sprintf("cell %s is not Excel text format", cell))
		}
	}
	check.IsTextFormat = textFormat
	return check, nil
}

func normalizeColumnDefinition(definition ColumnDefinition) ColumnDefinition {
	definition.Name = strings.TrimSpace(definition.Name)
	definition.ClientName = strings.TrimSpace(definition.ClientName)
	definition.DisplayName = strings.TrimSpace(definition.DisplayName)
	definition.Type = strings.TrimSpace(definition.Type)
	definition.Comment = strings.TrimSpace(definition.Comment)
	if definition.ClientName == "" {
		definition.ClientName = definition.Name
	}
	if definition.DisplayName == "" {
		definition.DisplayName = definition.ClientName
	}
	if definition.Type == "" {
		definition.Type = "string"
	}
	return definition
}

func columnDefinitionFromColumn(column Column) ColumnDefinition {
	return ColumnDefinition{Name: column.Name, ClientName: column.ClientName, DisplayName: column.ClientName, Type: column.Type}
}

func getColumnNumber(columns []Column, columnName string) (int, bool) {
	columnNumber, _, found := findColumn(columns, columnName)
	return columnNumber, found
}

func findColumn(columns []Column, columnName string) (int, Column, bool) {
	for index := 0; index < len(columns); index++ {
		if columns[index].Name == columnName {
			return columns[index].ExcelColumnNumber, columns[index], true
		}
	}
	return 0, Column{}, false
}

func getInsertColumnNumber(columns []Column, columnName, afterColumn string) (int, error) {
	if _, found := getColumnNumber(columns, columnName); found {
		return 0, fmt.Errorf("server column already exists: %s", columnName)
	}
	if strings.TrimSpace(afterColumn) == "" {
		return columns[len(columns)-1].ExcelColumnNumber + 1, nil
	}
	afterColumnNumber, found := getColumnNumber(columns, strings.TrimSpace(afterColumn))
	if !found {
		return 0, fmt.Errorf("after column not found: %s", afterColumn)
	}
	return afterColumnNumber + 1, nil
}

func copyColumnStyle(f *excelize.File, sheet string, targetColumnNumber int) error {
	sourceColumnNumber := targetColumnNumber + 1
	sourceHeaderCell, err := excelize.CoordinatesToCellName(sourceColumnNumber, serverRowNumber)
	if err != nil {
		return err
	}
	sourceHeader, err := f.GetCellValue(sheet, sourceHeaderCell)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sourceHeader) == "" {
		sourceColumnNumber = targetColumnNumber - 1
	}
	for rowNumber := 1; rowNumber <= dataRowNumber; rowNumber++ {
		sourceCell, err := excelize.CoordinatesToCellName(sourceColumnNumber, rowNumber)
		if err != nil {
			return err
		}
		targetCell, err := excelize.CoordinatesToCellName(targetColumnNumber, rowNumber)
		if err != nil {
			return err
		}
		styleID, err := f.GetCellStyle(sheet, sourceCell)
		if err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, targetCell, targetCell, styleID); err != nil {
			return err
		}
	}
	sourceColumn, err := excelize.ColumnNumberToName(sourceColumnNumber)
	if err != nil {
		return err
	}
	targetColumn, err := excelize.ColumnNumberToName(targetColumnNumber)
	if err != nil {
		return err
	}
	width, err := f.GetColWidth(sheet, sourceColumn)
	if err != nil {
		return err
	}
	return f.SetColWidth(sheet, targetColumn, targetColumn, width)
}

func writeColumnHeaders(f *excelize.File, sheet string, columnNumber int, definition ColumnDefinition) error {
	values := []string{definition.Comment, definition.DisplayName, definition.ClientName, definition.Type, definition.Name}
	for index := 0; index < len(values); index++ {
		cell, err := excelize.CoordinatesToCellName(columnNumber, index+1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell, values[index]); err != nil {
			return err
		}
	}
	return nil
}

func setColumnTextFormat(f *excelize.File, sheet string, columnNumber int) error {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return fmt.Errorf("read rows: %w", err)
	}
	endRow := len(rows)
	if endRow < dataRowNumber {
		endRow = dataRowNumber
	}
	for rowNumber := 1; rowNumber <= endRow; rowNumber++ {
		cell, cellErr := excelize.CoordinatesToCellName(columnNumber, rowNumber)
		if cellErr != nil {
			return cellErr
		}
		styleID, styleErr := f.GetCellStyle(sheet, cell)
		if styleErr != nil {
			return styleErr
		}
		style, styleErr := f.GetStyle(styleID)
		if styleErr != nil {
			return styleErr
		}
		style.NumFmt = 49
		textStyleID, styleErr := f.NewStyle(style)
		if styleErr != nil {
			return styleErr
		}
		if styleErr := f.SetCellStyle(sheet, cell, cell, textStyleID); styleErr != nil {
			return styleErr
		}
	}
	return nil
}
