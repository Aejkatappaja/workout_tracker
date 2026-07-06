package views

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildActivity(t *testing.T) {
	today := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	counts := map[string]int{
		today.Format("2006-01-02"):                    3,
		today.AddDate(0, 0, -1).Format("2006-01-02"):  1,
		today.AddDate(0, 0, -40).Format("2006-01-02"): 6,
	}

	a := BuildActivity(counts, 16, today)

	assert.Len(t, a.Weeks, 16, "16 week columns")
	for _, w := range a.Weeks {
		assert.Len(t, w, 7, "7 days per column")
	}
	assert.Equal(t, 10, a.Total, "3 + 1 + 6")

	last := a.Weeks[15][int(today.Weekday())]
	assert.Equal(t, today.Format("2006-01-02"), last.Date)
	assert.Equal(t, 3, last.Count)
	assert.Equal(t, 3, last.Level, "count 3 -> level 3")

	if int(today.Weekday()) < 6 {
		assert.Equal(t, -1, a.Weeks[15][6].Level, "future day not rendered")
	}
}

func TestBuildActivityLevels(t *testing.T) {
	today := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	cases := map[int]int{0: 0, 1: 1, 2: 2, 3: 3, 4: 3, 5: 4, 20: 4}
	for count, wantLevel := range cases {
		a := BuildActivity(map[string]int{today.Format("2006-01-02"): count}, 4, today)
		got := a.Weeks[3][int(today.Weekday())].Level
		assert.Equal(t, wantLevel, got, "count %d", count)
	}
}
