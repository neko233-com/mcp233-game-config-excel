// Package mcp implements offline JSON-RPC transport for Model Context Protocol.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel"
	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel/validation"
	"github.com/neko233-com/mcp233-game-config-excel/internal/updater"
)

const protocolVersion = "2025-06-18"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads JSON-RPC messages from stdin and emits one JSON response per request.
// It intentionally opens only local paths supplied by caller; no HTTP or network client exists.
func Serve(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := encoder.Encode(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}}); err != nil {
				return err
			}
			continue
		}
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue // JSON-RPC notification: never respond.
		}
		res := dispatch(req)
		if err := encoder.Encode(res); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read MCP stdin: %w", err)
	}
	return nil
}

func dispatch(req request) response {
	res := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "mcp233-game-config-excel", "version": "0.4.0"},
		}
	case "ping":
		res.Result = map[string]any{}
	case "tools/list":
		res.Result = map[string]any{"tools": tools()}
	case "tools/call":
		result, err := callTool(req.Params)
		if err != nil {
			res.Error = &rpcError{Code: -32000, Message: err.Error()}
		} else {
			res.Result = result
		}
	default:
		res.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return res
}

func tools() []map[string]any {
	pathProperty := map[string]any{"type": "string", "description": "Local .xlsx file path"}
	sheetProperty := map[string]any{"type": "string", "description": "Optional sheet name; first sheet when omitted"}
	return []map[string]any{
		{
			"name": "config_excel_inspect", "description": "Inspect config233 SERVER/TYPE/CLIENT rows and data count from local Excel.",
			"inputSchema": schema(map[string]any{"path": pathProperty, "sheet": sheetProperty}, "path"),
		},
		{
			"name": "config_excel_validate", "description": "Validate config233 fixed rows, exact server columns and duplicate UID values.",
			"inputSchema": schema(map[string]any{
				"path": pathProperty, "sheet": sheetProperty,
				"expectedColumns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"uidColumn":       map[string]any{"type": "string", "default": "id"},
			}, "path"),
		},
		{
			"name": "config_excel_read_rows", "description": "Read local Excel configuration data rows as string values, preserving config text.",
			"inputSchema": schema(map[string]any{
				"path": pathProperty, "sheet": sheetProperty,
				"limit": map[string]any{"type": "integer", "minimum": 1},
			}, "path"),
		},
		{
			"name": "config_excel_upsert_row", "description": "Create or update one local config233 data row by UID. This writes the supplied Excel file.",
			"inputSchema": schema(map[string]any{
				"path": pathProperty, "sheet": sheetProperty,
				"uidColumn": map[string]any{"type": "string", "default": "id"},
				"uid":       map[string]any{"type": "string"},
				"values":    map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			}, "path", "uid", "values"),
		},
		{
			"name": "config_excel_add_column", "description": "Insert a config233 column, preserve neighbouring style and apply Excel text format by default. This writes the supplied Excel file.",
			"inputSchema": schema(map[string]any{
				"path": pathProperty, "sheet": sheetProperty,
				"name":        map[string]any{"type": "string", "description": "SERVER field name"},
				"clientName":  map[string]any{"type": "string", "description": "CLIENT field name; defaults to name"},
				"displayName": map[string]any{"type": "string", "description": "row 2 display name; defaults to clientName"},
				"type":        map[string]any{"type": "string", "default": "string", "description": "config233 TYPE; defaults to string"},
				"comment":     map[string]any{"type": "string", "description": "Row 1 field comment"},
				"afterColumn": map[string]any{"type": "string", "description": "Insert after this SERVER field; defaults to append"},
			}, "path", "name"),
		},
		{
			"name": "config_excel_delete_column", "description": "Delete one config233 SERVER column and its aligned header/data cells. This writes the supplied Excel file.",
			"inputSchema": schema(map[string]any{"path": pathProperty, "sheet": sheetProperty, "name": map[string]any{"type": "string", "description": "SERVER field name"}}, "path", "name"),
		},
		{
			"name": "config_excel_check_column_format", "description": "Inspect one config233 column and report TYPE plus per-cell Excel text-format violations.",
			"inputSchema": schema(map[string]any{
				"path": pathProperty, "sheet": sheetProperty,
				"name":        map[string]any{"type": "string", "description": "SERVER field name"},
				"requireText": map[string]any{"type": "boolean", "default": true, "description": "Require TYPE string and Excel text cells"},
			}, "path", "name"),
		},
		{
			"name": "config_excel_search", "description": "Recursively search config233 Excel data cells by literal value or regular expression. Results include deduplicated values and exact file/sheet/cell locations.",
			"inputSchema": schema(map[string]any{
				"paths":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Excel files or directories to scan recursively"},
				"projectConfig": map[string]any{"type": "string", "description": "mcp233-game-config-excel.txt path; supports properties and TOML config_dirs"},
				"query":         map[string]any{"type": "string", "description": "Literal text or regular expression, for example icon_handbook_.*"},
				"regex":         map[string]any{"type": "boolean", "default": false}, "caseSensitive": map[string]any{"type": "boolean", "default": false}, "includeHeaders": map[string]any{"type": "boolean", "default": false},
			}, "query"),
		},
		{
			"name": "config_excel_replace", "description": "Preview or apply a literal/regular-expression replacement across config Excel files. Always returns per-cell before/after Markdown comparison; apply=false is safe preview.",
			"inputSchema": schema(map[string]any{
				"paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "projectConfig": map[string]any{"type": "string"},
				"query": map[string]any{"type": "string"}, "replacement": map[string]any{"type": "string"}, "regex": map[string]any{"type": "boolean", "default": false}, "caseSensitive": map[string]any{"type": "boolean", "default": false}, "includeHeaders": map[string]any{"type": "boolean", "default": false}, "apply": map[string]any{"type": "boolean", "default": false, "description": "Only true writes files after review"},
			}, "query", "replacement"),
		},
		{
			"name": "config_excel_check_text_formats", "description": "Check every string field in recursive config Excel files. Reports non-text cells and likely Excel date auto-conversions.",
			"inputSchema": schema(map[string]any{
				"paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "projectConfig": map[string]any{"type": "string"},
			}),
		},
		{
			"name": "config_excel_deep_validate", "description": "Run non-mutating automated validation across every configured workbook: config233 structure, string cell type risks, and Excel-open lock markers. Open workbooks remain searchable but must be closed before writes.",
			"inputSchema": schema(map[string]any{
				"paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "projectConfig": map[string]any{"type": "string"},
			}),
		},
		{
			"name": "config_excel_cache", "description": "Inspect or reset the process-local workbook index cache used to reduce repeated Excel parsing and MCP response cost. reset=true only clears memory.",
			"inputSchema": schema(map[string]any{"reset": map[string]any{"type": "boolean", "default": false}}),
		},
		{
			"name": "config_excel_configure_project", "description": "Let an agent preview or apply this MCP's project configuration through dialogue. Writes only mcp233-game-config-excel.txt when apply=true and returns before/after table.",
			"inputSchema": schema(map[string]any{
				"path": map[string]any{"type": "string", "description": "mcp233-game-config-excel.txt path"}, "configDirs": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Business Excel directories"}, "idColumn": map[string]any{"type": "string", "default": "id", "description": "Fixed project rule: must be exact lowercase id"}, "battleValueScale": map[string]any{"type": "integer", "default": 100, "description": "Raw battle value scale: 100 means in-game display 1.00"}, "battleAttributePatterns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Regular expressions matched against SERVER/CLIENT battle field names"}, "apply": map[string]any{"type": "boolean", "default": false},
			}, "path", "configDirs"),
		},
		{
			"name": "config_excel_validate_project", "description": "Comprehensive non-mutating project validation: config233 structure, exact unique id column rule, configured battle fields, raw 100 = displayed 1.00 convention, text-format risks, and Excel locks.",
			"inputSchema": schema(map[string]any{"projectConfig": map[string]any{"type": "string", "description": "mcp233-game-config-excel.txt path"}}, "projectConfig"),
		},
		{
			"name": "config_excel_self_check", "description": "MCP deep self-check using project rules. Returns configuration, validation summary, locked files, id failures and battle-value failures without writing Excel.",
			"inputSchema": schema(map[string]any{"projectConfig": map[string]any{"type": "string", "description": "mcp233-game-config-excel.txt path"}}, "projectConfig"),
		},
		{
			"name": "config_excel_update_status", "description": "Inspect offline local MCP versions and automatic-selection policy. Network update/download is intentionally disabled.",
			"inputSchema": schema(map[string]any{}),
		},
		{
			"name": "config_excel_update_activate", "description": "Preview or activate an already-built local MCP version. version=auto selects newest local version on next MCP restart; apply=false is preview.",
			"inputSchema": schema(map[string]any{"version": map[string]any{"type": "string", "description": "Version such as 0.4.0 or auto"}, "apply": map[string]any{"type": "boolean", "default": false}}, "version"),
		},
		{
			"name": "config_excel_update_rollback", "description": "Preview or activate the previous local MCP version on next restart. Does not delete any binary.",
			"inputSchema": schema(map[string]any{"apply": map[string]any{"type": "boolean", "default": false}}),
		},
		{
			"name": "config_excel_create_i18n_template", "description": "Create I18nTipsConfig-compatible Excel: id:string and tips_CN:string. This writes a new local file.",
			"inputSchema": schema(map[string]any{"path": pathProperty, "sheet": sheetProperty}, "path"),
		},
		{
			"name": "config_excel_export_i18n", "description": "Extract multilingual columns such as tips_CN/tips_EN to local JSON, CSV or TSV. Full exports all values; incremental compares baseline Excel and includes upsert/delete changes.",
			"inputSchema": schema(map[string]any{
				"path":            pathProperty,
				"sheet":           sheetProperty,
				"outputDir":       map[string]any{"type": "string", "description": "Local output directory"},
				"format":          map[string]any{"type": "string", "enum": []string{"json", "csv", "tsv"}, "default": "json"},
				"mode":            map[string]any{"type": "string", "enum": []string{"full", "incremental"}, "default": "full"},
				"baselinePath":    map[string]any{"type": "string", "description": "Required in incremental mode: previous compatible Excel file"},
				"baselineSheet":   sheetProperty,
				"uidColumn":       map[string]any{"type": "string", "default": "id"},
				"languageColumns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional column or column=locale mapping; auto-detects *_CN, *_EN, *_zh_CN"},
			}, "path", "outputDir"),
		},
	}
}

func schema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

type toolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type commonArguments struct {
	Path  string `json:"path"`
	Sheet string `json:"sheet"`
}

func callTool(raw json.RawMessage) (any, error) {
	var call toolCall
	if err := json.Unmarshal(raw, &call); err != nil {
		return nil, fmt.Errorf("invalid tools/call params: %w", err)
	}
	switch call.Name {
	case "config_excel_inspect":
		var args commonArguments
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		inspection, err := configexcel.Inspect(args.Path, args.Sheet)
		return toolResult(inspection, err)
	case "config_excel_validate":
		var args struct {
			commonArguments
			ExpectedColumns []string `json:"expectedColumns"`
			UIDColumn       string   `json:"uidColumn"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		validation, err := configexcel.Validate(args.Path, args.Sheet, args.ExpectedColumns, args.UIDColumn)
		return toolResult(validation, err)
	case "config_excel_read_rows":
		var args struct {
			commonArguments
			Limit int `json:"limit"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		rows, err := configexcel.ReadRows(args.Path, args.Sheet, args.Limit)
		return toolResult(rows, err)
	case "config_excel_upsert_row":
		var args struct {
			commonArguments
			UIDColumn string            `json:"uidColumn"`
			UID       string            `json:"uid"`
			Values    map[string]string `json:"values"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		created, err := configexcel.UpsertRow(args.Path, args.Sheet, args.UIDColumn, args.UID, args.Values)
		return toolResult(map[string]any{"created": created, "uid": args.UID}, err)
	case "config_excel_add_column":
		var args struct {
			commonArguments
			Name        string `json:"name"`
			ClientName  string `json:"clientName"`
			DisplayName string `json:"displayName"`
			Type        string `json:"type"`
			Comment     string `json:"comment"`
			AfterColumn string `json:"afterColumn"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		err := configexcel.AddColumn(args.Path, args.Sheet, configexcel.ColumnDefinition{
			Name: args.Name, ClientName: args.ClientName, DisplayName: args.DisplayName, Type: args.Type, Comment: args.Comment,
		}, args.AfterColumn)
		return toolResult(map[string]string{"name": args.Name}, err)
	case "config_excel_delete_column":
		var args struct {
			commonArguments
			Name string `json:"name"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		err := configexcel.DeleteColumn(args.Path, args.Sheet, args.Name)
		return toolResult(map[string]string{"name": args.Name}, err)
	case "config_excel_check_column_format":
		var args struct {
			commonArguments
			Name        string `json:"name"`
			RequireText *bool  `json:"requireText"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		requireText := true
		if args.RequireText != nil {
			requireText = *args.RequireText
		}
		result, err := configexcel.CheckColumnFormat(args.Path, args.Sheet, args.Name, requireText)
		return toolResult(result, err)
	case "config_excel_search":
		var args configexcel.SearchOptions
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		result, err := configexcel.Search(args)
		return toolResult(result, err)
	case "config_excel_replace":
		var args configexcel.ReplaceOptions
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		result, err := configexcel.Replace(args)
		return toolResult(result, err)
	case "config_excel_check_text_formats":
		var args struct {
			Paths         []string `json:"paths"`
			ProjectConfig string   `json:"projectConfig"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		result, err := configexcel.CheckTextFormats(args.Paths, args.ProjectConfig)
		return toolResult(result, err)
	case "config_excel_deep_validate":
		var args struct {
			Paths         []string `json:"paths"`
			ProjectConfig string   `json:"projectConfig"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		result, err := configexcel.DeepValidate(args.Paths, args.ProjectConfig)
		return toolResult(result, err)
	case "config_excel_cache":
		var args struct {
			Reset bool `json:"reset"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		if args.Reset {
			return toolResult(configexcel.ResetCache(), nil)
		}
		return toolResult(configexcel.GetCacheStats(), nil)
	case "config_excel_configure_project":
		var args struct {
			Path                    string   `json:"path"`
			ConfigDirs              []string `json:"configDirs"`
			IDColumn                string   `json:"idColumn"`
			BattleValueScale        int64    `json:"battleValueScale"`
			BattleAttributePatterns []string `json:"battleAttributePatterns"`
			Apply                   bool     `json:"apply"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		result, err := configexcel.ConfigureProjectWithOptions(configexcel.ConfigureProjectOptions{Path: args.Path, ConfigDirs: args.ConfigDirs, IDColumn: args.IDColumn, BattleValueScale: args.BattleValueScale, BattleAttributePatterns: args.BattleAttributePatterns, Apply: args.Apply})
		return toolResult(result, err)
	case "config_excel_validate_project", "config_excel_self_check":
		var args struct {
			ProjectConfig string `json:"projectConfig"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		result, err := validation.SelfCheck(args.ProjectConfig)
		return toolResult(result, err)
	case "config_excel_update_status":
		result, err := updater.ReadStatus(updateDirectory())
		return toolResult(result, err)
	case "config_excel_update_activate":
		var args struct {
			Version string `json:"version"`
			Apply   bool   `json:"apply"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		result, err := updater.SetActiveVersion(updateDirectory(), args.Version, args.Apply)
		return toolResult(result, err)
	case "config_excel_update_rollback":
		var args struct {
			Apply bool `json:"apply"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		result, err := updater.Rollback(updateDirectory(), args.Apply)
		return toolResult(result, err)
	case "config_excel_create_i18n_template":
		var args commonArguments
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		err := configexcel.CreateI18nTemplate(args.Path, args.Sheet)
		return toolResult(map[string]string{"path": args.Path}, err)
	case "config_excel_export_i18n":
		var args configexcel.I18nExportOptions
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return nil, err
		}
		report, err := configexcel.ExportI18n(args)
		return toolResult(report, err)
	default:
		return nil, fmt.Errorf("unknown tool: %s", call.Name)
	}
}

func toolResult(value any, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string]any{"content": []map[string]string{{"type": "text", "text": string(encoded)}}}, nil
}

func updateDirectory() string {
	executable, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(executable)
}
