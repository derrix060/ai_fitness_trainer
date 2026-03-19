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
	part1 := strings.Repeat("a", 2000) + "\n" + strings.Repeat("b", 1000)
	text := part1 + "\n\n" + strings.Repeat("c", 3000)

	chunks := SplitMessage(text)
	if len(chunks) < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != part1 {
		t.Errorf("first chunk should be everything before \\n\\n")
	}
}
