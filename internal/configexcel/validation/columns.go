package validation

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Column is minimal config233 metadata used by advanced validators.
type Column struct {
	Name              string
	ClientName        string
	Type              string
	ExcelColumnNumber int
}

func columnsFromSheet(f *excelize.File, sheet string) ([]Column, error) {
	marker, err := f.GetCellValue(sheet, "A5")
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(marker), "SERVER") {
		return nil, fmt.Errorf("config233 SERVER row missing at A5")
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) < 5 || len(rows[4]) <= 1 {
		return nil, fmt.Errorf("SERVER row has no fields")
	}
	result := make([]Column, 0)
	for index := 1; index < len(rows[4]); index++ {
		name := strings.TrimSpace(rows[4][index])
		if name == "" {
			continue
		}
		column := Column{Name: name, ExcelColumnNumber: index + 1}
		if len(rows) > 2 && index < len(rows[2]) {
			column.ClientName = strings.TrimSpace(rows[2][index])
		}
		if len(rows) > 3 && index < len(rows[3]) {
			column.Type = strings.TrimSpace(rows[3][index])
		}
		result = append(result, column)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("SERVER row has no fields")
	}
	return result, nil
}
