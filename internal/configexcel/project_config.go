package configexcel

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProjectConfig describes local business configuration locations.
// It accepts simple properties and a small TOML-compatible subset: config_dirs = ["dir1", "dir2"].
type ProjectConfig struct {
	ConfigDirs              []string `json:"configDirs"`
	IDColumn                string   `json:"idColumn"`
	BattleValueScale        int64    `json:"battleValueScale"`
	BattleAttributePatterns []string `json:"battleAttributePatterns"`
}

// LoadProjectConfig reads mcp233-game-config-excel.txt and resolves relative paths against its directory.
func LoadProjectConfig(path string) (ProjectConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("open project config: %w", err)
	}
	defer file.Close()
	config := DefaultProjectConfig()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		switch key {
		case "config_dir", "config_dirs", "business_config_dir":
			values := parseProjectConfigValues(value)
			for index := 0; index < len(values); index++ {
				item := values[index]
				if !filepath.IsAbs(item) {
					item = filepath.Join(filepath.Dir(path), item)
				}
				config.ConfigDirs = appendUniqueString(config.ConfigDirs, filepath.Clean(item))
			}
		case "id_column":
			config.IDColumn = strings.Trim(strings.TrimSpace(value), "\"'")
		case "battle_value_scale":
			scale, parseErr := strconv.ParseInt(strings.Trim(strings.TrimSpace(value), "\"'"), 10, 64)
			if parseErr != nil || scale <= 0 {
				return ProjectConfig{}, fmt.Errorf("battle_value_scale must be a positive integer")
			}
			config.BattleValueScale = scale
		case "battle_attribute_pattern", "battle_attribute_patterns":
			config.BattleAttributePatterns = parseProjectConfigValues(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return ProjectConfig{}, fmt.Errorf("read project config: %w", err)
	}
	if len(config.ConfigDirs) == 0 {
		return ProjectConfig{}, fmt.Errorf("project config has no config_dir or config_dirs")
	}
	return config, nil
}

// DefaultProjectConfig returns project rules that agents can inspect and override through MCP.
func DefaultProjectConfig() ProjectConfig {
	return ProjectConfig{
		ConfigDirs:              []string{},
		IDColumn:                "id",
		BattleValueScale:        100,
		BattleAttributePatterns: []string{"(?i)(attr|talent|speed)"},
	}
}

func parseProjectConfigValues(value string) []string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, "[]")
	parts := strings.Split(trimmed, ",")
	result := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		item := strings.Trim(strings.TrimSpace(parts[index]), "\"'")
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func appendUniqueString(items []string, value string) []string {
	for index := 0; index < len(items); index++ {
		if items[index] == value {
			return items
		}
	}
	return append(items, value)
}
