package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	maxFeedBytes = 5 << 20
	dateLayout   = "2006-01-02"
)

type workout struct {
	Start       string `json:"start"`
	End         string `json:"end,omitempty"`
	AllDay      bool   `json:"all_day"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

func main() {
	log.SetOutput(os.Stderr)

	feedURL := os.Getenv("TRAININGPEAKS_ICAL_URL")
	if feedURL == "" {
		log.Fatal("TRAININGPEAKS_ICAL_URL is required")
	}
	if u, err := url.Parse(feedURL); err != nil || u.Scheme != "https" || u.Host == "" {
		log.Fatal("TRAININGPEAKS_ICAL_URL must be an https URL")
	}

	s := server.NewMCPServer("trainingpeaks", "1.0.0")
	s.AddTool(mcp.NewTool("list_workouts",
		mcp.WithDescription("List workouts from the athlete's TrainingPeaks calendar (read-only iCal feed): planned and completed sessions, with planned vs actual time and distance in the description. The feed only covers a window around today."),
		mcp.WithString("from", mcp.Description("First day to include, YYYY-MM-DD. Defaults to today.")),
		mcp.WithString("to", mcp.Description("Last day to include, YYYY-MM-DD. Defaults to 14 days after from.")),
	), listWorkouts(feedURL))

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func listWorkouts(feedURL string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		loc := time.Local
		from, to, err := dayRange(req.GetString("from", ""), req.GetString("to", ""), time.Now().In(loc))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, err := fetchFeed(ctx, feedURL)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		out := []workout{}
		for _, w := range parseWorkouts(body, loc) {
			if d := w.Start[:10]; d >= from && d <= to {
				out = append(out, w)
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	}
}

func dayRange(fromArg, toArg string, now time.Time) (string, string, error) {
	from := now
	if fromArg != "" {
		t, err := time.Parse(dateLayout, fromArg)
		if err != nil {
			return "", "", fmt.Errorf("from must be YYYY-MM-DD")
		}
		from = t
	}
	to := from.AddDate(0, 0, 14)
	if toArg != "" {
		t, err := time.Parse(dateLayout, toArg)
		if err != nil {
			return "", "", fmt.Errorf("to must be YYYY-MM-DD")
		}
		to = t
	}
	return from.Format(dateLayout), to.Format(dateLayout), nil
}

// fetchFeed never returns the URL in an error: the feed token is the credential.
func fetchFeed(ctx context.Context, feedURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, errors.New("invalid feed URL")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("fetch TrainingPeaks feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TrainingPeaks feed returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, errors.New("read TrainingPeaks feed")
	}
	if len(body) > maxFeedBytes {
		return nil, errors.New("TrainingPeaks feed too large")
	}
	return body, nil
}

// parseWorkouts reads VEVENTs from an iCal body. Floating times (no Z) are
// interpreted in loc; events without a parseable DTSTART are dropped.
func parseWorkouts(body []byte, loc *time.Location) []workout {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n ", "")
	text = strings.ReplaceAll(text, "\n\t", "")

	var out []workout
	var cur *workout
	for _, line := range strings.Split(text, "\n") {
		switch line {
		case "BEGIN:VEVENT":
			cur = &workout{}
			continue
		case "END:VEVENT":
			if cur != nil && cur.Start != "" {
				out = append(out, *cur)
			}
			cur = nil
			continue
		}
		if cur == nil {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name, params, _ := strings.Cut(key, ";")
		switch name {
		case "SUMMARY":
			cur.Title = unescape(val)
		case "DESCRIPTION":
			cur.Description = strings.TrimSpace(unescape(val))
		case "DTSTART":
			cur.Start, cur.AllDay = parseTime(val, params, loc)
		case "DTEND":
			cur.End, _ = parseTime(val, params, loc)
		}
	}
	return out
}

func parseTime(val, params string, loc *time.Location) (string, bool) {
	if strings.Contains(params, "VALUE=DATE") && !strings.Contains(params, "DATE-TIME") {
		t, err := time.Parse("20060102", val)
		if err != nil {
			return "", false
		}
		return t.Format(dateLayout), true
	}
	if strings.HasSuffix(val, "Z") {
		t, err := time.Parse("20060102T150405Z", val)
		if err != nil {
			return "", false
		}
		return t.In(loc).Format("2006-01-02T15:04:05"), false
	}
	t, err := time.ParseInLocation("20060102T150405", val, loc)
	if err != nil {
		return "", false
	}
	return t.Format("2006-01-02T15:04:05"), false
}

func unescape(s string) string {
	return strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`).Replace(s)
}
