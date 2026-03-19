package claude

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseSystemEvent(t *testing.T) {
	data := `{"type":"system","mcp_servers":[{"name":"intervals-icu","status":"connected"},{"name":"google-calendar","status":"connected"}]}`
	var event StreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.Type != "system" {
		t.Errorf("type = %q, want %q", event.Type, "system")
	}
	if len(event.MCPServers) != 2 {
		t.Fatalf("mcp_servers len = %d, want 2", len(event.MCPServers))
	}
	if event.MCPServers[0].Name != "intervals-icu" {
		t.Errorf("server[0] = %q, want %q", event.MCPServers[0].Name, "intervals-icu")
	}
}

func TestParseResultEvent(t *testing.T) {
	data := `{"type":"result","result":"Hello!","session_id":"sess-123","duration_ms":1500,"num_turns":1,"total_cost_usd":0.01}`
	var event StreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.Result != "Hello!" {
		t.Errorf("result = %q, want %q", event.Result, "Hello!")
	}
	if event.SessionID != "sess-123" {
		t.Errorf("session_id = %q, want %q", event.SessionID, "sess-123")
	}
	if event.DurationMS != 1500 {
		t.Errorf("duration_ms = %d, want %d", event.DurationMS, 1500)
	}
}

func TestParseAssistantEvent(t *testing.T) {
	data := `{"type":"assistant","message":{"content":[{"type":"text","text":"Thinking..."},{"type":"tool_use","name":"WebSearch","input":{"query":"test"}}]}}`
	var event StreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.Message == nil {
		t.Fatal("message is nil")
	}
	if len(event.Message.Content) != 2 {
		t.Fatalf("content len = %d, want 2", len(event.Message.Content))
	}
	if event.Message.Content[0].Text != "Thinking..." {
		t.Errorf("content[0].text = %q", event.Message.Content[0].Text)
	}
	if event.Message.Content[1].Name != "WebSearch" {
		t.Errorf("content[1].name = %q", event.Message.Content[1].Name)
	}
}

func TestParseToolResultEvent(t *testing.T) {
	data := `{"type":"tool_result","tool_name":"WebSearch","content":"some result text"}`
	var event StreamEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.ToolName != "WebSearch" {
		t.Errorf("tool_name = %q, want %q", event.ToolName, "WebSearch")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("sonnet", 120*time.Second)
	if c.model != "sonnet" {
		t.Errorf("model = %q, want %q", c.model, "sonnet")
	}
	if c.timeout != 120*time.Second {
		t.Errorf("timeout = %v, want 120s", c.timeout)
	}
}
