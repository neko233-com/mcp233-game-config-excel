package configexcel

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// FormatIssue reports a cell that may have lost text semantics through Excel automatic conversion.
type FormatIssue struct {
	File         string `json:"file"`
	Sheet        string `json:"sheet"`
	Cell         string `json:"cell"`
	Column       string `json:"column"`
	Value        string `json:"value"`
	RawValue     string `json:"rawValue"`
	CellType     string `json:"cellType"`
	NumberFormat int    `json:"numberFormat"`
	Reason       string `json:"reason"`
}

// FormatReport checks all string fields in all config233 workbooks.
type FormatReport struct {
	FilesScanned int           `json:"filesScanned"`
	CheckedCells int           `json:"checkedCells"`
	IssueCount   int           `json:"issueCount"`
	Issues       []FormatIssue `json:"issues"`
}

// CheckTextFormats detects non-text cells in string columns, including date-formatted values.
func CheckTextFormats(paths []string, projectConfig string) (FormatReport, error) {
	resolvedPaths, err := resolveSearchPaths(paths, projectConfig)
	if err != nil {
		return FormatReport{}, err
	}
	files, err := discoverExcelFiles(resolvedPaths)
	if err != nil {
		return FormatReport{}, err
	}
	report := FormatReport{FilesScanned: len(files), Issues: []FormatIssue{}}
	for index := 0; index < len(files); index++ {
		if err := checkWorkbookTextFormats(files[index], &report); err != nil {
			return FormatReport{}, err
		}
	}
	report.IssueCount = len(report.Issues)
	return report, nil
}

func checkWorkbookTextFormats(path string, report *FormatReport) error {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("open workbook %s: %w", path, err)
	}
	defer f.Close()
	for _, sheet := range f.GetSheetList() {
		columns, columnErr := readColumns(f, sheet)
		if columnErr != nil {
			continue
		}
		rows, rowsErr := f.GetRows(sheet)
		if rowsErr != nil {
			return rowsErr
		}
		for columnIndex := 0; columnIndex < len(columns); columnIndex++ {
			column := columns[columnIndex]
			if !strings.EqualFold(column.Type, "string") {
				continue
			}
			for rowNumber := dataRowNumber; rowNumber <= len(rows); rowNumber++ {
				cell, cellErr := excelize.CoordinatesToCellName(column.ExcelColumnNumber, rowNumber)
				if cellErr != nil {
					return cellErr
				}
				value, valueErr := f.GetCellValue(sheet, cell)
				if valueErr != nil || strings.TrimSpace(value) == "" {
					continue
				}
				report.CheckedCells++
				styleID, styleErr := f.GetCellStyle(sheet, cell)
				if styleErr != nil {
					return styleErr
				}
				style, styleErr := f.GetStyle(styleID)
				if styleErr != nil {
					return styleErr
				}
				cellType, typeErr := f.GetCellType(sheet, cell)
				if typeErr != nil {
					return typeErr
				}
				if cellType == excelize.CellTypeSharedString || cellType == excelize.CellTypeInlineString {
					continue
				}
				rawValue, rawErr := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
				if rawErr != nil {
					return rawErr
				}
				reason := "字符串字段单元格不是文本类型"
				if isDateFormat(style.NumFmt) || cellType == excelize.CellTypeDate {
					reason = "字符串字段疑似被 Excel 自动转换为日期"
				}
				report.Issues = append(report.Issues, FormatIssue{File: path, Sheet: sheet, Cell: cell, Column: column.Name, Value: value, RawValue: rawValue, CellType: fmt.Sprintf("%d", cellType), NumberFormat: style.NumFmt, Reason: reason})
			}
		}
	}
	return nil
}

func isDateFormat(numberFormat int) bool {
	return numberFormat >= 14 && numberFormat <= 22
}
