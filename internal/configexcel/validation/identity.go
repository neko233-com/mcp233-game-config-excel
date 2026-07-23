package validation

import (
	"fmt"
	"strings"

	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel"
	"github.com/xuri/excelize/v2"
)

// IdentityIssue reports an id column naming, empty value, or uniqueness failure.
type IdentityIssue struct {
	Sheet  string `json:"sheet"`
	Cell   string `json:"cell"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// CheckID verifies the fixed project rule: every config233 sheet uses an exact lowercase id column with non-empty unique values.
func CheckID(path, sheet string) ([]IdentityIssue, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	resolvedSheet, err := configexcel.ResolveSheet(f, sheet)
	if err != nil {
		return nil, err
	}
	columns, err := columnsFromSheet(f, resolvedSheet)
	if err != nil {
		return nil, err
	}
	idColumnNumber := 0
	for index := 0; index < len(columns); index++ {
		if columns[index].Name == "id" {
			idColumnNumber = columns[index].ExcelColumnNumber
			break
		}
	}
	issues := make([]IdentityIssue, 0)
	if idColumnNumber == 0 {
		return append(issues, IdentityIssue{Sheet: resolvedSheet, Reason: "缺少规范唯一主键列 id；禁止使用 ID、uid 等其它列名"}), nil
	}
	rows, err := f.GetRows(resolvedSheet)
	if err != nil {
		return nil, err
	}
	seen := map[string]string{}
	for rowNumber := 6; rowNumber <= len(rows); rowNumber++ {
		cell, cellErr := excelize.CoordinatesToCellName(idColumnNumber, rowNumber)
		if cellErr != nil {
			return nil, cellErr
		}
		value, valueErr := f.GetCellValue(resolvedSheet, cell)
		if valueErr != nil {
			return nil, valueErr
		}
		value = strings.TrimSpace(value)
		if value == "" {
			if rowHasConfigData(rows[rowNumber-1]) {
				issues = append(issues, IdentityIssue{Sheet: resolvedSheet, Cell: cell, Reason: "数据行 id 为空"})
			}
			continue
		}
		if firstCell, found := seen[value]; found {
			issues = append(issues, IdentityIssue{Sheet: resolvedSheet, Cell: cell, Value: value, Reason: "id 重复；首次出现于 " + firstCell})
			continue
		}
		seen[value] = cell
	}
	return issues, nil
}

func rowHasConfigData(row []string) bool {
	for index := 1; index < len(row); index++ {
		if strings.TrimSpace(row[index]) != "" {
			return true
		}
	}
	return false
}
