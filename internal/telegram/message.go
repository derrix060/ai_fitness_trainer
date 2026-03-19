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
