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
