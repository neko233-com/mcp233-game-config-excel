package configexcel

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ReplaceOptions replaces literal or regular-expression matches across discovered config Excel files.
type ReplaceOptions struct {
	SearchOptions
	Replacement string `json:"replacement"`
	Apply       bool   `json:"apply"`
}

// Change describes an individual cell before and after a bulk operation.
type Change struct {
	File   string `json:"file"`
	Sheet  string `json:"sheet"`
	Cell   string `json:"cell"`
	Column string `json:"column"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// ReplaceReport always presents a reviewable before/after list; applied reports were persisted.
type ReplaceReport struct {
	Applied      bool     `json:"applied"`
	FilesChanged []string `json:"filesChanged"`
	ChangeCount  int      `json:"changeCount"`
	Changes      []Change `json:"changes"`
	MarkdownDiff string   `json:"markdownDiff"`
}

// Replace previews by default. apply=true persists all listed cell changes.
func Replace(options ReplaceOptions) (ReplaceReport, error) {
	if strings.TrimSpace(options.Query) == "" {
		return ReplaceReport{}, fmt.Errorf("query is required")
	}
	paths, err := resolveSearchPaths(options.Paths, options.ProjectConfig)
	if err != nil {
		return ReplaceReport{}, err
	}
	files, err := discoverExcelFiles(paths)
	if err != nil {
		return ReplaceReport{}, err
	}
	if options.Apply {
		if err := ensureWritableWorkbooks(files); err != nil {
			return ReplaceReport{}, err
		}
	}
	replacer, err := newTextReplacer(options.Query, options.Replacement, options.Regex, options.CaseSensitive)
	if err != nil {
		return ReplaceReport{}, err
	}
	report := ReplaceReport{Applied: options.Apply, FilesChanged: []string{}, Changes: []Change{}}
	for index := 0; index < len(files); index++ {
		changed, fileChanges, replaceErr := replaceWorkbook(files[index], options.IncludeHeaders, replacer, options.Apply)
		if replaceErr != nil {
			return ReplaceReport{}, replaceErr
		}
		if changed {
			report.FilesChanged = append(report.FilesChanged, files[index])
			if options.Apply {
				invalidateWorkbookCache(files[index])
			}
		}
		report.Changes = append(report.Changes, fileChanges...)
	}
	report.ChangeCount = len(report.Changes)
	report.MarkdownDiff = formatChangeTable(report.Changes)
	return report, nil
}

func newTextReplacer(query, replacement string, regex bool, caseSensitive bool) (func(string) string, error) {
	if regex {
		if !caseSensitive {
			query = "(?i)" + query
		}
		pattern, err := regexp.Compile(query)
		if err != nil {
			return nil, err
		}
		return func(value string) string { return pattern.ReplaceAllString(value, replacement) }, nil
	}
	if caseSensitive {
		return func(value string) string { return strings.ReplaceAll(value, query, replacement) }, nil
	}
	pattern, err := regexp.Compile("(?i)" + regexp.QuoteMeta(query))
	if err != nil {
		return nil, err
	}
	return func(value string) string { return pattern.ReplaceAllString(value, replacement) }, nil
}

func replaceWorkbook(path string, includeHeaders bool, replacer func(string) string, apply bool) (bool, []Change, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return false, nil, fmt.Errorf("open workbook %s: %w", path, err)
	}
	defer f.Close()
	changes := make([]Change, 0)
	for _, sheet := range f.GetSheetList() {
		columns, columnErr := readColumns(f, sheet)
		if columnErr != nil {
			continue
		}
		startRow := dataRowNumber
		if includeHeaders {
			startRow = 1
		}
		rows, rowsErr := f.GetRows(sheet)
		if rowsErr != nil {
			return false, nil, rowsErr
		}
		for rowNumber := startRow; rowNumber <= len(rows); rowNumber++ {
			for columnIndex := 0; columnIndex < len(columns); columnIndex++ {
				column := columns[columnIndex]
				cell, cellErr := excelize.CoordinatesToCellName(column.ExcelColumnNumber, rowNumber)
				if cellErr != nil {
					return false, nil, cellErr
				}
				before, valueErr := f.GetCellValue(sheet, cell)
				if valueErr != nil {
					return false, nil, valueErr
				}
				after := replacer(before)
				if after == before {
					continue
				}
				changes = append(changes, Change{File: path, Sheet: sheet, Cell: cell, Column: column.Name, Before: before, After: after})
				if apply {
					if setErr := f.SetCellValue(sheet, cell, after); setErr != nil {
						return false, nil, setErr
					}
				}
			}
		}
	}
	if apply && len(changes) > 0 {
		if saveErr := f.Save(); saveErr != nil {
			return false, nil, fmt.Errorf("save workbook %s: %w", path, saveErr)
		}
	}
	return len(changes) > 0, changes, nil
}

func formatChangeTable(changes []Change) string {
	if len(changes) == 0 {
		return "无匹配修改。"
	}
	lines := []string{"| 文件 | Sheet | 单元格 | 字段 | 修改前 | 修改后 |", "| --- | --- | --- | --- | --- | --- |"}
	for index := 0; index < len(changes); index++ {
		change := changes[index]
		lines = append(lines, "| "+escapeMarkdownCell(change.File)+" | "+escapeMarkdownCell(change.Sheet)+" | "+change.Cell+" | "+escapeMarkdownCell(change.Column)+" | "+escapeMarkdownCell(change.Before)+" | "+escapeMarkdownCell(change.After)+" |")
	}
	return strings.Join(lines, "\n")
}

func escapeMarkdownCell(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "|", "\\|"), "\n", "<br>")
}
