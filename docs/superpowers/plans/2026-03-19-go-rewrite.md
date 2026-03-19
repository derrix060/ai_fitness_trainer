# Go Rewrite Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the Python Telegram bot and replace the npm Google Calendar MCP server with Go, eliminating Python and Node.js runtime dependencies entirely.

**Architecture:** Two Go binaries — the main Telegram bot (`cmd/bot`) that shells out to Claude CLI for AI responses, and a Google Calendar MCP server (`cmd/gcal-mcp`) that Claude CLI spawns as a tool server. The existing `intervals-mcp` Go binary stays as-is. All concurrency uses goroutines; SQLite via pure-Go driver (no CGO) for ARM cross-compilation.

**Tech Stack:** Go 1.25+, `github.com/go-telegram/bot` (v1.20.0), `modernc.org/sqlite`, `github.com/go-co-op/gocron/v2`, `github.com/mark3labs/mcp-go` (v0.44.1), `google.golang.org/api/calendar/v3`, `golang.org/x/oauth2`

**API Verification:** All library APIs verified against source code (2026-03-19). MCP tool handlers use typed accessors (`req.GetString`, `req.GetFloat`, etc.) per mcp-go v0.44.1 API.

---

## File Structure

```
ai_fitness_trainer/
├── cmd/
│   ├── bot/
│   │   └── main.go                    # CREATE — Application entrypoint
│   └── gcal-mcp/
│       └── main.go                    # CREATE — Google Calendar MCP server + auth CLI
├── internal/
│   ├── config/
│   │   ├── config.go                  # CREATE — Config loading from env vars
│   │   └── config_test.go             # CREATE — Config tests
│   ├── store/
│   │   ├── store.go                   # CREATE — SQLite session/KV/activity store
│   │   └── store_test.go              # CREATE — Store tests
│   ├── claude/
│   │   ├── client.go                  # CREATE — Claude CLI subprocess wrapper
│   │   ├── events.go                  # CREATE — Stream JSON event types
│   │   └── client_test.go             # CREATE — JSON parsing tests
│   ├── telegram/
│   │   ├── bot.go                     # CREATE — Telegram bot setup + handlers
│   │   ├── message.go                 # CREATE — Message splitting + formatting
│   │   └── message_test.go            # CREATE — Message splitting tests
│   └── scheduler/
│       ├── scheduler.go               # CREATE — Cron/interval jobs
│       └── prompts.go                 # CREATE — Prompt templates
├── go.mod                             # CREATE — Go module definition
├── Dockerfile                         # MODIFY — Go-only multi-stage build
├── docker-compose.yml                 # MODIFY — Remove Python/Node references
├── .mcp.json                          # MODIFY — Point to Go gcal-mcp binary
├── CLAUDE.md                          # UNCHANGED
├── .env.example                       # UNCHANGED
├── config/                            # UNCHANGED
├── data/                              # UNCHANGED (SQLite schema is compatible)
├── profile/                           # UNCHANGED
├── docs/
│   └── google-calendar-setup.md       # MODIFY — Replace npx commands with Go binary
└── src/                               # DELETE — Python source (after Go is working)
```

---

## Chunk 1: Foundation

### Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `cmd/bot/main.go` (placeholder)
- Create: `cmd/gcal-mcp/main.go` (placeholder)

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p cmd/bot cmd/gcal-mcp internal/config internal/store internal/claude internal/telegram internal/scheduler
```

- [ ] **Step 2: Initialize Go module**

```bash
go mod init github.com/derrix060/ai-fitness-trainer
```

- [ ] **Step 3: Create placeholder main files**

`cmd/bot/main.go`:
```go
package main

func main() {}
```

`cmd/gcal-mcp/main.go`:
```go
package main

func main() {}
```

- [ ] **Step 4: Add dependencies**

```bash
go get github.com/go-telegram/bot@latest
go get github.com/joho/godotenv@latest
go get modernc.org/sqlite@latest
go get github.com/go-co-op/gocron/v2@latest
go get github.com/mark3labs/mcp-go@latest
go get golang.org/x/oauth2@latest
go get google.golang.org/api@latest
```

- [ ] **Step 5: Verify module compiles**

```bash
go build ./...
```
Expected: Success, no errors.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/
git commit -m "feat: initialize Go module with dependencies"
```

---

### Task 2: Config Package (TDD)

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/config/config_test.go`:
```go
package config

import (
	"os"
	"testing"
)

func clearEnv() {
	for _, key := range []string{
		"TELEGRAM_BOT_TOKEN", "ALLOWED_TELEGRAM_USER_IDS", "CLAUDE_MODEL",
		"CLAUDE_TIMEOUT", "INTERVALS_API_KEY", "INTERVALS_ATHLETE_ID",
		"DATA_DIR", "BRIEFING_HOUR", "BRIEFING_MINUTE", "TZ", "LOG_LEVEL",
	} {
		os.Unsetenv(key)
	}
}

func TestLoad_MissingToken(t *testing.T) {
	clearEnv()
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing TELEGRAM_BOT_TOKEN")
	}
}

func TestLoad_MissingTZ(t *testing.T) {
	clearEnv()
	os.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing TZ")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv()
	os.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	os.Setenv("TZ", "UTC")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.TelegramBotToken != "tok" {
		t.Errorf("TelegramBotToken = %q, want %q", cfg.TelegramBotToken, "tok")
	}
	if cfg.ClaudeModel != "sonnet" {
		t.Errorf("ClaudeModel = %q, want %q", cfg.ClaudeModel, "sonnet")
	}
	if cfg.ClaudeTimeout != 120 {
		t.Errorf("ClaudeTimeout = %d, want %d", cfg.ClaudeTimeout, 120)
	}
	if cfg.DataDir != "data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "data")
	}
	if cfg.BriefingHour != 7 {
		t.Errorf("BriefingHour = %d, want %d", cfg.BriefingHour, 7)
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "INFO")
	}
}

func TestLoad_AllowedUserIDs(t *testing.T) {
	clearEnv()
	os.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	os.Setenv("TZ", "UTC")
	os.Setenv("ALLOWED_TELEGRAM_USER_IDS", "123, 456, 789")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, id := range []int64{123, 456, 789} {
		if !cfg.IsAllowed(id) {
			t.Errorf("user %d should be allowed", id)
		}
	}
	if cfg.IsAllowed(999) {
		t.Error("user 999 should not be allowed")
	}
}

func TestLoad_InvalidUserID(t *testing.T) {
	clearEnv()
	os.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	os.Setenv("TZ", "UTC")
	os.Setenv("ALLOWED_TELEGRAM_USER_IDS", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid user ID")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv()
	os.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	os.Setenv("TZ", "Europe/Lisbon")
	os.Setenv("CLAUDE_MODEL", "opus")
	os.Setenv("CLAUDE_TIMEOUT", "300")
	os.Setenv("BRIEFING_HOUR", "8")
	os.Setenv("BRIEFING_MINUTE", "30")
	os.Setenv("DATA_DIR", "/tmp/data")
	os.Setenv("LOG_LEVEL", "DEBUG")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ClaudeModel != "opus" {
		t.Errorf("ClaudeModel = %q, want %q", cfg.ClaudeModel, "opus")
	}
	if cfg.ClaudeTimeout != 300 {
		t.Errorf("ClaudeTimeout = %d, want %d", cfg.ClaudeTimeout, 300)
	}
	if cfg.BriefingHour != 8 {
		t.Errorf("BriefingHour = %d, want %d", cfg.BriefingHour, 8)
	}
	if cfg.BriefingMinute != 30 {
		t.Errorf("BriefingMinute = %d, want %d", cfg.BriefingMinute, 30)
	}
	if cfg.Timezone != "Europe/Lisbon" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone, "Europe/Lisbon")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/config/ -v
```
Expected: FAIL — `Load` function not found.

- [ ] **Step 3: Implement config.go**

`internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken   string
	AllowedUserIDs     map[int64]struct{}
	ClaudeModel        string
	ClaudeTimeout      int
	IntervalsAPIKey    string
	IntervalsAthleteID string
	DataDir            string
	BriefingHour       int
	BriefingMinute     int
	Timezone           string
	LogLevel           string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	tz := os.Getenv("TZ")
	if tz == "" {
		return nil, fmt.Errorf("TZ is required")
	}

	allowedIDs := make(map[int64]struct{})
	for _, s := range strings.Split(os.Getenv("ALLOWED_TELEGRAM_USER_IDS"), ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID %q: %w", s, err)
		}
		allowedIDs[id] = struct{}{}
	}

	return &Config{
		TelegramBotToken:   token,
		AllowedUserIDs:     allowedIDs,
		ClaudeModel:        envOr("CLAUDE_MODEL", "sonnet"),
		ClaudeTimeout:      envInt("CLAUDE_TIMEOUT", 120),
		IntervalsAPIKey:    os.Getenv("INTERVALS_API_KEY"),
		IntervalsAthleteID: os.Getenv("INTERVALS_ATHLETE_ID"),
		DataDir:            envOr("DATA_DIR", "data"),
		BriefingHour:       envInt("BRIEFING_HOUR", 7),
		BriefingMinute:     envInt("BRIEFING_MINUTE", 0),
		Timezone:           tz,
		LogLevel:           envOr("LOG_LEVEL", "INFO"),
	}, nil
}

func (c *Config) IsAllowed(userID int64) bool {
	_, ok := c.AllowedUserIDs[userID]
	return ok
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	s := os.Getenv(key)
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/ -v
```
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package with env var loading"
```

---

### Task 3: Store Package (TDD)

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/store/store_test.go`:
```go
package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSession_SaveGetDelete(t *testing.T) {
	s := newTestStore(t)

	// No session initially
	sid, err := s.GetSession(123)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sid != "" {
		t.Fatalf("expected empty, got %q", sid)
	}

	// Save
	if err := s.SaveSession(123, "sess-abc"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	sid, _ = s.GetSession(123)
	if sid != "sess-abc" {
		t.Fatalf("expected %q, got %q", "sess-abc", sid)
	}

	// Overwrite
	s.SaveSession(123, "sess-def")
	sid, _ = s.GetSession(123)
	if sid != "sess-def" {
		t.Fatalf("expected %q, got %q", "sess-def", sid)
	}

	// Delete
	s.DeleteSession(123)
	sid, _ = s.GetSession(123)
	if sid != "" {
		t.Fatalf("expected empty after delete, got %q", sid)
	}
}

func TestKV_SetAndGet(t *testing.T) {
	s := newTestStore(t)

	val, _ := s.GetValue("k1")
	if val != "" {
		t.Fatalf("expected empty, got %q", val)
	}

	s.SetValue("k1", "v1")
	val, _ = s.GetValue("k1")
	if val != "v1" {
		t.Fatalf("expected %q, got %q", "v1", val)
	}

	// Overwrite
	s.SetValue("k1", "v2")
	val, _ = s.GetValue("k1")
	if val != "v2" {
		t.Fatalf("expected %q, got %q", "v2", val)
	}
}

func TestAnalyzedActivities(t *testing.T) {
	s := newTestStore(t)

	ids, _ := s.GetAnalyzedActivities()
	if len(ids) != 0 {
		t.Fatalf("expected empty, got %d", len(ids))
	}

	s.MarkActivityAnalyzed("i123")
	s.MarkActivityAnalyzed("i456")
	s.MarkActivityAnalyzed("i123") // duplicate — should be ignored

	ids, _ = s.GetAnalyzedActivities()
	if len(ids) != 2 {
		t.Fatalf("expected 2, got %d", len(ids))
	}
	if _, ok := ids["i123"]; !ok {
		t.Error("expected i123")
	}
	if _, ok := ids["i456"]; !ok {
		t.Error("expected i456")
	}
}

func TestGetLastActivityCheck_Default(t *testing.T) {
	s := newTestStore(t)
	val := s.GetLastActivityCheck()
	if val == "" {
		t.Fatal("expected non-empty date")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/store/ -v
```
Expected: FAIL — `Store` type not found.

- [ ] **Step 3: Implement store.go**

`internal/store/store.go`:
```go
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.ensureTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) ensureTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			user_id INTEGER PRIMARY KEY,
			session_id TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS kv (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS analyzed_activities (
			activity_id TEXT PRIMARY KEY,
			analyzed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (s *Store) GetSession(userID int64) (string, error) {
	var sid string
	err := s.db.QueryRow(
		"SELECT session_id FROM sessions WHERE user_id = ?", userID,
	).Scan(&sid)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return sid, err
}

func (s *Store) SaveSession(userID int64, sessionID string) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (user_id, session_id, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			session_id = excluded.session_id,
			updated_at = CURRENT_TIMESTAMP
	`, userID, sessionID)
	return err
}

func (s *Store) DeleteSession(userID int64) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

func (s *Store) GetValue(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM kv WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetValue(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO kv (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func (s *Store) GetAnalyzedActivities() (map[string]struct{}, error) {
	rows, err := s.db.Query("SELECT activity_id FROM analyzed_activities")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = struct{}{}
	}
	return ids, rows.Err()
}

func (s *Store) MarkActivityAnalyzed(activityID string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO analyzed_activities (activity_id) VALUES (?)",
		activityID,
	)
	return err
}

func (s *Store) GetLastActivityCheck() string {
	val, err := s.GetValue("last_activity_check")
	if err != nil || val == "" {
		return time.Now().Format("2006-01-02")
	}
	return val
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/store/ -v
```
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/
git commit -m "feat: add SQLite store for sessions, KV, and activities"
```

---

## Chunk 2: Claude Client

### Task 4: Stream Event Types

**Files:**
- Create: `internal/claude/events.go`

- [ ] **Step 1: Create event types**

`internal/claude/events.go`:
```go
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
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/claude/
```
Expected: Success.

- [ ] **Step 3: Commit**

```bash
git add internal/claude/events.go
git commit -m "feat: add Claude CLI stream event types"
```

---

### Task 5: Claude Client (TDD)

**Files:**
- Create: `internal/claude/client.go`
- Test: `internal/claude/client_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/claude/client_test.go`:
```go
package claude

import (
	"encoding/json"
	"testing"
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
	c := NewClient("sonnet", 120)
	if c.model != "sonnet" {
		t.Errorf("model = %q, want %q", c.model, "sonnet")
	}
	if c.timeout.Seconds() != 120 {
		t.Errorf("timeout = %v, want 120s", c.timeout)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/claude/ -v
```
Expected: FAIL — `NewClient` not found.

- [ ] **Step 3: Implement client.go**

`internal/claude/client.go`:
```go
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

func NewClient(model string, timeoutSec int) *Client {
	return &Client{
		model:   model,
		timeout: time.Duration(timeoutSec) * time.Second,
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/claude/ -v
```
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/claude/
git commit -m "feat: add Claude CLI subprocess client with stream-json parsing"
```

---

## Chunk 3: Telegram Bot

### Task 6: Message Splitting (TDD)

**Files:**
- Create: `internal/telegram/message.go`
- Test: `internal/telegram/message_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/telegram/message_test.go`:
```go
package telegram

import (
	"strings"
	"testing"
)

func TestSplitMessage_Short(t *testing.T) {
	chunks := SplitMessage("hello")
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Errorf("got %v", chunks)
	}
}

func TestSplitMessage_ExactLimit(t *testing.T) {
	text := strings.Repeat("a", MaxTelegramLength)
	chunks := SplitMessage(text)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestSplitMessage_SplitsAtParagraph(t *testing.T) {
	part1 := strings.Repeat("a", 2000)
	part2 := strings.Repeat("b", 2000)
	part3 := strings.Repeat("c", 2000)
	text := part1 + "\n\n" + part2 + "\n\n" + part3

	chunks := SplitMessage(text)
	if len(chunks) < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > MaxTelegramLength {
			t.Errorf("chunk[%d] len %d exceeds limit", i, len(c))
		}
	}
}

func TestSplitMessage_SplitsAtNewline(t *testing.T) {
	text := strings.Repeat("a", 4000) + "\n" + strings.Repeat("b", 4000)
	chunks := SplitMessage(text)
	if len(chunks) < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > MaxTelegramLength {
			t.Errorf("chunk[%d] len %d exceeds limit", i, len(c))
		}
	}
}

func TestSplitMessage_HardCut(t *testing.T) {
	text := strings.Repeat("x", MaxTelegramLength*2+100)
	chunks := SplitMessage(text)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != MaxTelegramLength {
		t.Errorf("chunk[0] len = %d, want %d", len(chunks[0]), MaxTelegramLength)
	}
}

func TestSplitMessage_Empty(t *testing.T) {
	chunks := SplitMessage("")
	if len(chunks) != 1 || chunks[0] != "" {
		t.Errorf("got %v", chunks)
	}
}

func TestSplitMessage_PrefersParagraphOverNewline(t *testing.T) {
	// Both \n\n and \n exist — should split at \n\n
	part1 := strings.Repeat("a", 2000) + "\n" + strings.Repeat("b", 1000)
	text := part1 + "\n\n" + strings.Repeat("c", 3000)

	chunks := SplitMessage(text)
	if len(chunks) < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", len(chunks))
	}
	// First chunk should end at the \n\n boundary
	if chunks[0] != part1 {
		t.Errorf("first chunk should be everything before \\n\\n")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/telegram/ -v
```
Expected: FAIL — `SplitMessage` not found.

- [ ] **Step 3: Implement message.go**

`internal/telegram/message.go`:
```go
package telegram

import "strings"

const MaxTelegramLength = 4096

// SplitMessage splits text into chunks that fit within Telegram's message limit.
// Prefers splitting at paragraph boundaries (\n\n), then newlines (\n), then hard-cuts.
func SplitMessage(text string) []string {
	if len(text) <= MaxTelegramLength {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= MaxTelegramLength {
			chunks = append(chunks, remaining)
			break
		}

		chunk := remaining[:MaxTelegramLength]

		if cut := strings.LastIndex(chunk, "\n\n"); cut > 0 {
			chunks = append(chunks, remaining[:cut])
			remaining = remaining[cut+2:]
			continue
		}

		if cut := strings.LastIndex(chunk, "\n"); cut > 0 {
			chunks = append(chunks, remaining[:cut])
			remaining = remaining[cut+1:]
			continue
		}

		chunks = append(chunks, chunk)
		remaining = remaining[MaxTelegramLength:]
	}

	return chunks
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/telegram/ -v
```
Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telegram/message.go internal/telegram/message_test.go
git commit -m "feat: add Telegram message splitting"
```

---

### Task 7: Telegram Bot Handlers

**Files:**
- Create: `internal/telegram/bot.go`

- [ ] **Step 1: Implement bot.go**

`internal/telegram/bot.go`:
```go
package telegram

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/derrix060/ai-fitness-trainer/internal/claude"
	"github.com/derrix060/ai-fitness-trainer/internal/config"
	"github.com/derrix060/ai-fitness-trainer/internal/store"
)

type Bot struct {
	tg     *tgbot.Bot
	cfg    *config.Config
	store  *store.Store
	claude *claude.Client
}

func NewBot(cfg *config.Config, s *store.Store, c *claude.Client) (*Bot, error) {
	tb := &Bot{cfg: cfg, store: s, claude: c}

	b, err := tgbot.New(cfg.TelegramBotToken,
		tgbot.WithDefaultHandler(tb.defaultHandler),
	)
	if err != nil {
		return nil, err
	}
	tb.tg = b

	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/start", tgbot.MatchTypePrefix, tb.handleStart)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "/new", tgbot.MatchTypePrefix, tb.handleNew)
	b.RegisterHandler(tgbot.HandlerTypeMessageText, "", tgbot.MatchTypePrefix, tb.handleText)

	return tb, nil
}

func (tb *Bot) Start(ctx context.Context) {
	log.Printf("Starting Telegram bot")
	tb.tg.Start(ctx)
}

func (tb *Bot) TgBot() *tgbot.Bot {
	return tb.tg
}

// SendFormatted splits and sends a message with HTML formatting, falling back to plain text.
func (tb *Bot) SendFormatted(ctx context.Context, chatID int64, text string) {
	for _, chunk := range SplitMessage(text) {
		_, err := tb.tg.SendMessage(ctx, &tgbot.SendMessageParams{
			ChatID:    chatID,
			Text:      chunk,
			ParseMode: models.ParseModeHTML,
		})
		if err != nil {
			log.Printf("HTML send failed, falling back to plain: %v", err)
			tb.tg.SendMessage(ctx, &tgbot.SendMessageParams{
				ChatID: chatID,
				Text:   chunk,
			})
		}
	}
}

func (tb *Bot) handleStart(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if !tb.isAllowed(update) {
		return
	}
	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text: "Hey! I'm your AI fitness coach. Ask me anything about your " +
			"training, schedule, nutrition, or recovery. Use /new to start a " +
			"fresh conversation.",
	})
}

func (tb *Bot) handleNew(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if !tb.isAllowed(update) {
		return
	}
	userID := update.Message.From.ID
	if err := tb.store.DeleteSession(userID); err != nil {
		log.Printf("Failed to delete session for user %d: %v", userID, err)
	}
	b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   "Fresh start! What would you like to work on?",
	})
	log.Printf("Session reset for user %d", userID)
}

func (tb *Bot) handleText(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	if !tb.isAllowed(update) {
		return
	}

	userID := update.Message.From.ID

	// React with eyes to acknowledge
	b.SetMessageReaction(ctx, &tgbot.SetMessageReactionParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
		Reaction: []models.ReactionType{
			{Type: models.ReactionTypeTypeEmoji, ReactionTypeEmoji: &models.ReactionTypeEmoji{Emoji: "👀"}},
		},
	})

	sessionID, _ := tb.store.GetSession(userID)

	responseText, newSessionID, err := tb.claude.SendMessage(ctx, update.Message.Text, sessionID)
	if err != nil {
		log.Printf("Claude error for user %d: %v", userID, err)
		responseText = "Sorry, something went wrong. Please try again."
	}

	// Remove eyes reaction
	b.SetMessageReaction(ctx, &tgbot.SetMessageReactionParams{
		ChatID:    update.Message.Chat.ID,
		MessageID: update.Message.ID,
		Reaction:  []models.ReactionType{},
	})

	if newSessionID != "" {
		if err := tb.store.SaveSession(userID, newSessionID); err != nil {
			log.Printf("Failed to save session for user %d: %v", userID, err)
		}
	}

	tb.SendFormatted(ctx, update.Message.Chat.ID, responseText)
}

func (tb *Bot) isAllowed(update *models.Update) bool {
	if update.Message == nil || update.Message.From == nil {
		return false
	}
	if !tb.cfg.IsAllowed(update.Message.From.ID) {
		log.Printf("Ignored message from unauthorized user %d", update.Message.From.ID)
		return false
	}
	return true
}

func (tb *Bot) defaultHandler(ctx context.Context, b *tgbot.Bot, update *models.Update) {
	// Ignore non-text messages
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./internal/telegram/
```
Expected: Success.

- [ ] **Step 3: Commit**

```bash
git add internal/telegram/bot.go
git commit -m "feat: add Telegram bot handlers with reactions and HTML formatting"
```

---

## Chunk 4: Scheduler

### Task 8: Prompts and Scheduler

**Files:**
- Create: `internal/scheduler/prompts.go`
- Create: `internal/scheduler/scheduler.go`

- [ ] **Step 1: Create prompt templates**

`internal/scheduler/prompts.go`:
```go
package scheduler

const MorningBriefingPrompt = `Good morning! Please give me my daily training briefing:
1. Check my training plan for today on Intervals.icu — what workout is scheduled?
2. Check my recent wellness data (sleep, soreness, HRV, mood).
3. Look at my recent training load (ATL/CTL/TSB) and fatigue.
4. Based on all this, tell me what I should do today.
5. Ask me how I'm feeling and if anything needs adjusting.
Keep it concise — this is my morning check-in.`

const ActivityAnalysisPrompt = `Check Intervals.icu for any activities from the last 24 hours (use oldest=%s).

%s

For EACH new activity, start your response with ANALYZED:<activity_id> on its own line (e.g. ANALYZED:i129194330), then provide a detailed analysis:

1. **Activity summary**: type, duration, distance, average HR, average power/pace
2. **Training classification**: what kind of training was this? (recovery, endurance/zone 2, tempo/sweetspot, threshold, VO2max, anaerobic, sprint, strength, etc.). Explain WHY you classified it this way based on the intensity distribution, HR zones, and power/pace data.
3. **Performance assessment**: how well did it go? Compare to recent similar activities. Look at pacing consistency, cardiac drift, power/pace decoupling, RPE vs actual intensity.
4. **Scientific context**: use WebSearch to find relevant exercise science research (peer-reviewed papers, systematic reviews) that supports your analysis.
5. **Key takeaways**: 2-3 actionable insights for future training.

For every scientific claim, include a reference with author, year, journal, and the finding. Format references as a numbered list at the end.

If there are NO new activities to analyze, respond with exactly: NO_NEW_ACTIVITIES`
```

- [ ] **Step 2: Implement scheduler.go**

`internal/scheduler/scheduler.go`:
```go
package scheduler

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"

	"github.com/derrix060/ai-fitness-trainer/internal/claude"
	"github.com/derrix060/ai-fitness-trainer/internal/config"
	"github.com/derrix060/ai-fitness-trainer/internal/store"
)

var (
	analyzedRe     = regexp.MustCompile(`ANALYZED:(i\d+)`)
	analyzedLineRe = regexp.MustCompile(`(?m)^ANALYZED:i\d+\n?`)
)

// Sender can send formatted messages to a Telegram chat.
type Sender interface {
	SendFormatted(ctx context.Context, chatID int64, text string)
}

func Setup(
	cfg *config.Config,
	sender Sender,
	c *claude.Client,
	s *store.Store,
) (gocron.Scheduler, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", cfg.Timezone, err)
	}

	sched, err := gocron.NewScheduler(gocron.WithLocation(loc))
	if err != nil {
		return nil, err
	}

	_, err = sched.NewJob(
		gocron.CronJob(
			fmt.Sprintf("%d %d * * *", cfg.BriefingMinute, cfg.BriefingHour),
			false,
		),
		gocron.NewTask(func() {
			morningBriefing(cfg, sender, c, s)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("schedule morning briefing: %w", err)
	}

	_, err = sched.NewJob(
		gocron.DurationJob(30*time.Minute),
		gocron.NewTask(func() {
			activityCheck(cfg, sender, c, s)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("schedule activity check: %w", err)
	}

	log.Printf("Morning briefing scheduled at %02d:%02d %s",
		cfg.BriefingHour, cfg.BriefingMinute, cfg.Timezone)
	log.Printf("Activity check scheduled every 30 minutes")

	return sched, nil
}

func morningBriefing(cfg *config.Config, sender Sender, c *claude.Client, s *store.Store) {
	ctx := context.Background()
	for userID := range cfg.AllowedUserIDs {
		log.Printf("Sending morning briefing to user %d", userID)

		sessionID, _ := s.GetSession(userID)
		responseText, newSessionID, err := c.SendMessage(ctx, MorningBriefingPrompt, sessionID)
		if err != nil {
			log.Printf("Morning briefing error for user %d: %v", userID, err)
			continue
		}

		if newSessionID != "" {
			s.SaveSession(userID, newSessionID)
		}
		sender.SendFormatted(ctx, userID, responseText)
		log.Printf("Morning briefing sent to user %d", userID)
	}
}

func activityCheck(cfg *config.Config, sender Sender, c *claude.Client, s *store.Store) {
	ctx := context.Background()
	sinceDate := time.Now().Add(-24 * time.Hour).Format("2006-01-02T15:04")
	analyzed, _ := s.GetAnalyzedActivities()
	log.Printf("Checking for new activities since %s (skip %d already analyzed)",
		sinceDate, len(analyzed))

	var skipClause string
	if len(analyzed) > 0 {
		ids := make([]string, 0, len(analyzed))
		for id := range analyzed {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		skipClause = "\nSkip these activity IDs (already analyzed): " + strings.Join(ids, ", ")
	}

	for userID := range cfg.AllowedUserIDs {
		sessionID, _ := s.GetSession(userID)
		prompt := fmt.Sprintf(ActivityAnalysisPrompt, sinceDate, skipClause)

		responseText, newSessionID, err := c.SendMessage(ctx, prompt, sessionID)
		if err != nil {
			log.Printf("Activity check error for user %d: %v", userID, err)
			continue
		}

		if newSessionID != "" {
			s.SaveSession(userID, newSessionID)
		}

		if strings.Contains(responseText, "NO_NEW_ACTIVITIES") {
			log.Printf("No new activities for user %d", userID)
			continue
		}

		for _, match := range analyzedRe.FindAllStringSubmatch(responseText, -1) {
			s.MarkActivityAnalyzed(match[1])
			log.Printf("Marked activity %s as analyzed", match[1])
		}

		clean := strings.TrimSpace(analyzedLineRe.ReplaceAllString(responseText, ""))
		if clean != "" {
			log.Printf("Sending activity analysis to user %d", userID)
			sender.SendFormatted(ctx, userID, clean)
		}
	}
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/scheduler/
```
Expected: Success.

- [ ] **Step 4: Commit**

```bash
git add internal/scheduler/
git commit -m "feat: add scheduler with morning briefing and activity check"
```

---

## Chunk 5: Google Calendar MCP Server

### Task 9: Google Calendar MCP Server

**Files:**
- Create: `cmd/gcal-mcp/main.go`

This replaces the `@cocal/google-calendar-mcp` npm package. It reads the same credential/token file formats for backward compatibility, so existing tokens work without re-authenticating.

- [ ] **Step 1: Implement the MCP server**

`cmd/gcal-mcp/main.go`:
```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// tokenEntry matches the format written by @cocal/google-calendar-mcp.
type tokenEntry struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	ExpiryDate   int64  `json:"expiry_date"` // milliseconds since epoch
}

type account struct {
	name    string
	service *calendar.Service
}

func main() {
	log.SetOutput(os.Stderr)

	// Handle "auth" subcommand for initial OAuth flow
	if len(os.Args) > 1 && os.Args[1] == "auth" {
		name := ""
		if len(os.Args) > 2 {
			name = os.Args[2]
		}
		if err := runAuth(name); err != nil {
			log.Fatalf("Auth failed: %v", err)
		}
		return
	}

	credPath := os.Getenv("GOOGLE_OAUTH_CREDENTIALS")
	tokenPath := os.Getenv("GOOGLE_CALENDAR_MCP_TOKEN_PATH")
	if credPath == "" || tokenPath == "" {
		log.Fatal("GOOGLE_OAUTH_CREDENTIALS and GOOGLE_CALENDAR_MCP_TOKEN_PATH are required")
	}

	accounts, err := loadAccounts(credPath, tokenPath)
	if err != nil {
		log.Fatalf("Failed to load accounts: %v", err)
	}

	s := server.NewMCPServer("google-calendar", "1.0.0")
	s.AddTool(listCalendarsTool(), listCalendarsHandler(accounts))
	s.AddTool(listEventsTool(), listEventsHandler(accounts))
	s.AddTool(createEventTool(), createEventHandler(accounts))
	s.AddTool(updateEventTool(), updateEventHandler(accounts))
	s.AddTool(deleteEventTool(), deleteEventHandler(accounts))

	log.Printf("Starting google-calendar MCP server with %d accounts", len(accounts))
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func loadAccounts(credPath, tokenPath string) ([]account, error) {
	credJSON, err := os.ReadFile(credPath)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	oauthCfg, err := google.ConfigFromJSON(credJSON, calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}

	tokenJSON, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("read tokens: %w", err)
	}

	var tokens map[string]tokenEntry
	if err := json.Unmarshal(tokenJSON, &tokens); err != nil {
		// Try single-account (bare token object)
		var single tokenEntry
		if err2 := json.Unmarshal(tokenJSON, &single); err2 != nil {
			return nil, fmt.Errorf("parse tokens: %w", err)
		}
		tokens = map[string]tokenEntry{"default": single}
	}

	var accts []account
	for name, te := range tokens {
		tok := &oauth2.Token{
			AccessToken:  te.AccessToken,
			RefreshToken: te.RefreshToken,
			TokenType:    te.TokenType,
			Expiry:       time.UnixMilli(te.ExpiryDate),
		}
		client := oauthCfg.Client(context.Background(), tok)
		svc, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
		if err != nil {
			return nil, fmt.Errorf("create service for %s: %w", name, err)
		}
		accts = append(accts, account{name: name, service: svc})
	}
	sort.Slice(accts, func(i, j int) bool { return accts[i].name < accts[j].name })
	return accts, nil
}

func findAccount(accounts []account, name string) *account {
	for i := range accounts {
		if accounts[i].name == name {
			return &accounts[i]
		}
	}
	return nil
}

func filterAccounts(accounts []account, name string) []account {
	if name == "" {
		return accounts
	}
	if a := findAccount(accounts, name); a != nil {
		return []account{*a}
	}
	return nil
}

// --- list-calendars ---

func listCalendarsTool() mcp.Tool {
	return mcp.NewTool("list-calendars",
		mcp.WithDescription("List all calendars from all connected Google accounts"),
		mcp.WithString("account", mcp.Description("Account nickname (omit for all)")),
	)
}

func listCalendarsHandler(accounts []account) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		acctName := req.GetString("account", "")
		accts := filterAccounts(accounts, acctName)

		type calInfo struct {
			Account  string `json:"account"`
			ID       string `json:"id"`
			Summary  string `json:"summary"`
			Primary  bool   `json:"primary"`
			TimeZone string `json:"timeZone"`
		}

		var calendars []calInfo
		for _, a := range accts {
			list, err := a.service.CalendarList.List().Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("list calendars for %s: %w", a.name, err)
			}
			for _, c := range list.Items {
				calendars = append(calendars, calInfo{
					Account: a.name, ID: c.Id, Summary: c.Summary,
					Primary: c.Primary, TimeZone: c.TimeZone,
				})
			}
		}

		data, _ := json.MarshalIndent(calendars, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- list-events ---

func listEventsTool() mcp.Tool {
	return mcp.NewTool("list-events",
		mcp.WithDescription("List calendar events within a time range"),
		mcp.WithString("timeMin", mcp.Required(), mcp.Description("Start time (RFC3339)")),
		mcp.WithString("timeMax", mcp.Required(), mcp.Description("End time (RFC3339)")),
		mcp.WithString("calendarId", mcp.Description("Calendar ID (default: primary)")),
		mcp.WithString("account", mcp.Description("Account nickname (omit for all)")),
	)
}

func listEventsHandler(accounts []account) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		timeMin := req.GetString("timeMin", "")
		timeMax := req.GetString("timeMax", "")
		calID := req.GetString("calendarId", "primary")
		acctName := req.GetString("account", "")
		accts := filterAccounts(accounts, acctName)

		type eventInfo struct {
			Account     string `json:"account"`
			ID          string `json:"id"`
			Summary     string `json:"summary"`
			Start       string `json:"start"`
			End         string `json:"end"`
			Description string `json:"description,omitempty"`
			Location    string `json:"location,omitempty"`
			Status      string `json:"status"`
		}

		var events []eventInfo
		for _, a := range accts {
			list, err := a.service.Events.List(calID).
				TimeMin(timeMin).TimeMax(timeMax).
				SingleEvents(true).OrderBy("startTime").
				Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("list events for %s: %w", a.name, err)
			}
			for _, e := range list.Items {
				start := e.Start.DateTime
				if start == "" {
					start = e.Start.Date
				}
				end := e.End.DateTime
				if end == "" {
					end = e.End.Date
				}
				events = append(events, eventInfo{
					Account: a.name, ID: e.Id, Summary: e.Summary,
					Start: start, End: end, Description: e.Description,
					Location: e.Location, Status: e.Status,
				})
			}
		}

		data, _ := json.MarshalIndent(events, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- create-event ---

func createEventTool() mcp.Tool {
	return mcp.NewTool("create-event",
		mcp.WithDescription("Create a new calendar event"),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Event title")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Start time (RFC3339)")),
		mcp.WithString("end", mcp.Required(), mcp.Description("End time (RFC3339)")),
		mcp.WithString("description", mcp.Description("Event description")),
		mcp.WithString("location", mcp.Description("Event location")),
		mcp.WithString("calendarId", mcp.Description("Calendar ID (default: primary)")),
		mcp.WithString("account", mcp.Required(), mcp.Description("Account nickname")),
	)
}

func createEventHandler(accounts []account) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		acctName := req.GetString("account", "")
		summary := req.GetString("summary", "")
		start := req.GetString("start", "")
		end := req.GetString("end", "")
		description := req.GetString("description", "")
		location := req.GetString("location", "")
		calID := req.GetString("calendarId", "primary")

		acct := findAccount(accounts, acctName)
		if acct == nil {
			return nil, fmt.Errorf("account %q not found", acctName)
		}

		event := &calendar.Event{
			Summary: summary, Description: description, Location: location,
			Start: &calendar.EventDateTime{DateTime: start},
			End:   &calendar.EventDateTime{DateTime: end},
		}

		created, err := acct.service.Events.Insert(calID, event).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("create event: %w", err)
		}

		data, _ := json.MarshalIndent(map[string]string{
			"id": created.Id, "summary": created.Summary, "link": created.HtmlLink,
		}, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- update-event ---

func updateEventTool() mcp.Tool {
	return mcp.NewTool("update-event",
		mcp.WithDescription("Update an existing calendar event"),
		mcp.WithString("eventId", mcp.Required(), mcp.Description("Event ID")),
		mcp.WithString("summary", mcp.Description("New title")),
		mcp.WithString("start", mcp.Description("New start time (RFC3339)")),
		mcp.WithString("end", mcp.Description("New end time (RFC3339)")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("location", mcp.Description("New location")),
		mcp.WithString("calendarId", mcp.Description("Calendar ID (default: primary)")),
		mcp.WithString("account", mcp.Required(), mcp.Description("Account nickname")),
	)
}

func updateEventHandler(accounts []account) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		acctName := req.GetString("account", "")
		eventID := req.GetString("eventId", "")
		calID := req.GetString("calendarId", "primary")

		acct := findAccount(accounts, acctName)
		if acct == nil {
			return nil, fmt.Errorf("account %q not found", acctName)
		}

		existing, err := acct.service.Events.Get(calID, eventID).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("get event: %w", err)
		}

		// For optional update fields, use GetString with empty default —
		// only apply if the caller provided a value
		if s := req.GetString("summary", ""); s != "" {
			existing.Summary = s
		}
		if s := req.GetString("description", ""); s != "" {
			existing.Description = s
		}
		if s := req.GetString("location", ""); s != "" {
			existing.Location = s
		}
		if s := req.GetString("start", ""); s != "" {
			existing.Start = &calendar.EventDateTime{DateTime: s}
		}
		if s := req.GetString("end", ""); s != "" {
			existing.End = &calendar.EventDateTime{DateTime: s}
		}

		updated, err := acct.service.Events.Update(calID, eventID, existing).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("update event: %w", err)
		}

		data, _ := json.MarshalIndent(map[string]string{
			"id": updated.Id, "summary": updated.Summary, "link": updated.HtmlLink,
		}, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- delete-event ---

func deleteEventTool() mcp.Tool {
	return mcp.NewTool("delete-event",
		mcp.WithDescription("Delete a calendar event"),
		mcp.WithString("eventId", mcp.Required(), mcp.Description("Event ID")),
		mcp.WithString("calendarId", mcp.Description("Calendar ID (default: primary)")),
		mcp.WithString("account", mcp.Required(), mcp.Description("Account nickname")),
	)
}

func deleteEventHandler(accounts []account) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		acctName := req.GetString("account", "")
		eventID := req.GetString("eventId", "")
		calID := req.GetString("calendarId", "primary")

		acct := findAccount(accounts, acctName)
		if acct == nil {
			return nil, fmt.Errorf("account %q not found", acctName)
		}

		if err := acct.service.Events.Delete(calID, eventID).Context(ctx).Do(); err != nil {
			return nil, fmt.Errorf("delete event: %w", err)
		}
		return mcp.NewToolResultText("Event deleted successfully"), nil
	}
}

// --- auth subcommand ---

func runAuth(accountName string) error {
	credPath := os.Getenv("GOOGLE_OAUTH_CREDENTIALS")
	tokenPath := os.Getenv("GOOGLE_CALENDAR_MCP_TOKEN_PATH")
	if credPath == "" || tokenPath == "" {
		return fmt.Errorf("GOOGLE_OAUTH_CREDENTIALS and GOOGLE_CALENDAR_MCP_TOKEN_PATH are required")
	}

	credJSON, err := os.ReadFile(credPath)
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(credJSON, calendar.CalendarScope)
	if err != nil {
		return fmt.Errorf("parse credentials: %w", err)
	}
	oauthCfg.RedirectURL = "http://localhost:3000/callback"

	authURL := oauthCfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Open this URL in your browser:\n\n%s\n\nWaiting for authorization...\n", authURL)

	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code received", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "Authorization successful! You can close this window.")
		codeCh <- code
	})
	srv := &http.Server{Addr: ":3000", Handler: mux}
	go srv.ListenAndServe()

	code := <-codeCh
	srv.Shutdown(context.Background())

	tok, err := oauthCfg.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("exchange token: %w", err)
	}

	// Merge into existing token file
	tokens := make(map[string]tokenEntry)
	if data, err := os.ReadFile(tokenPath); err == nil {
		json.Unmarshal(data, &tokens)
	}

	name := accountName
	if name == "" {
		name = "default"
	}
	tokens[name] = tokenEntry{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Scope:        calendar.CalendarScope,
		TokenType:    tok.TokenType,
		ExpiryDate:   tok.Expiry.UnixMilli(),
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tokens: %w", err)
	}
	if err := os.WriteFile(tokenPath, data, 0o600); err != nil {
		return fmt.Errorf("write tokens: %w", err)
	}

	fmt.Printf("Token saved for account %q\n", name)
	return nil
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/gcal-mcp/
```
Expected: Success.

- [ ] **Step 3: Test auth flow manually** (requires browser)

```bash
GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens-test.json \
  go run ./cmd/gcal-mcp/ auth test-account
```
Expected: Browser opens, authorize, token saved.

- [ ] **Step 4: Test MCP tools manually**

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list-calendars","arguments":{}}}' | \
  GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
  GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens.json \
  go run ./cmd/gcal-mcp/ 2>/dev/null | python3 -m json.tool
```
Expected: JSON list of calendars.

- [ ] **Step 5: Commit**

```bash
git add cmd/gcal-mcp/
git commit -m "feat: add Google Calendar MCP server in Go with auth CLI"
```

---

## Chunk 6: Integration and Deployment

### Task 10: Main Entrypoint

**Files:**
- Modify: `cmd/bot/main.go`

- [ ] **Step 1: Implement main.go**

`cmd/bot/main.go`:
```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/derrix060/ai-fitness-trainer/internal/claude"
	"github.com/derrix060/ai-fitness-trainer/internal/config"
	"github.com/derrix060/ai-fitness-trainer/internal/scheduler"
	"github.com/derrix060/ai-fitness-trainer/internal/store"
	"github.com/derrix060/ai-fitness-trainer/internal/telegram"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("Create data dir: %v", err)
	}

	s, err := store.New(filepath.Join(cfg.DataDir, "sessions.db"))
	if err != nil {
		log.Fatalf("Store error: %v", err)
	}
	defer s.Close()

	c := claude.NewClient(cfg.ClaudeModel, cfg.ClaudeTimeout)

	bot, err := telegram.NewBot(cfg, s, c)
	if err != nil {
		log.Fatalf("Bot error: %v", err)
	}

	sched, err := scheduler.Setup(cfg, bot, c, s)
	if err != nil {
		log.Fatalf("Scheduler error: %v", err)
	}
	sched.Start()
	defer func() {
		if err := sched.Shutdown(); err != nil {
			log.Printf("Scheduler shutdown error: %v", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	bot.Start(ctx)
}
```

- [ ] **Step 2: Verify full project compiles**

```bash
go build ./...
```
Expected: Success.

- [ ] **Step 3: Commit**

```bash
git add cmd/bot/main.go
git commit -m "feat: add main entrypoint wiring config, store, bot, and scheduler"
```

---

### Task 11: Dockerfile and Docker Compose

**Files:**
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`
- Modify: `.mcp.json`

- [ ] **Step 1: Rewrite Dockerfile**

`Dockerfile`:
```dockerfile
# Stage 1: Build all Go binaries
FROM golang:1.25-bookworm AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bot ./cmd/bot/
RUN CGO_ENABLED=0 GOOS=linux go build -o gcal-mcp ./cmd/gcal-mcp/

# Also build intervals-mcp
RUN go install github.com/derrix060/intervals-mcp@latest

# Stage 2: Minimal runtime
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Claude CLI (standalone binary, no Node.js needed)
RUN curl -fsSL https://claude.ai/install.sh | bash && \
    cp /root/.local/share/claude/versions/* /usr/local/bin/claude

# Copy Go binaries
COPY --from=builder /build/bot /app/bot
COPY --from=builder /build/gcal-mcp /app/gcal-mcp
COPY --from=builder /go/bin/intervals-mcp /app/intervals-mcp

COPY CLAUDE.md .mcp.json /app/

WORKDIR /app

RUN useradd --create-home --shell /bin/bash appuser && \
    mkdir -p /app/data /app/config && \
    chown -R appuser:appuser /app

USER appuser

CMD ["/app/bot"]
```

- [ ] **Step 2: Update .mcp.json**

`.mcp.json`:
```json
{
  "mcpServers": {
    "intervals-icu": {
      "command": "/app/intervals-mcp",
      "env": {
        "INTERVALS_API_KEY": "${INTERVALS_API_KEY}",
        "INTERVALS_ATHLETE_ID": "${INTERVALS_ATHLETE_ID}"
      }
    },
    "google-calendar": {
      "command": "/app/gcal-mcp",
      "env": {
        "GOOGLE_OAUTH_CREDENTIALS": "/app/config/gcp-oauth.keys.json",
        "GOOGLE_CALENDAR_MCP_TOKEN_PATH": "/app/config/gcal-tokens.json"
      }
    }
  }
}
```

- [ ] **Step 3: docker-compose.yml stays the same** (already correct — only the CMD changed inside Dockerfile)

Verify no changes needed:
```yaml
services:
  bot:
    build: .
    env_file: .env
    volumes:
      - ./data:/app/data
      - ./data/claude-home:/home/appuser/.claude
      - ./config:/app/config
      - ./CLAUDE.md:/app/CLAUDE.md
      - ./profile/athlete_profile.md:/app/profile/athlete_profile.md
      - ./profile/learned_preferences.md:/app/profile/learned_preferences.md
    network_mode: host
    restart: unless-stopped
```

- [ ] **Step 4: Build and verify**

```bash
docker compose build
```
Expected: Success — no Python, no Node.js in the image.

- [ ] **Step 5: Run and verify bot starts**

```bash
docker compose up -d && docker compose logs -f --tail=20
```
Expected: See "Starting Telegram bot" and scheduler messages in logs.

- [ ] **Step 6: Send a test message via Telegram**

Send any message to the bot. Verify it responds.

- [ ] **Step 7: Commit**

```bash
git add Dockerfile .mcp.json
git commit -m "feat: Go-only Dockerfile, no Python or Node.js"
```

---

### Task 12: Cleanup

**Files:**
- Delete: `src/` directory (all Python source)
- Delete: `pyproject.toml`
- Modify: `docs/google-calendar-setup.md` (replace npx with Go binary)

- [ ] **Step 1: Remove Python source files**

```bash
rm -rf src/ pyproject.toml
```

- [ ] **Step 2: Update Google Calendar setup docs**

In `docs/google-calendar-setup.md`, replace all `npx -y @cocal/google-calendar-mcp` references with the Go binary equivalent:

Auth command:
```bash
GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens.json \
  go run ./cmd/gcal-mcp/ auth
```

Multi-account:
```bash
GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens.json \
  go run ./cmd/gcal-mcp/ auth personal
```

Verification:
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list-calendars","arguments":{}}}' | \
  GOOGLE_OAUTH_CREDENTIALS=./config/gcp-oauth.keys.json \
  GOOGLE_CALENDAR_MCP_TOKEN_PATH=./config/gcal-tokens.json \
  go run ./cmd/gcal-mcp/ 2>/dev/null | python3 -m json.tool
```

Remove the Node.js prerequisite. Remove the Docker notes section about npx.

- [ ] **Step 3: Update README.md** to reflect Go build instead of Python

- [ ] **Step 4: Remove .cursorrules symlink** (it pointed to AGENT.md which referenced Python)

- [ ] **Step 5: Final build + test**

```bash
docker compose build && docker compose up -d
docker compose logs -f --tail=30
```
Expected: Bot starts, morning briefing scheduled, activity check scheduled.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove Python/Node.js, complete Go rewrite"
```

---

## Notes

### ARM64 Cross-Compilation

To build for Raspberry Pi Zero 2W from macOS:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o bot-arm64 ./cmd/bot/
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o gcal-mcp-arm64 ./cmd/gcal-mcp/
```

Or in Dockerfile with `--platform linux/arm64`:
```bash
docker buildx build --platform linux/arm64 -t ai-fitness-trainer:arm64 .
```

### SQLite Compatibility

The Go `modernc.org/sqlite` driver uses the same on-disk format as standard SQLite. The existing `data/sessions.db` from the Python version will work without migration.

### Backward Compatibility

- Token file format (`gcal-tokens.json`) is identical — existing tokens work without re-auth
- SQLite schema is identical — no data migration needed
- `.env` file format is identical — no config changes needed
- CLAUDE.md is unchanged — coach personality and behavior preserved
