package recap

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Aejkatappaja/go-gym/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRecapStore struct {
	due       []store.RecapCandidate
	summaries map[int]store.RecapSummary
	claimable map[int]bool // MarkRecapSent result; defaults to true
	marked    []int
}

func (f *fakeRecapStore) DueForRecap(time.Time) ([]store.RecapCandidate, error) {
	return f.due, nil
}

func (f *fakeRecapStore) WeeklyRecap(userID int, _ time.Time) (store.RecapSummary, error) {
	return f.summaries[userID], nil
}

func (f *fakeRecapStore) MarkRecapSent(userID int, _, _ time.Time) (bool, error) {
	f.marked = append(f.marked, userID)
	if v, ok := f.claimable[userID]; ok {
		return v, nil
	}
	return true, nil
}

type fakeMailer struct{ sentTo []string }

func (m *fakeMailer) Send(_ context.Context, to, _, _, _ string) error {
	m.sentTo = append(m.sentTo, to)
	return nil
}

func testService(rs store.RecapStore, m *fakeMailer) *Service {
	return NewService(rs, m, slog.New(slog.NewTextHandler(io.Discard, nil)), "https://go-gym.test")
}

func TestSendDue(t *testing.T) {
	rs := &fakeRecapStore{
		due: []store.RecapCandidate{
			{UserID: 1, Username: "neo", Email: "neo@example.com"},
			{UserID: 2, Username: "trin", Email: "trin@example.com"},
		},
		summaries: map[int]store.RecapSummary{
			1: {Sessions: 3, Volume: 4200, BestExercise: "bench press", BestE1RM: 116},
			2: {Sessions: 0}, // trained window empty of weighted sets; skip
		},
	}
	m := &fakeMailer{}
	testService(rs, m).SendDue(context.Background(), time.Now())

	assert.Equal(t, []string{"neo@example.com"}, m.sentTo, "only users with sessions get a recap")
	assert.Equal(t, []int{1}, rs.marked, "the zero-session user is skipped before the claim")
}

func TestSendDue_SkipsUnclaimed(t *testing.T) {
	rs := &fakeRecapStore{
		due:       []store.RecapCandidate{{UserID: 1, Username: "neo", Email: "neo@example.com"}},
		summaries: map[int]store.RecapSummary{1: {Sessions: 2, Volume: 800}},
		claimable: map[int]bool{1: false}, // another run already claimed it
	}
	m := &fakeMailer{}
	testService(rs, m).SendDue(context.Background(), time.Now())

	require.Empty(t, m.sentTo, "a user already claimed by another run is not emailed again")
}
