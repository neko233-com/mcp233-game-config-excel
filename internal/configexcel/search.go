package configexcel

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SearchOptions controls recursive config Excel searches.
type SearchOptions struct {
	Paths          []string `json:"paths"`
	ProjectConfig  string   `json:"projectConfig"`
	Query          string   `json:"query"`
	Regex          bool     `json:"regex"`
	CaseSensitive  bool     `json:"caseSensitive"`
	IncludeHeaders bool     `json:"includeHeaders"`
	MaxResults     int      `json:"maxResults"`
}

// SearchHit identifies one unique matching cell.
type SearchHit struct {
	File   string `json:"file"`
	Sheet  string `json:"sheet"`
	Cell   string `json:"cell"`
	Column string `json:"column"`
	Value  string `json:"value"`
}

// SearchReport provides unique locations plus deduplicated matching values.
type SearchReport struct {
	FilesScanned  int         `json:"filesScanned"`
	MatchCount    int         `json:"matchCount"`
	UniqueValues  []string    `json:"uniqueValues"`
	Hits          []SearchHit `json:"hits"`
	HitsTruncated bool        `json:"hitsTruncated"`
	Cache         CacheStats  `json:"cache"`
}

// Search recursively finds text in all config233 data cells from explicit paths or project config.
func Search(options SearchOptions) (SearchReport, error) {
	if strings.TrimSpace(options.Query) == "" {
		return SearchReport{}, fmt.Errorf("query is required")
	}
	paths, err := resolveSearchPaths(options.Paths, options.ProjectConfig)
	if err != nil {
		return SearchReport{}, err
	}
	matcher, err := newTextMatcher(options.Query, options.Regex, options.CaseSensitive)
	if err != nil {
		return SearchReport{}, err
	}
	files, err := discoverExcelFiles(paths)
	if err != nil {
		return SearchReport{}, err
	}
	report := SearchReport{FilesScanned: len(files), UniqueValues: []string{}, Hits: []SearchHit{}}
	if options.MaxResults <= 0 {
		options.MaxResults = 100
	}
	seenHits := map[string]bool{}
	seenValues := map[string]bool{}
	for fileIndex := 0; fileIndex < len(files); fileIndex++ {
		if err := searchWorkbook(files[fileIndex], options.IncludeHeaders, options.MaxResults, matcher, &report, seenHits, seenValues); err != nil {
			return SearchReport{}, err
		}
	}
	sort.Strings(report.UniqueValues)
	report.Cache = GetCacheStats()
	return report, nil
}

func resolveSearchPaths(paths []string, projectConfig string) ([]string, error) {
	result := make([]string, 0, len(paths))
	for index := 0; index < len(paths); index++ {
		if strings.TrimSpace(paths[index]) != "" {
			result = appendUniqueString(result, filepath.Clean(paths[index]))
		}
	}
	if strings.TrimSpace(projectConfig) != "" {
		config, err := LoadProjectConfig(projectConfig)
		if err != nil {
			return nil, err
		}
		for index := 0; index < len(config.ConfigDirs); index++ {
			result = appendUniqueString(result, config.ConfigDirs[index])
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("paths or projectConfig is required")
	}
	return result, nil
}

func discoverExcelFiles(paths []string) ([]string, error) {
	result := make([]string, 0)
	for index := 0; index < len(paths); index++ {
		path := paths[index]
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat search path %s: %w", path, err)
		}
		if !info.IsDir() {
			if isExcelPath(path) {
				result = appendUniqueString(result, filepath.Clean(path))
			}
			continue
		}
		err = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && isExcelPath(current) {
				result = appendUniqueString(result, filepath.Clean(current))
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan directory %s: %w", path, err)
		}
	}
	sort.Strings(result)
	return result, nil
}

func isExcelPath(path string) bool {
	if strings.HasPrefix(filepath.Base(path), "~$") {
		return false
	}
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".xlsx"
}

func newTextMatcher(query string, regex bool, caseSensitive bool) (func(string) bool, error) {
	if regex {
		if !caseSensitive {
			query = "(?i)" + query
		}
		pattern, err := regexp.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("invalid regular expression: %w", err)
		}
		return pattern.MatchString, nil
	}
	if !caseSensitive {
		query = strings.ToLower(query)
		return func(value string) bool { return strings.Contains(strings.ToLower(value), query) }, nil
	}
	return func(value string) bool { return strings.Contains(value, query) }, nil
}

func searchWorkbook(path string, includeHeaders bool, maxResults int, matcher func(string) bool, report *SearchReport, seenHits map[string]bool, seenValues map[string]bool) error {
	workbook, _, err := getCachedWorkbook(path)
	if err != nil {
		return err
	}
	for index := 0; index < len(workbook.Cells); index++ {
		cached := workbook.Cells[index]
		if (!includeHeaders && cached.Header) || !matcher(cached.Value) {
			continue
		}
		key := path + "\x00" + cached.Sheet + "\x00" + cached.Cell
		if seenHits[key] {
			continue
		}
		seenHits[key] = true
		report.MatchCount++
		if len(report.Hits) < maxResults {
			report.Hits = append(report.Hits, SearchHit{File: path, Sheet: cached.Sheet, Cell: cached.Cell, Column: cached.Column, Value: cached.Value})
		} else {
			report.HitsTruncated = true
		}
		if !seenValues[cached.Value] {
			seenValues[cached.Value] = true
			report.UniqueValues = append(report.UniqueValues, cached.Value)
		}
	}
	return nil
}
