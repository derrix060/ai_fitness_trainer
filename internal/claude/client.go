package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

const allowedTools = "mcp__intervals-icu__*,mcp__google-calendar__*,WebSearch,Read,Edit"

type Client struct {
	model   string
	timeout time.Duration
}

func NewClient(model string, timeout time.Duration) *Client {
	return &Client{
		model:   model,
		timeout: timeout,
	}
}

// SendMessage sends a message to Claude CLI and returns (response, sessionID, error).
func (c *Client) SendMessage(ctx context.Context, userText, sessionID string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := []string{
		"-p", userText,
		"--output-format", "stream-json",
		"--verbose",
		"--model", c.model,
		"--allowedTools", allowedTools,
		"--dangerously-skip-permissions",
	}
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}

	sid := sessionID
	if sid == "" {
		sid = "new"
	}
	log.Printf(">>> User: %.200s (session=%s)", userText, sid)

	cmd := exec.CommandContext(ctx, "claude", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("start claude: %w", err)
	}

	var result *StreamEvent
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		switch event.Type {
		case "system":
			logMCPServers(event.MCPServers)
		case "assistant":
			logAssistantMessage(event.Message)
		case "tool_result":
			logToolResult(event.ToolName, event.Content)
		case "result":
			result = &event
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("Claude subprocess timed out after %s", c.timeout)
			return "Sorry, the request timed out. Please try again.", sessionID, nil
		}
		// Retry without session if session-based call failed
		if sessionID != "" {
			log.Printf("Claude failed with session %s, retrying without session", sessionID)
			return c.SendMessage(context.Background(), userText, "")
		}
		return "Sorry, something went wrong processing your message. Please try again.", "", nil
	}

	if result == nil {
		return "Sorry, I received an unexpected response. Please try again.", sessionID, nil
	}

	responseText := result.Result
	newSessionID := result.SessionID
	if newSessionID == "" {
		newSessionID = sessionID
	}
	if responseText == "" {
		responseText = "I didn't generate a response. Please try rephrasing."
	}

	log.Printf("<<< Claude (%d turns, %.1fs, $%.4f, session=%s)",
		result.NumTurns, float64(result.DurationMS)/1000, result.TotalCost, newSessionID)
	log.Printf("<<< Response: %.500s", responseText)

	return responseText, newSessionID, nil
}

func logMCPServers(servers []MCPServer) {
	if len(servers) == 0 {
		return
	}
	parts := make([]string, len(servers))
	for i, s := range servers {
		parts[i] = s.Name + "=" + s.Status
	}
	log.Printf("    MCP: %s", strings.Join(parts, ", "))
}

func logAssistantMessage(msg *AssistantMsg) {
	if msg == nil {
		return
	}
	for _, block := range msg.Content {
		switch block.Type {
		case "tool_use":
			input := string(block.Input)
			if len(input) > 200 {
				input = input[:200]
			}
			log.Printf("    Tool call: %s(%s)", block.Name, input)
		case "text":
			text := block.Text
			if len(text) > 200 {
				text = text[:200]
			}
			if text != "" {
				log.Printf("    Thinking: %s", text)
			}
		}
	}
}

func logToolResult(toolName string, content json.RawMessage) {
	snippet := string(content)
	if len(snippet) > 150 {
		snippet = snippet[:150]
	}
	log.Printf("    Tool result: %s -> %s", toolName, snippet)
}
