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
