package validation

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// StructureIssue reports a fixed config233 header or field-definition inconsistency.
type StructureIssue struct {
	Sheet  string `json:"sheet"`
	Cell   string `json:"cell"`
	Reason string `json:"reason"`
}

// CheckStructure validates all five config233 header markers and aligned CLIENT/TYPE field definitions.
func CheckStructure(path, sheet string) ([]StructureIssue, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	resolvedSheet, err := resolveStructureSheet(f, sheet)
	if err != nil {
		return nil, err
	}
	issues := make([]StructureIssue, 0)
	columns, columnErr := columnsFromSheet(f, resolvedSheet)
	if columnErr != nil {
		return nil, columnErr
	}
	seenServerNames := map[string]bool{}
	for index := 0; index < len(columns); index++ {
		column := columns[index]
		cell, cellErr := excelize.CoordinatesToCellName(column.ExcelColumnNumber, 5)
		if cellErr != nil {
			return nil, cellErr
		}
		if seenServerNames[column.Name] {
			issues = append(issues, StructureIssue{Sheet: resolvedSheet, Cell: cell, Reason: "SERVER 字段重复: " + column.Name})
		}
		seenServerNames[column.Name] = true
		if column.Type == "" {
			typeCell, typeCellErr := excelize.CoordinatesToCellName(column.ExcelColumnNumber, 4)
			if typeCellErr != nil {
				return nil, typeCellErr
			}
			issues = append(issues, StructureIssue{Sheet: resolvedSheet, Cell: typeCell, Reason: "SERVER 字段缺少 TYPE: " + column.Name})
		}
	}
	return issues, nil
}

func resolveStructureSheet(f *excelize.File, sheet string) (string, error) {
	if sheet != "" {
		return sheet, nil
	}
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return "", fmt.Errorf("workbook has no sheets")
	}
	return sheets[0], nil
}
