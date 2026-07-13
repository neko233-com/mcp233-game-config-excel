package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel"
)

// ProjectFileReport joins config233 structural checks with project id and battle rules.
type ProjectFileReport struct {
	File            string           `json:"file"`
	Skipped         bool             `json:"skipped"`
	Valid           bool             `json:"valid"`
	IDIssues        []IdentityIssue  `json:"idIssues"`
	StructureIssues []StructureIssue `json:"structureIssues"`
	Battle          BattleReport     `json:"battle"`
	OtherIssues     []string         `json:"otherIssues"`
}

// ProjectReport is the comprehensive non-mutating configuration audit.
type ProjectReport struct {
	Rules        configexcel.ProjectConfig     `json:"rules"`
	FilesScanned int                           `json:"filesScanned"`
	ValidFiles   int                           `json:"validFiles"`
	InvalidFiles int                           `json:"invalidFiles"`
	SkippedFiles int                           `json:"skippedFiles"`
	LockedFiles  []configexcel.FileAccessIssue `json:"lockedFiles"`
	IDIssueCount int                           `json:"idIssueCount"`
	BattleIssues int                           `json:"battleIssueCount"`
	Files        []ProjectFileReport           `json:"files"`
}

// ValidateProject applies the fixed id rule and configured battle scale rule to every config233 workbook.
func ValidateProject(projectConfigPath string) (ProjectReport, error) {
	config, err := configexcel.LoadProjectConfig(projectConfigPath)
	if err != nil {
		return ProjectReport{}, err
	}
	if config.IDColumn != "id" {
		return ProjectReport{}, fmt.Errorf("project rule id_column must be exactly id")
	}
	deep, err := configexcel.DeepValidate(nil, projectConfigPath)
	if err != nil {
		return ProjectReport{}, err
	}
	report := ProjectReport{Rules: config, FilesScanned: deep.FilesScanned, SkippedFiles: deep.SkippedFiles, LockedFiles: deep.LockedFiles, Files: []ProjectFileReport{}}
	for index := 0; index < len(deep.Files); index++ {
		base := deep.Files[index]
		file := ProjectFileReport{File: base.File, Skipped: base.Skipped, Valid: base.Valid, IDIssues: []IdentityIssue{}, StructureIssues: []StructureIssue{}, OtherIssues: base.Issues, Battle: BattleReport{Scale: config.BattleValueScale, MatchedColumns: []string{}, Issues: []BattleIssue{}}}
		if !file.Skipped && file.Valid {
			structureIssues, structureErr := CheckStructure(base.File, "")
			if structureErr != nil {
				file.Valid = false
				file.OtherIssues = append(file.OtherIssues, structureErr.Error())
			} else {
				file.StructureIssues = structureIssues
				if len(structureIssues) > 0 {
					file.Valid = false
				}
			}
			idIssues, idErr := CheckID(base.File, "")
			if idErr != nil {
				file.Valid = false
				file.OtherIssues = append(file.OtherIssues, idErr.Error())
			} else {
				file.IDIssues = idIssues
				report.IDIssueCount += len(idIssues)
				if len(idIssues) > 0 {
					file.Valid = false
				}
			}
			battle, battleErr := CheckBattleValues(base.File, "", config)
			if battleErr != nil {
				file.Valid = false
				file.OtherIssues = append(file.OtherIssues, battleErr.Error())
			} else {
				file.Battle = battle
				report.BattleIssues += battle.IssueCount
				if battle.IssueCount > 0 {
					file.Valid = false
				}
			}
		}
		if file.Skipped {
			// Count already comes from deep validation.
		} else if file.Valid {
			report.ValidFiles++
		} else {
			report.InvalidFiles++
		}
		report.Files = append(report.Files, file)
	}
	return report, nil
}

// SelfCheck verifies MCP-facing project configuration can be parsed and its configured directories are usable.
func SelfCheck(projectConfigPath string) (ProjectReport, error) {
	return ValidateProject(projectConfigPath)
}

// FindConfigFiles is retained as a validator-level discover helper for future standalone rules.
func FindConfigFiles(paths []string) ([]string, error) {
	result := make([]string, 0)
	for index := 0; index < len(paths); index++ {
		path := paths[index]
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if strings.EqualFold(filepath.Ext(path), ".xlsx") && !strings.HasPrefix(filepath.Base(path), "~$") {
				result = append(result, path)
			}
			continue
		}
		err = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(current), ".xlsx") && !strings.HasPrefix(filepath.Base(current), "~$") {
				result = append(result, current)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(result)
	return result, nil
}
