package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel"
	"github.com/neko233-com/mcp233-game-config-excel/internal/mcp"
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
	default:
		return fmt.Errorf("unknown command %q; use serve, inspect, validate, read, upsert, init-i18n or export-i18n", args[0])
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
