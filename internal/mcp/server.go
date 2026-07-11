// Package mcp implements offline JSON-RPC transport for Model Context Protocol.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/neko233-com/mcp233-game-config-excel/internal/configexcel"
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
			"serverInfo":      map[string]string{"name": "mcp233-game-config-excel", "version": "0.2.0"},
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
