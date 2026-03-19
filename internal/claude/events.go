package claude

import "encoding/json"

// StreamEvent represents a JSON event from Claude CLI's stream-json output.
type StreamEvent struct {
	Type       string          `json:"type"`
	Result     string          `json:"result,omitempty"`
	SessionID  string          `json:"session_id,omitempty"`
	DurationMS int             `json:"duration_ms,omitempty"`
	NumTurns   int             `json:"num_turns,omitempty"`
	TotalCost  float64         `json:"total_cost_usd,omitempty"`
	MCPServers []MCPServer     `json:"mcp_servers,omitempty"`
	Message    *AssistantMsg   `json:"message,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
}

type MCPServer struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type AssistantMsg struct {
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}
