package server

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func makeRequest(args interface{}) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func TestGetRequiredString(t *testing.T) {
	tests := []struct {
		name    string
		args    interface{}
		key     string
		want    string
		wantErr string
	}{
		{
			name: "valid",
			args: map[string]interface{}{"id": "abc"},
			key:  "id",
			want: "abc",
		},
		{
			name:    "missing key",
			args:    map[string]interface{}{},
			key:     "id",
			wantErr: "missing required argument",
		},
		{
			name:    "empty string rejected",
			args:    map[string]interface{}{"id": ""},
			key:     "id",
			wantErr: "must not be empty",
		},
		{
			name:    "wrong type",
			args:    map[string]interface{}{"id": 42},
			key:     "id",
			wantErr: "must be a string",
		},
		{
			name:    "wrong args container type",
			args:    "not a map",
			key:     "id",
			wantErr: "invalid arguments type",
		},
		{
			name:    "nil args",
			args:    nil,
			key:     "id",
			wantErr: "invalid arguments type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRequiredString(makeRequest(tt.args), tt.key)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestArgsMap(t *testing.T) {
	if got := argsMap(makeRequest(nil)); len(got) != 0 {
		t.Fatalf("expected empty map for nil args, got %v", got)
	}
	if got := argsMap(makeRequest("garbage")); len(got) != 0 {
		t.Fatalf("expected empty map for non-map args, got %v", got)
	}
	in := map[string]interface{}{"x": "y"}
	if got := argsMap(makeRequest(in)); got["x"] != "y" {
		t.Fatalf("expected pass-through map, got %v", got)
	}
}

func TestJsonResultMarshal(t *testing.T) {
	res := jsonResult(map[string]string{"hello": "world"})
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if res.IsError {
		t.Fatalf("expected success result, got error")
	}
}

func TestJsonResultMarshalError(t *testing.T) {
	// channels cannot be marshaled to JSON
	res := jsonResult(make(chan int))
	if res == nil || !res.IsError {
		t.Fatalf("expected error result for unmarshalable value")
	}
}
