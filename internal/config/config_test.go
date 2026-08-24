package config

import (
	"os"
	"testing"
	"time"
)

// helper to clear all config-related env vars before each test.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"TELEGRAM_BOT_TOKEN",
		"ALLOWED_TELEGRAM_USER_IDS",
		"CLAUDE_MODEL",
		"CLAUDE_TIMEOUT",
		"INTERVALS_API_KEY",
		"INTERVALS_ATHLETE_ID",
		"DATA_DIR",
		"BRIEFING_HOUR",
		"BRIEFING_MINUTE",
		"TZ",
		"LOG_LEVEL",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func TestLoad_MissingTelegramToken(t *testing.T) {
	clearEnv(t)
	t.Setenv("TZ", "UTC")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing TELEGRAM_BOT_TOKEN, got nil")
	}
}

func TestLoad_MissingTZ(t *testing.T) {
	clearEnv(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	// TZ is unset after clearEnv

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing TZ, got nil")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TZ", "UTC")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ClaudeModel != "sonnet" {
		t.Errorf("ClaudeModel = %q, want %q", cfg.ClaudeModel, "sonnet")
	}
	if cfg.ClaudeTimeout != 1200*time.Second {
		t.Errorf("ClaudeTimeout = %v, want %v", cfg.ClaudeTimeout, 1200*time.Second)
	}
	if cfg.DataDir != "data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "data")
	}
	if cfg.BriefingHour != 7 {
		t.Errorf("BriefingHour = %d, want %d", cfg.BriefingHour, 7)
	}
	if cfg.BriefingMinute != 0 {
		t.Errorf("BriefingMinute = %d, want %d", cfg.BriefingMinute, 0)
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "INFO")
	}
}

func TestLoad_AllowedUserIDs(t *testing.T) {
	clearEnv(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TZ", "UTC")
	t.Setenv("ALLOWED_TELEGRAM_USER_IDS", "111,222,333")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, id := range []int64{111, 222, 333} {
		if !cfg.IsAllowed(id) {
			t.Errorf("expected user %d to be allowed", id)
		}
	}
	if cfg.IsAllowed(999) {
		t.Error("expected user 999 to NOT be allowed")
	}
}

func TestLoad_InvalidUserID(t *testing.T) {
	clearEnv(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TZ", "UTC")
	t.Setenv("ALLOWED_TELEGRAM_USER_IDS", "111,notanumber,333")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid user ID, got nil")
	}
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "custom-token")
	t.Setenv("TZ", "America/New_York")
	t.Setenv("CLAUDE_MODEL", "opus")
	t.Setenv("CLAUDE_TIMEOUT", "300")
	t.Setenv("INTERVALS_API_KEY", "my-api-key")
	t.Setenv("INTERVALS_ATHLETE_ID", "i99999")
	t.Setenv("DATA_DIR", "/tmp/mydata")
	t.Setenv("BRIEFING_HOUR", "9")
	t.Setenv("BRIEFING_MINUTE", "30")
	t.Setenv("LOG_LEVEL", "DEBUG")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.TelegramBotToken != "custom-token" {
		t.Errorf("TelegramBotToken = %q, want %q", cfg.TelegramBotToken, "custom-token")
	}
	if cfg.ClaudeModel != "opus" {
		t.Errorf("ClaudeModel = %q, want %q", cfg.ClaudeModel, "opus")
	}
	if cfg.ClaudeTimeout != 300*time.Second {
		t.Errorf("ClaudeTimeout = %v, want %v", cfg.ClaudeTimeout, 300*time.Second)
	}
	if cfg.IntervalsAPIKey != "my-api-key" {
		t.Errorf("IntervalsAPIKey = %q, want %q", cfg.IntervalsAPIKey, "my-api-key")
	}
	if cfg.IntervalsAthleteID != "i99999" {
		t.Errorf("IntervalsAthleteID = %q, want %q", cfg.IntervalsAthleteID, "i99999")
	}
	if cfg.DataDir != "/tmp/mydata" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "/tmp/mydata")
	}
	if cfg.BriefingHour != 9 {
		t.Errorf("BriefingHour = %d, want %d", cfg.BriefingHour, 9)
	}
	if cfg.BriefingMinute != 30 {
		t.Errorf("BriefingMinute = %d, want %d", cfg.BriefingMinute, 30)
	}
	if cfg.Timezone.String() != "America/New_York" {
		t.Errorf("Timezone = %q, want %q", cfg.Timezone.String(), "America/New_York")
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "DEBUG")
	}
}

func TestLoad_EmptyAllowedUsers(t *testing.T) {
	clearEnv(t)
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TZ", "UTC")
	// ALLOWED_TELEGRAM_USER_IDS not set

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.AllowedUserIDs) != 0 {
		t.Errorf("AllowedUserIDs length = %d, want 0", len(cfg.AllowedUserIDs))
	}
	if cfg.IsAllowed(123) {
		t.Error("expected no users to be allowed when list is empty")
	}
}
