package configexcel

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
)

// CacheStats describes in-process index reuse. Cache never writes to project files.
type CacheStats struct {
	Entries int `json:"entries"`
	Hits    int `json:"hits"`
	Misses  int `json:"misses"`
}

type cachedWorkbook struct {
	Size       int64
	ModifiedAt time.Time
	Cells      []cachedCell
}

type cachedCell struct {
	Sheet  string
	Row    int
	Column string
	Cell   string
	Value  string
	Header bool
}

var workbookCache = struct {
	sync.Mutex
	Items  map[string]cachedWorkbook
	Hits   int
	Misses int
}{Items: map[string]cachedWorkbook{}}

// ResetCache clears only the MCP process memory cache.
func ResetCache() CacheStats {
	workbookCache.Lock()
	defer workbookCache.Unlock()
	workbookCache.Items = map[string]cachedWorkbook{}
	workbookCache.Hits = 0
	workbookCache.Misses = 0
	return cacheStatsLocked()
}

// GetCacheStats returns process-local cache statistics.
func GetCacheStats() CacheStats {
	workbookCache.Lock()
	defer workbookCache.Unlock()
	return cacheStatsLocked()
}

func cacheStatsLocked() CacheStats {
	return CacheStats{Entries: len(workbookCache.Items), Hits: workbookCache.Hits, Misses: workbookCache.Misses}
}

func getCachedWorkbook(path string) (cachedWorkbook, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return cachedWorkbook{}, false, err
	}
	workbookCache.Lock()
	cached, found := workbookCache.Items[path]
	if found && cached.Size == info.Size() && cached.ModifiedAt.Equal(info.ModTime()) {
		workbookCache.Hits++
		workbookCache.Unlock()
		return cached, true, nil
	}
	workbookCache.Misses++
	workbookCache.Unlock()
	loaded, err := indexWorkbook(path)
	if err != nil {
		return cachedWorkbook{}, false, err
	}
	loaded.Size = info.Size()
	loaded.ModifiedAt = info.ModTime()
	workbookCache.Lock()
	workbookCache.Items[path] = loaded
	workbookCache.Unlock()
	return loaded, false, nil
}

func invalidateWorkbookCache(path string) {
	workbookCache.Lock()
	defer workbookCache.Unlock()
	delete(workbookCache.Items, path)
}

func indexWorkbook(path string) (cachedWorkbook, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return cachedWorkbook{}, fmt.Errorf("open workbook %s: %w", path, err)
	}
	defer f.Close()
	result := cachedWorkbook{Cells: []cachedCell{}}
	for _, sheet := range f.GetSheetList() {
		columns, columnErr := readColumns(f, sheet)
		if columnErr != nil {
			continue
		}
		rows, rowsErr := f.GetRows(sheet)
		if rowsErr != nil {
			return cachedWorkbook{}, rowsErr
		}
		for rowNumber := 1; rowNumber <= len(rows); rowNumber++ {
			for columnIndex := 0; columnIndex < len(columns); columnIndex++ {
				column := columns[columnIndex]
				cell, cellErr := excelize.CoordinatesToCellName(column.ExcelColumnNumber, rowNumber)
				if cellErr != nil {
					return cachedWorkbook{}, cellErr
				}
				value, valueErr := f.GetCellValue(sheet, cell)
				if valueErr != nil {
					return cachedWorkbook{}, valueErr
				}
				result.Cells = append(result.Cells, cachedCell{Sheet: sheet, Row: rowNumber, Column: column.Name, Cell: cell, Value: value, Header: rowNumber < dataRowNumber})
			}
		}
	}
	return result, nil
}
