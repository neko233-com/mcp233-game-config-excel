package configexcel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileAccessIssue tells agents why a workbook cannot safely be changed.
type FileAccessIssue struct {
	File       string `json:"file"`
	LockMarker string `json:"lockMarker"`
	Reason     string `json:"reason"`
}

// DeepValidationFile is one automated validation result, suitable for a compact agent summary.
type DeepValidationFile struct {
	File            string   `json:"file"`
	Valid           bool     `json:"valid"`
	DataRows        int      `json:"dataRows"`
	FormatIssues    int      `json:"formatIssues"`
	Issues          []string `json:"issues"`
	Locked          bool     `json:"locked"`
	Skipped         bool     `json:"skipped"`
	LockExplanation string   `json:"lockExplanation,omitempty"`
}

// DeepValidationReport validates structure and text-type risks for every discovered workbook.
type DeepValidationReport struct {
	FilesScanned int                  `json:"filesScanned"`
	ValidFiles   int                  `json:"validFiles"`
	InvalidFiles int                  `json:"invalidFiles"`
	SkippedFiles int                  `json:"skippedFiles"`
	LockedFiles  []FileAccessIssue    `json:"lockedFiles"`
	Files        []DeepValidationFile `json:"files"`
	Cache        CacheStats           `json:"cache"`
}

// DeepValidate performs non-mutating full directory validation. Open workbooks remain readable but are flagged as not writable.
func DeepValidate(paths []string, projectConfig string) (DeepValidationReport, error) {
	resolvedPaths, err := resolveSearchPaths(paths, projectConfig)
	if err != nil {
		return DeepValidationReport{}, err
	}
	files, err := discoverExcelFiles(resolvedPaths)
	if err != nil {
		return DeepValidationReport{}, err
	}
	report := DeepValidationReport{FilesScanned: len(files), LockedFiles: []FileAccessIssue{}, Files: []DeepValidationFile{}}
	for index := 0; index < len(files); index++ {
		file := files[index]
		result := DeepValidationFile{File: file, Valid: true, Issues: []string{}}
		if lock, locked := findWorkbookLock(file); locked {
			result.Locked = true
			result.LockExplanation = "Excel 正打开此表；可以搜索/检查，但关闭后才能修改。"
			report.LockedFiles = append(report.LockedFiles, FileAccessIssue{File: file, LockMarker: lock, Reason: result.LockExplanation})
		}
		inspection, inspectErr := Inspect(file, "")
		if inspectErr != nil {
			if strings.Contains(inspectErr.Error(), "SERVER row has no fields") {
				result.Skipped = true
				result.Issues = append(result.Issues, "SERVER 无字段：可能是客户端专用表，已跳过 config233 校验")
			} else {
				result.Valid = false
				result.Issues = append(result.Issues, inspectErr.Error())
			}
		} else {
			result.DataRows = inspection.DataRows
			for columnIndex := 0; columnIndex < len(inspection.Columns); columnIndex++ {
				if inspection.Columns[columnIndex].Type == "" {
					result.Valid = false
					result.Issues = append(result.Issues, "空 TYPE: "+inspection.Columns[columnIndex].Name)
				}
			}
		}
		formatReport, formatErr := CheckTextFormats([]string{file}, "")
		if formatErr != nil {
			result.Valid = false
			result.Issues = append(result.Issues, formatErr.Error())
		} else {
			result.FormatIssues = formatReport.IssueCount
			if formatReport.IssueCount > 0 {
				result.Issues = append(result.Issues, fmt.Sprintf("%d 个 string 单元格不是文本类型", formatReport.IssueCount))
			}
		}
		if result.Skipped {
			report.SkippedFiles++
		} else if result.Valid {
			report.ValidFiles++
		} else {
			report.InvalidFiles++
		}
		report.Files = append(report.Files, result)
	}
	report.Cache = GetCacheStats()
	return report, nil
}

func findWorkbookLock(path string) (string, bool) {
	marker := filepath.Join(filepath.Dir(path), "~$"+filepath.Base(path))
	if _, err := os.Stat(marker); err == nil {
		return marker, true
	}
	return "", false
}

func ensureWritableWorkbooks(paths []string) error {
	locked := make([]string, 0)
	for index := 0; index < len(paths); index++ {
		if marker, found := findWorkbookLock(paths[index]); found {
			locked = append(locked, paths[index]+"（Excel 锁文件："+marker+"）")
		}
	}
	if len(locked) > 0 {
		return fmt.Errorf("以下 Excel 正被打开，可搜索/检查但不能修改；请关闭后重试：%s", strings.Join(locked, "；"))
	}
	return nil
}
