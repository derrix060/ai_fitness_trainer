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
