package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	TelegramBotToken   string
	AllowedUserIDs     map[int64]struct{}
	ClaudeModel        string
	ClaudeTimeout      time.Duration
	IntervalsAPIKey    string
	IntervalsAthleteID string
	DataDir            string
	BriefingHour       int
	BriefingMinute     int
	Timezone           *time.Location
	LogLevel           string
}

// Load reads configuration from environment variables (with optional .env file)
// and returns a validated Config.
func Load() (*Config, error) {
	// Load .env file if present; ignore error if missing.
	_ = godotenv.Load()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	tzName := os.Getenv("TZ")
	if tzName == "" {
		return nil, fmt.Errorf("TZ is required")
	}
	tz, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("invalid TZ %q: %w", tzName, err)
	}

	allowed, err := parseAllowedUserIDs(os.Getenv("ALLOWED_TELEGRAM_USER_IDS"))
	if err != nil {
		return nil, fmt.Errorf("invalid ALLOWED_TELEGRAM_USER_IDS: %w", err)
	}

	timeoutSec, err := envInt("CLAUDE_TIMEOUT", 1200)
	if err != nil {
		return nil, err
	}

	briefingHour, err := envInt("BRIEFING_HOUR", 7)
	if err != nil {
		return nil, err
	}

	briefingMinute, err := envInt("BRIEFING_MINUTE", 0)
	if err != nil {
		return nil, err
	}

	return &Config{
		TelegramBotToken:   token,
		AllowedUserIDs:     allowed,
		ClaudeModel:        envOr("CLAUDE_MODEL", "sonnet"),
		ClaudeTimeout:      time.Duration(timeoutSec) * time.Second,
		IntervalsAPIKey:    os.Getenv("INTERVALS_API_KEY"),
		IntervalsAthleteID: os.Getenv("INTERVALS_ATHLETE_ID"),
		DataDir:            envOr("DATA_DIR", "data"),
		BriefingHour:       briefingHour,
		BriefingMinute:     briefingMinute,
		Timezone:           tz,
		LogLevel:           envOr("LOG_LEVEL", "INFO"),
	}, nil
}

// IsAllowed reports whether the given Telegram user ID is in the allow-list.
func (c *Config) IsAllowed(userID int64) bool {
	_, ok := c.AllowedUserIDs[userID]
	return ok
}

// envOr returns the value of the environment variable named by key,
// or fallback if the variable is empty or unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt returns the value of the environment variable named by key
// parsed as an integer, or fallback if the variable is empty or unset.
func envInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", key, v, err)
	}
	return n, nil
}

// parseAllowedUserIDs converts a comma-separated string of user IDs into a set.
func parseAllowedUserIDs(raw string) (map[int64]struct{}, error) {
	result := make(map[int64]struct{})
	if raw == "" {
		return result, nil
	}
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID %q: %w", s, err)
		}
		result[id] = struct{}{}
	}
	return result, nil
}
