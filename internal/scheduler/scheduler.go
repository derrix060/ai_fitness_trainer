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
	sched, err := gocron.NewScheduler(gocron.WithLocation(cfg.Timezone))
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
		cfg.BriefingHour, cfg.BriefingMinute, cfg.Timezone.String())
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
