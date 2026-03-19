package store

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSessionSaveAndGet(t *testing.T) {
	s := newTestStore(t)

	sess, err := s.GetSession(123)
	if err != nil {
		t.Fatalf("GetSession on empty DB: %v", err)
	}
	if sess != "" {
		t.Fatalf("expected empty session, got %q", sess)
	}

	if err := s.SaveSession(123, "sid-abc"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	sess, err = s.GetSession(123)
	if err != nil {
		t.Fatalf("GetSession after save: %v", err)
	}
	if sess != "sid-abc" {
		t.Fatalf("expected %q, got %q", "sid-abc", sess)
	}
}

func TestSessionOverwrite(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveSession(1, "first"); err != nil {
		t.Fatalf("SaveSession first: %v", err)
	}
	if err := s.SaveSession(1, "second"); err != nil {
		t.Fatalf("SaveSession second: %v", err)
	}

	sess, err := s.GetSession(1)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess != "second" {
		t.Fatalf("expected %q, got %q", "second", sess)
	}
}

func TestSessionDelete(t *testing.T) {
	s := newTestStore(t)

	if err := s.SaveSession(42, "to-delete"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if err := s.DeleteSession(42); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	sess, err := s.GetSession(42)
	if err != nil {
		t.Fatalf("GetSession after delete: %v", err)
	}
	if sess != "" {
		t.Fatalf("expected empty session after delete, got %q", sess)
	}
}

func TestKVSetAndGet(t *testing.T) {
	s := newTestStore(t)

	val, err := s.GetValue("missing")
	if err != nil {
		t.Fatalf("GetValue on missing key: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty string for missing key, got %q", val)
	}

	if err := s.SetValue("color", "blue"); err != nil {
		t.Fatalf("SetValue: %v", err)
	}

	val, err = s.GetValue("color")
	if err != nil {
		t.Fatalf("GetValue: %v", err)
	}
	if val != "blue" {
		t.Fatalf("expected %q, got %q", "blue", val)
	}
}

func TestKVOverwrite(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetValue("k", "v1"); err != nil {
		t.Fatalf("SetValue v1: %v", err)
	}
	if err := s.SetValue("k", "v2"); err != nil {
		t.Fatalf("SetValue v2: %v", err)
	}

	val, err := s.GetValue("k")
	if err != nil {
		t.Fatalf("GetValue: %v", err)
	}
	if val != "v2" {
		t.Fatalf("expected %q, got %q", "v2", val)
	}
}

func TestAnalyzedActivitiesMark(t *testing.T) {
	s := newTestStore(t)

	if err := s.MarkActivityAnalyzed("act-1"); err != nil {
		t.Fatalf("MarkActivityAnalyzed: %v", err)
	}
	if err := s.MarkActivityAnalyzed("act-2"); err != nil {
		t.Fatalf("MarkActivityAnalyzed: %v", err)
	}

	ids, err := s.GetAnalyzedActivities()
	if err != nil {
		t.Fatalf("GetAnalyzedActivities: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 activities, got %d", len(ids))
	}
	if _, ok := ids["act-1"]; !ok {
		t.Fatal("expected act-1 in set")
	}
	if _, ok := ids["act-2"]; !ok {
		t.Fatal("expected act-2 in set")
	}
}

func TestAnalyzedActivitiesDuplicateIgnored(t *testing.T) {
	s := newTestStore(t)

	if err := s.MarkActivityAnalyzed("dup"); err != nil {
		t.Fatalf("MarkActivityAnalyzed first: %v", err)
	}
	if err := s.MarkActivityAnalyzed("dup"); err != nil {
		t.Fatalf("MarkActivityAnalyzed duplicate: %v", err)
	}

	ids, err := s.GetAnalyzedActivities()
	if err != nil {
		t.Fatalf("GetAnalyzedActivities: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 activity after duplicate, got %d", len(ids))
	}
}

func TestGetLastActivityCheckDefaultsToToday(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetLastActivityCheck()
	if err != nil {
		t.Fatalf("GetLastActivityCheck: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	if got != today {
		t.Fatalf("expected today %q, got %q", today, got)
	}
}

func TestGetLastActivityCheckReturnsStored(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetValue("last_activity_check", "2025-12-25"); err != nil {
		t.Fatalf("SetValue: %v", err)
	}

	got, err := s.GetLastActivityCheck()
	if err != nil {
		t.Fatalf("GetLastActivityCheck: %v", err)
	}
	if got != "2025-12-25" {
		t.Fatalf("expected %q, got %q", "2025-12-25", got)
	}
}
