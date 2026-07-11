package mcp

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestServeToolsList(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	output := new(bytes.Buffer)
	if err := Serve(input, output); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var response map[string]any
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; raw=%s", err, output.String())
	}
	result := response["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 6 {
		t.Fatalf("tool count = %d, want 6", len(tools))
	}
}

func TestServeIgnoresNotification(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	output := new(bytes.Buffer)
	if err := Serve(input, output); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("notification must not receive response: %q", output.String())
	}
}
