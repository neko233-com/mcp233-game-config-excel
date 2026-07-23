package configexcel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectConfigChange is a dialogue-friendly project configuration preview or write result.
type ProjectConfigChange struct {
	Path    string        `json:"path"`
	Applied bool          `json:"applied"`
	Before  ProjectConfig `json:"before"`
	After   ProjectConfig `json:"after"`
	Diff    string        `json:"diff"`
}

// ConfigureProjectOptions holds all dialogue-configurable project rules.
type ConfigureProjectOptions struct {
	Path                    string   `json:"path"`
	ConfigDirs              []string `json:"configDirs"`
	IDColumn                string   `json:"idColumn"`
	BattleValueScale        int64    `json:"battleValueScale"`
	BattleAttributePatterns []string `json:"battleAttributePatterns"`
	Apply                   bool     `json:"apply"`
}

// ConfigureProject previews or writes mcp233-game-config-excel.txt. apply=false never changes disk.
func ConfigureProject(path string, configDirs []string, apply bool) (ProjectConfigChange, error) {
	return ConfigureProjectWithOptions(ConfigureProjectOptions{Path: path, ConfigDirs: configDirs, IDColumn: "id", BattleValueScale: 100, Apply: apply})
}

// ConfigureProjectWithOptions previews or writes directories and validation rules. apply=false never changes disk.
func ConfigureProjectWithOptions(options ConfigureProjectOptions) (ProjectConfigChange, error) {
	path := filepath.Clean(strings.TrimSpace(options.Path))
	if path == "" {
		return ProjectConfigChange{}, fmt.Errorf("path is required")
	}
	before := DefaultProjectConfig()
	if _, err := os.Stat(path); err == nil {
		loaded, loadErr := LoadProjectConfig(path)
		if loadErr != nil {
			return ProjectConfigChange{}, loadErr
		}
		before = loaded
	} else if !os.IsNotExist(err) {
		return ProjectConfigChange{}, err
	}
	after := DefaultProjectConfig()
	after.IDColumn = strings.TrimSpace(options.IDColumn)
	if after.IDColumn == "" {
		after.IDColumn = "id"
	}
	if after.IDColumn != "id" {
		return ProjectConfigChange{}, fmt.Errorf("idColumn must be exactly id")
	}
	if options.BattleValueScale > 0 {
		after.BattleValueScale = options.BattleValueScale
	}
	if after.BattleValueScale <= 0 {
		return ProjectConfigChange{}, fmt.Errorf("battleValueScale must be a positive integer")
	}
	after.BattleAttributePatterns = append(after.BattleAttributePatterns, options.BattleAttributePatterns...)
	for index := 0; index < len(options.ConfigDirs); index++ {
		directory := strings.TrimSpace(options.ConfigDirs[index])
		if directory == "" {
			continue
		}
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(filepath.Dir(path), directory)
		}
		after.ConfigDirs = appendUniqueString(after.ConfigDirs, filepath.Clean(directory))
	}
	if len(after.ConfigDirs) == 0 {
		return ProjectConfigChange{}, fmt.Errorf("configDirs is required")
	}
	result := ProjectConfigChange{Path: path, Applied: options.Apply, Before: before, After: after, Diff: formatProjectConfigDiff(before, after)}
	if !options.Apply {
		return result, nil
	}
	for index := 0; index < len(after.ConfigDirs); index++ {
		info, err := os.Stat(after.ConfigDirs[index])
		if err != nil || !info.IsDir() {
			return ProjectConfigChange{}, fmt.Errorf("config directory does not exist: %s", after.ConfigDirs[index])
		}
	}
	content := "# mcp233-game-config-excel project configuration.\n# Set through config_excel_configure_project; properties and TOML syntax supported.\nconfig_dirs = ["
	for index := 0; index < len(after.ConfigDirs); index++ {
		if index > 0 {
			content += ", "
		}
		relative, err := filepath.Rel(filepath.Dir(path), after.ConfigDirs[index])
		if err != nil {
			return ProjectConfigChange{}, err
		}
		content += "\"" + filepath.ToSlash(relative) + "\""
	}
	content += "]\n"
	content += "id_column = \"id\"\n"
	content += fmt.Sprintf("battle_value_scale = %d\n", after.BattleValueScale)
	if len(after.BattleAttributePatterns) > 0 {
		content += "battle_attribute_patterns = ["
		for index := 0; index < len(after.BattleAttributePatterns); index++ {
			if index > 0 {
				content += ", "
			}
			content += "\"" + strings.ReplaceAll(after.BattleAttributePatterns[index], "\"", "\\\"") + "\""
		}
		content += "]\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ProjectConfigChange{}, fmt.Errorf("write project config: %w", err)
	}
	return result, nil
}

func formatProjectConfigDiff(before, after ProjectConfig) string {
	lines := []string{"| 配置项 | 修改前 | 修改后 |", "| --- | --- | --- |", "| config_dirs | " + strings.Join(before.ConfigDirs, "<br>") + " | " + strings.Join(after.ConfigDirs, "<br>") + " |", "| id_column | " + before.IDColumn + " | " + after.IDColumn + " |", "| battle_value_scale | " + fmt.Sprintf("%d", before.BattleValueScale) + " | " + fmt.Sprintf("%d", after.BattleValueScale) + " |", "| battle_attribute_patterns | " + strings.Join(before.BattleAttributePatterns, "<br>") + " | " + strings.Join(after.BattleAttributePatterns, "<br>") + " |"}
	return strings.Join(lines, "\n")
}
