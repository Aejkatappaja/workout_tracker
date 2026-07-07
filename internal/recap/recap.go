// Package recap sends users a weekly summary email of their training activity.
package recap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/mail"
	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/Aejkatappaja/go-gym/internal/web/views"
)

// window is how far back a recap looks, and the minimum gap between two recaps
// for the same user. A user gets at most one recap per 7 days.
const window = 7 * 24 * time.Hour

type Service struct {
	store  store.RecapStore
	mailer mail.Mailer
	logger *slog.Logger
	appURL string
}

func NewService(rs store.RecapStore, mailer mail.Mailer, logger *slog.Logger, appURL string) *Service {
	return &Service{store: rs, mailer: mailer, logger: logger, appURL: appURL}
}

// Run sends due recaps once on start, then every interval until ctx is cancelled.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	s.SendDue(ctx, time.Now())

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.SendDue(ctx, now)
		}
	}
}

// SendDue finds every user due for a recap and emails them. It claims each user
// before sending (a conditional stamp), so overlapping runs never double-send.
func (s *Service) SendDue(ctx context.Context, now time.Time) {
	cutoff := now.Add(-window)
	candidates, err := s.store.DueForRecap(cutoff)
	if err != nil {
		s.logger.Error("recap: due lookup", "err", err)
		return
	}

	for _, c := range candidates {
		if ctx.Err() != nil {
			return
		}

		summary, err := s.store.WeeklyRecap(c.UserID, cutoff)
		if err != nil {
			s.logger.Error("recap: summary", "user", c.UserID, "err", err)
			continue
		}
		if summary.Sessions == 0 {
			continue // nothing worth reporting
		}

		claimed, err := s.store.MarkRecapSent(c.UserID, now, cutoff)
		if err != nil {
			s.logger.Error("recap: claim", "user", c.UserID, "err", err)
			continue
		}
		if !claimed {
			continue // another run already sent this week
		}

		s.send(ctx, c, summary)
	}
}

func (s *Service) send(ctx context.Context, c store.RecapCandidate, sum store.RecapSummary) {
	var sb strings.Builder
	if err := views.RecapEmail(c.Username, sum, s.appURL).Render(ctx, &sb); err != nil {
		s.logger.Error("recap: render", "user", c.UserID, "err", err)
		return
	}

	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := s.mailer.Send(sendCtx, c.Email, "your go-gym week", sb.String(), recapText(c.Username, sum, s.appURL)); err != nil {
		s.logger.Error("recap: send", "user", c.UserID, "err", err)
		return
	}
	s.logger.Info("recap sent", "user", c.UserID, "sessions", sum.Sessions)
}

func recapText(username string, s store.RecapSummary, appURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "your go-gym week, %s\n\n", username)
	fmt.Fprintf(&b, "Over the last 7 days: %d sessions, %.0f total volume.\n", s.Sessions, s.Volume)
	if s.BestExercise != "" {
		fmt.Fprintf(&b, "Best lift: %s at %.0f estimated 1RM.\n", s.BestExercise, s.BestE1RM)
	}
	fmt.Fprintf(&b, "\nOpen go-gym: %s\n\ngo-gym", appURL)
	return b.String()
}
