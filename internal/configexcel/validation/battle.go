package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel"
	"github.com/xuri/excelize/v2"
)

// BattleIssue explains a value that cannot follow the raw 100-to-display-1.00 convention.
type BattleIssue struct {
	Sheet        string `json:"sheet"`
	Cell         string `json:"cell"`
	Column       string `json:"column"`
	RawValue     string `json:"rawValue"`
	DisplayValue string `json:"displayValue,omitempty"`
	Reason       string `json:"reason"`
}

// BattleReport checks configured battle fields. Every valid raw integer n displays as n / 100 with two decimals.
type BattleReport struct {
	Scale          int64         `json:"scale"`
	DisplayRule    string        `json:"displayRule"`
	MatchedColumns []string      `json:"matchedColumns"`
	CheckedCells   int           `json:"checkedCells"`
	IssueCount     int           `json:"issueCount"`
	Issues         []BattleIssue `json:"issues"`
}

// CheckBattleValues validates explicitly configured battle attribute patterns.
func CheckBattleValues(path, sheet string, config configexcel.ProjectConfig) (BattleReport, error) {
	report := BattleReport{Scale: config.BattleValueScale, DisplayRule: fmt.Sprintf("游戏显示值 = 原始整数 / %d；原始值 %d = 显示 1.00", config.BattleValueScale, config.BattleValueScale), MatchedColumns: []string{}, Issues: []BattleIssue{}}
	if report.Scale <= 0 {
		return BattleReport{}, fmt.Errorf("battle value scale must be positive")
	}
	patterns, err := compilePatterns(config.BattleAttributePatterns)
	if err != nil {
		return BattleReport{}, err
	}
	if len(patterns) == 0 {
		return report, nil
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return BattleReport{}, fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	resolvedSheet, err := configexcel.ResolveSheet(f, sheet)
	if err != nil {
		return BattleReport{}, err
	}
	columns, err := columnsFromSheet(f, resolvedSheet)
	if err != nil {
		return BattleReport{}, err
	}
	rows, err := f.GetRows(resolvedSheet)
	if err != nil {
		return BattleReport{}, err
	}
	for index := 0; index < len(columns); index++ {
		column := columns[index]
		if column.Type != "int" || (!matchesPattern(column.Name, patterns) && !matchesPattern(column.ClientName, patterns)) {
			continue
		}
		report.MatchedColumns = append(report.MatchedColumns, column.Name)
		for rowNumber := 6; rowNumber <= len(rows); rowNumber++ {
			cell, cellErr := excelize.CoordinatesToCellName(column.ExcelColumnNumber, rowNumber)
			if cellErr != nil {
				return BattleReport{}, cellErr
			}
			raw, rawErr := f.GetCellValue(resolvedSheet, cell, excelize.Options{RawCellValue: true})
			if rawErr != nil || strings.TrimSpace(raw) == "" {
				continue
			}
			report.CheckedCells++
			value, parseErr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if parseErr != nil {
				report.Issues = append(report.Issues, BattleIssue{Sheet: resolvedSheet, Cell: cell, Column: column.Name, RawValue: raw, Reason: "战斗属性必须使用整数原始值；100 表示游戏显示 1.00"})
				continue
			}
			display := float64(value) / float64(report.Scale)
			_ = display
		}
	}
	report.IssueCount = len(report.Issues)
	return report, nil
}

func compilePatterns(rawPatterns []string) ([]*regexp.Regexp, error) {
	result := make([]*regexp.Regexp, 0, len(rawPatterns))
	for index := 0; index < len(rawPatterns); index++ {
		pattern := strings.TrimSpace(rawPatterns[index])
		if pattern == "" {
			continue
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid battle attribute pattern %q: %w", pattern, err)
		}
		result = append(result, compiled)
	}
	return result, nil
}

func matchesPattern(value string, patterns []*regexp.Regexp) bool {
	for index := 0; index < len(patterns); index++ {
		if patterns[index].MatchString(value) {
			return true
		}
	}
	return false
}
