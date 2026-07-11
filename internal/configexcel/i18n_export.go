package configexcel

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	I18nExportFormatJSON = "json"
	I18nExportFormatCSV  = "csv"
	I18nExportFormatTSV  = "tsv"
	I18nExportModeFull   = "full"
	I18nExportModeDelta  = "incremental"
)

// I18nExportOptions controls local multilingual extraction from a config233 Excel sheet.
// Incremental mode compares the source file with BaselinePath, another compatible Excel file.
type I18nExportOptions struct {
	Path            string   `json:"path"`
	Sheet           string   `json:"sheet"`
	OutputDir       string   `json:"outputDir"`
	Format          string   `json:"format"`
	Mode            string   `json:"mode"`
	BaselinePath    string   `json:"baselinePath"`
	BaselineSheet   string   `json:"baselineSheet"`
	UIDColumn       string   `json:"uidColumn"`
	LanguageColumns []string `json:"languageColumns"`
}

// I18nExportFile is one generated locale file.
type I18nExportFile struct {
	Locale      string `json:"locale"`
	Path        string `json:"path"`
	UpsertCount int    `json:"upsertCount"`
	DeleteCount int    `json:"deleteCount"`
}

// I18nExportReport describes completed output without returning all translated text over MCP.
type I18nExportReport struct {
	Format string           `json:"format"`
	Mode   string           `json:"mode"`
	Files  []I18nExportFile `json:"files"`
}

type i18nCatalog map[string]map[string]string // locale -> key -> text

type i18nDelta struct {
	Upsert map[string]string `json:"upsert"`
	Delete []string          `json:"delete"`
}

type languageColumn struct {
	Name   string
	Prefix string
	Locale string
}

// ExportI18n extracts local i18n text. JSON full files are {key:text}; CSV/TSV full files use
// key,value. Incremental JSON files are {upsert:{...},delete:[...]}; CSV/TSV add operation.
func ExportI18n(options I18nExportOptions) (I18nExportReport, error) {
	options = normalizeI18nExportOptions(options)
	if options.Path == "" {
		return I18nExportReport{}, fmt.Errorf("path is required")
	}
	if options.OutputDir == "" {
		return I18nExportReport{}, fmt.Errorf("outputDir is required")
	}
	if !isI18nFormat(options.Format) {
		return I18nExportReport{}, fmt.Errorf("unsupported format: %s (use json, csv or tsv)", options.Format)
	}
	if options.Mode != I18nExportModeFull && options.Mode != I18nExportModeDelta {
		return I18nExportReport{}, fmt.Errorf("unsupported mode: %s (use full or incremental)", options.Mode)
	}
	if options.Mode == I18nExportModeDelta && options.BaselinePath == "" {
		return I18nExportReport{}, fmt.Errorf("baselinePath is required for incremental export")
	}

	current, err := readI18nCatalog(options.Path, options.Sheet, options.UIDColumn, options.LanguageColumns)
	if err != nil {
		return I18nExportReport{}, err
	}
	baseline := i18nCatalog{}
	if options.Mode == I18nExportModeDelta {
		baseline, err = readI18nCatalog(options.BaselinePath, options.BaselineSheet, options.UIDColumn, options.LanguageColumns)
		if err != nil {
			return I18nExportReport{}, fmt.Errorf("read baseline: %w", err)
		}
	}
	if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
		return I18nExportReport{}, fmt.Errorf("create output directory: %w", err)
	}

	locales := catalogLocales(current, baseline)
	report := I18nExportReport{Format: options.Format, Mode: options.Mode, Files: make([]I18nExportFile, 0, len(locales))}
	for _, locale := range locales {
		currentValues := current[locale]
		baselineValues := baseline[locale]
		delta := diffI18n(currentValues, baselineValues)
		file := I18nExportFile{
			Locale:      locale,
			Path:        i18nOutputPath(options.OutputDir, options.Path, locale, options.Format),
			UpsertCount: len(delta.Upsert),
			DeleteCount: len(delta.Delete),
		}
		if options.Mode == I18nExportModeFull {
			file.UpsertCount = len(currentValues)
			file.DeleteCount = 0
			if err := writeI18nFull(file.Path, options.Format, currentValues); err != nil {
				return I18nExportReport{}, err
			}
		} else if err := writeI18nDelta(file.Path, options.Format, delta); err != nil {
			return I18nExportReport{}, err
		}
		report.Files = append(report.Files, file)
	}
	return report, nil
}

func normalizeI18nExportOptions(options I18nExportOptions) I18nExportOptions {
	options.Path = strings.TrimSpace(options.Path)
	options.Sheet = strings.TrimSpace(options.Sheet)
	options.OutputDir = strings.TrimSpace(options.OutputDir)
	options.BaselinePath = strings.TrimSpace(options.BaselinePath)
	options.BaselineSheet = strings.TrimSpace(options.BaselineSheet)
	options.UIDColumn = strings.TrimSpace(options.UIDColumn)
	options.Format = strings.ToLower(strings.TrimSpace(options.Format))
	options.Mode = strings.ToLower(strings.TrimSpace(options.Mode))
	if options.UIDColumn == "" {
		options.UIDColumn = "id"
	}
	if options.Format == "" {
		options.Format = I18nExportFormatJSON
	}
	if options.Mode == "" {
		options.Mode = I18nExportModeFull
	}
	return options
}

func readI18nCatalog(path, sheet, uidColumn string, requestedColumns []string) (i18nCatalog, error) {
	inspection, err := Inspect(path, sheet)
	if err != nil {
		return nil, err
	}
	columns := findLanguageColumns(inspection.Columns, requestedColumns)
	if len(columns) == 0 {
		return nil, fmt.Errorf("no language columns found; use names like tips_CN/tips_EN or set languageColumns")
	}
	foundUID := false
	for _, column := range inspection.Columns {
		if column.Name == uidColumn {
			foundUID = true
			break
		}
	}
	if !foundUID {
		return nil, fmt.Errorf("uid column not found: %s", uidColumn)
	}
	rows, err := ReadRows(path, inspection.Sheet, 0)
	if err != nil {
		return nil, err
	}
	usePrefixedKey := hasMultipleI18nFields(columns)
	catalog := i18nCatalog{}
	for _, column := range columns {
		if catalog[column.Locale] == nil {
			catalog[column.Locale] = map[string]string{}
		}
	}
	for rowNumber, row := range rows {
		uid := strings.TrimSpace(row[uidColumn])
		if uid == "" {
			return nil, fmt.Errorf("empty %s in data row %d", uidColumn, rowNumber+dataRowNumber)
		}
		for _, column := range columns {
			key := uid
			if usePrefixedKey {
				key += "." + column.Prefix
			}
			if _, exists := catalog[column.Locale][key]; exists {
				return nil, fmt.Errorf("duplicate i18n key: %s (%s)", key, column.Locale)
			}
			catalog[column.Locale][key] = row[column.Name]
		}
	}
	return catalog, nil
}

func findLanguageColumns(columns []Column, requested []string) []languageColumn {
	requestedLocales := map[string]string{}
	for _, item := range requested {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, locale, hasLocale := strings.Cut(item, "=")
		name = strings.TrimSpace(name)
		if name != "" {
			if hasLocale {
				requestedLocales[name] = strings.TrimSpace(locale)
			} else {
				requestedLocales[name] = ""
			}
		}
	}
	result := make([]languageColumn, 0)
	for _, column := range columns {
		prefix, locale, matched := splitLanguageColumn(column.Name)
		requestedLocale, isRequested := requestedLocales[column.Name]
		if len(requestedLocales) > 0 {
			if !isRequested {
				continue
			}
			if requestedLocale != "" {
				if !matched {
					prefix = column.Name
				}
				locale = requestedLocale
				matched = true
			}
			if !matched {
				continue
			}
			if prefix == "" {
				prefix = column.Name
			}
		} else if !matched {
			continue
		}
		result = append(result, languageColumn{Name: column.Name, Prefix: prefix, Locale: locale})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// splitLanguageColumn accepts config names such as tips_CN, tips_EN, tips_zh_CN and tips_en-US.
// Auto detection is intentionally uppercase-sensitive to avoid treating ordinary snake_case IDs as locales.
func splitLanguageColumn(name string) (prefix, locale string, matched bool) {
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return "", "", false
	}
	last := parts[len(parts)-1]
	if isUpperLanguageCode(last) {
		if len(parts) >= 3 && isLowerLanguageCode(parts[len(parts)-2]) {
			return strings.Join(parts[:len(parts)-2], "_"), parts[len(parts)-2] + "_" + last, len(parts) > 2
		}
		return strings.Join(parts[:len(parts)-1], "_"), last, true
	}
	return "", "", false
}

func isUpperLanguageCode(value string) bool {
	if len(value) < 2 || len(value) > 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func isLowerLanguageCode(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, char := range value {
		if char < 'a' || char > 'z' {
			return false
		}
	}
	return true
}

func catalogLocales(catalogs ...i18nCatalog) []string {
	set := map[string]bool{}
	for _, catalog := range catalogs {
		for locale := range catalog {
			set[locale] = true
		}
	}
	result := make([]string, 0, len(set))
	for locale := range set {
		result = append(result, locale)
	}
	sort.Strings(result)
	return result
}

func hasMultipleI18nFields(columns []languageColumn) bool {
	prefixesByLocale := map[string]map[string]bool{}
	for _, column := range columns {
		if prefixesByLocale[column.Locale] == nil {
			prefixesByLocale[column.Locale] = map[string]bool{}
		}
		prefixesByLocale[column.Locale][column.Prefix] = true
	}
	for _, prefixes := range prefixesByLocale {
		if len(prefixes) > 1 {
			return true
		}
	}
	return false
}

func diffI18n(current, baseline map[string]string) i18nDelta {
	delta := i18nDelta{Upsert: map[string]string{}, Delete: []string{}}
	for key, value := range current {
		if baselineValue, exists := baseline[key]; !exists || baselineValue != value {
			delta.Upsert[key] = value
		}
	}
	for key := range baseline {
		if _, exists := current[key]; !exists {
			delta.Delete = append(delta.Delete, key)
		}
	}
	sort.Strings(delta.Delete)
	return delta
}

func i18nOutputPath(outputDir, sourcePath, locale, format string) string {
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	return filepath.Join(outputDir, base+"."+locale+"."+format)
}

func writeI18nFull(path, format string, values map[string]string) error {
	if format == I18nExportFormatJSON {
		encoded, err := json.MarshalIndent(values, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(encoded, '\n'), 0o644)
	}
	rows := make([][]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		rows = append(rows, []string{key, values[key]})
	}
	return writeDelimited(path, format, []string{"key", "value"}, rows)
}

func writeI18nDelta(path, format string, delta i18nDelta) error {
	if format == I18nExportFormatJSON {
		encoded, err := json.MarshalIndent(delta, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(encoded, '\n'), 0o644)
	}
	rows := make([][]string, 0, len(delta.Upsert)+len(delta.Delete))
	for _, key := range sortedKeys(delta.Upsert) {
		rows = append(rows, []string{"upsert", key, delta.Upsert[key]})
	}
	for _, key := range delta.Delete {
		rows = append(rows, []string{"delete", key, ""})
	}
	return writeDelimited(path, format, []string{"operation", "key", "value"}, rows)
}

func writeDelimited(path, format string, header []string, rows [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if format == I18nExportFormatTSV {
		writer.Comma = '\t'
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	if err := writer.WriteAll(rows); err != nil {
		return err
	}
	return writer.Error()
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func isI18nFormat(format string) bool {
	return format == I18nExportFormatJSON || format == I18nExportFormatCSV || format == I18nExportFormatTSV
}
