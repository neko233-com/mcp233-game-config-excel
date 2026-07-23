package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel"
	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel/validation"
	"github.com/neko233-com/mcp233-game-config-excel/internal/mcp"
	"github.com/neko233-com/mcp233-game-config-excel/internal/updater"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "serve" || args[0] == "--stdio" {
		return mcp.Serve(os.Stdin, os.Stdout)
	}
	switch args[0] {
	case "inspect":
		flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
		path := flags.String("file", "", "local xlsx path")
		sheet := flags.String("sheet", "", "sheet name")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		inspection, err := configexcel.Inspect(*path, *sheet)
		if err != nil {
			return err
		}
		return printJSON(inspection)
	case "validate":
		flags := flag.NewFlagSet("validate", flag.ContinueOnError)
		path := flags.String("file", "", "local xlsx path")
		sheet := flags.String("sheet", "", "sheet name")
		expected := flags.String("expected-columns", "", "comma-separated exact SERVER fields")
		uidColumn := flags.String("uid-column", "id", "unique id field")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		result, err := configexcel.Validate(*path, *sheet, splitCSV(*expected), *uidColumn)
		if err != nil {
			return err
		}
		if err := printJSON(result); err != nil {
			return err
		}
		if !result.Valid {
			return fmt.Errorf("validation failed")
		}
		return nil
	case "read":
		flags := flag.NewFlagSet("read", flag.ContinueOnError)
		path := flags.String("file", "", "local xlsx path")
		sheet := flags.String("sheet", "", "sheet name")
		limit := flags.Int("limit", 0, "maximum data rows; zero means all")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		rows, err := configexcel.ReadRows(*path, *sheet, *limit)
		if err != nil {
			return err
		}
		return printJSON(rows)
	case "upsert":
		flags := flag.NewFlagSet("upsert", flag.ContinueOnError)
		path := flags.String("file", "", "local xlsx path")
		sheet := flags.String("sheet", "", "sheet name")
		uidColumn := flags.String("uid-column", "id", "unique id field")
		uid := flags.String("uid", "", "row id")
		values := flags.String("values", "{}", "JSON object with fields to write")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		valueMap := map[string]string{}
		if err := json.Unmarshal([]byte(*values), &valueMap); err != nil {
			return fmt.Errorf("parse --values: %w", err)
		}
		created, err := configexcel.UpsertRow(*path, *sheet, *uidColumn, *uid, valueMap)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"created": created, "uid": *uid})
	case "add-column":
		flags := flag.NewFlagSet("add-column", flag.ContinueOnError)
		path := flags.String("file", "", "local xlsx path")
		sheet := flags.String("sheet", "", "sheet name")
		name := flags.String("name", "", "SERVER field name")
		clientName := flags.String("client-name", "", "CLIENT field name; defaults to name")
		displayName := flags.String("display-name", "", "row 2 display name; defaults to client name")
		typeName := flags.String("type", "string", "config233 TYPE; defaults to string")
		comment := flags.String("comment", "", "row 1 field comment")
		afterColumn := flags.String("after-column", "", "insert after this SERVER field; defaults to append")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return configexcel.AddColumn(*path, *sheet, configexcel.ColumnDefinition{
			Name: *name, ClientName: *clientName, DisplayName: *displayName, Type: *typeName, Comment: *comment,
		}, *afterColumn)
	case "delete-column":
		flags := flag.NewFlagSet("delete-column", flag.ContinueOnError)
		path := flags.String("file", "", "local xlsx path")
		sheet := flags.String("sheet", "", "sheet name")
		name := flags.String("name", "", "SERVER field name")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return configexcel.DeleteColumn(*path, *sheet, *name)
	case "check-column":
		flags := flag.NewFlagSet("check-column", flag.ContinueOnError)
		path := flags.String("file", "", "local xlsx path")
		sheet := flags.String("sheet", "", "sheet name")
		name := flags.String("name", "", "SERVER field name")
		requireText := flags.Bool("require-text", true, "require TYPE string and Excel text number format")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		result, err := configexcel.CheckColumnFormat(*path, *sheet, *name, *requireText)
		if err != nil {
			return err
		}
		if err := printJSON(result); err != nil {
			return err
		}
		if len(result.Issues) > 0 {
			return fmt.Errorf("column format check failed")
		}
		return nil
	case "init-i18n":
		flags := flag.NewFlagSet("init-i18n", flag.ContinueOnError)
		path := flags.String("file", "I18nTipsConfig.xlsx", "output xlsx path")
		sheet := flags.String("sheet", "I18nTipsConfig", "sheet name")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return configexcel.CreateI18nTemplate(*path, *sheet)
	case "export-i18n":
		flags := flag.NewFlagSet("export-i18n", flag.ContinueOnError)
		path := flags.String("file", "", "local xlsx path")
		sheet := flags.String("sheet", "", "sheet name")
		outputDir := flags.String("output-dir", "i18n", "local output directory")
		format := flags.String("format", configexcel.I18nExportFormatJSON, "json, csv or tsv")
		mode := flags.String("mode", configexcel.I18nExportModeFull, "full or incremental")
		baselinePath := flags.String("baseline-file", "", "previous compatible xlsx; required for incremental")
		baselineSheet := flags.String("baseline-sheet", "", "baseline sheet name")
		uidColumn := flags.String("uid-column", "id", "unique id field")
		languageColumns := flags.String("language-columns", "", "optional comma-separated language columns")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		report, err := configexcel.ExportI18n(configexcel.I18nExportOptions{
			Path:            *path,
			Sheet:           *sheet,
			OutputDir:       *outputDir,
			Format:          *format,
			Mode:            *mode,
			BaselinePath:    *baselinePath,
			BaselineSheet:   *baselineSheet,
			UIDColumn:       *uidColumn,
			LanguageColumns: splitCSV(*languageColumns),
		})
		if err != nil {
			return err
		}
		return printJSON(report)
	case "search":
		flags := flag.NewFlagSet("search", flag.ContinueOnError)
		paths := flags.String("paths", "", "comma-separated Excel files or directories")
		projectConfig := flags.String("project-config", "", "mcp233-game-config-excel.txt path")
		query := flags.String("query", "", "literal text or regular expression")
		regex := flags.Bool("regex", false, "interpret query as regular expression")
		caseSensitive := flags.Bool("case-sensitive", false, "case-sensitive search")
		includeHeaders := flags.Bool("include-headers", false, "also search header rows")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return printJSONMust(configexcel.Search(configexcel.SearchOptions{Paths: splitCSV(*paths), ProjectConfig: *projectConfig, Query: *query, Regex: *regex, CaseSensitive: *caseSensitive, IncludeHeaders: *includeHeaders}))
	case "replace":
		flags := flag.NewFlagSet("replace", flag.ContinueOnError)
		paths := flags.String("paths", "", "comma-separated Excel files or directories")
		projectConfig := flags.String("project-config", "", "mcp233-game-config-excel.txt path")
		query := flags.String("query", "", "literal text or regular expression")
		replacement := flags.String("replacement", "", "replacement text")
		regex := flags.Bool("regex", false, "interpret query as regular expression")
		caseSensitive := flags.Bool("case-sensitive", false, "case-sensitive search")
		includeHeaders := flags.Bool("include-headers", false, "also replace header rows")
		apply := flags.Bool("apply", false, "write changes; without this only preview")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return printJSONMust(configexcel.Replace(configexcel.ReplaceOptions{SearchOptions: configexcel.SearchOptions{Paths: splitCSV(*paths), ProjectConfig: *projectConfig, Query: *query, Regex: *regex, CaseSensitive: *caseSensitive, IncludeHeaders: *includeHeaders}, Replacement: *replacement, Apply: *apply}))
	case "check-formats":
		flags := flag.NewFlagSet("check-formats", flag.ContinueOnError)
		paths := flags.String("paths", "", "comma-separated Excel files or directories")
		projectConfig := flags.String("project-config", "", "mcp233-game-config-excel.txt path")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return printJSONMust(configexcel.CheckTextFormats(splitCSV(*paths), *projectConfig))
	case "deep-validate":
		flags := flag.NewFlagSet("deep-validate", flag.ContinueOnError)
		paths := flags.String("paths", "", "comma-separated Excel files or directories")
		projectConfig := flags.String("project-config", "", "mcp233-game-config-excel.txt path")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return printJSONMust(configexcel.DeepValidate(splitCSV(*paths), *projectConfig))
	case "cache":
		flags := flag.NewFlagSet("cache", flag.ContinueOnError)
		reset := flags.Bool("reset", false, "clear process-local cache")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *reset {
			return printJSON(configexcel.ResetCache())
		}
		return printJSON(configexcel.GetCacheStats())
	case "configure-project":
		flags := flag.NewFlagSet("configure-project", flag.ContinueOnError)
		path := flags.String("file", "mcp233-game-config-excel.txt", "project config path")
		configDirs := flags.String("config-dirs", "", "comma-separated business config directories")
		idColumn := flags.String("id-column", "id", "fixed id column; must be id")
		battleValueScale := flags.Int64("battle-value-scale", 100, "raw battle scale: 100 displays as 1.00")
		battlePatterns := flags.String("battle-attribute-patterns", "", "comma-separated regular expressions for battle fields")
		apply := flags.Bool("apply", false, "write configuration; without this only preview")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return printJSONMust(configexcel.ConfigureProjectWithOptions(configexcel.ConfigureProjectOptions{Path: *path, ConfigDirs: splitCSV(*configDirs), IDColumn: *idColumn, BattleValueScale: *battleValueScale, BattleAttributePatterns: splitCSV(*battlePatterns), Apply: *apply}))
	case "validate-project", "self-check":
		flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
		projectConfig := flags.String("project-config", "mcp233-game-config-excel.txt", "mcp233-game-config-excel.txt path")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return printJSONMust(validation.SelfCheck(*projectConfig))
	case "update-status":
		return printJSONMust(updater.ReadStatus(cliUpdateDirectory()))
	case "update-activate":
		flags := flag.NewFlagSet("update-activate", flag.ContinueOnError)
		version := flags.String("version", "", "local version such as 0.4.0 or auto")
		apply := flags.Bool("apply", false, "write active version policy; without this only preview")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return printJSONMust(updater.SetActiveVersion(cliUpdateDirectory(), *version, *apply))
	case "update-rollback":
		flags := flag.NewFlagSet("update-rollback", flag.ContinueOnError)
		apply := flags.Bool("apply", false, "write previous version policy; without this only preview")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return printJSONMust(updater.Rollback(cliUpdateDirectory(), *apply))
	default:
		return fmt.Errorf("unknown command %q; use serve, inspect, validate, read, upsert, add-column, delete-column, check-column, init-i18n, export-i18n, search, replace, check-formats, deep-validate, cache, configure-project, validate-project, self-check, update-status, update-activate or update-rollback", args[0])
	}
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printJSONMust(value any, err error) error {
	if err != nil {
		return err
	}
	return printJSON(value)
}

func cliUpdateDirectory() string {
	executable, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(executable)
}
