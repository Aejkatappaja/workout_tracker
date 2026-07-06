package views

import "time"

type Stats struct {
	Sessions int
	Minutes  int
	Calories int
}

// ActivityDay is one heatmap cell; Level is 0..4 (intensity) or -1 for a future day.
type ActivityDay struct {
	Date  string
	Count int
	Level int
}

type Activity struct {
	Weeks [][]ActivityDay
	Total int
}

// BuildActivity turns a per-day count map ("YYYY-MM-DD" -> n) into a weeks x 7
// grid whose last column contains today.
func BuildActivity(counts map[string]int, weeks int, today time.Time) Activity {
	end := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	start := end.AddDate(0, 0, -(weeks-1)*7-int(end.Weekday()))

	grid := make([][]ActivityDay, 0, weeks)
	total := 0
	for w := 0; w < weeks; w++ {
		col := make([]ActivityDay, 7)
		for d := 0; d < 7; d++ {
			day := start.AddDate(0, 0, w*7+d)
			c := counts[day.Format("2006-01-02")]
			total += c
			col[d] = ActivityDay{Date: day.Format("2006-01-02"), Count: c, Level: level(c, day, end)}
		}
		grid = append(grid, col)
	}
	return Activity{Weeks: grid, Total: total}
}

func level(c int, day, today time.Time) int {
	switch {
	case day.After(today):
		return -1
	case c == 0:
		return 0
	case c == 1:
		return 1
	case c == 2:
		return 2
	case c <= 4:
		return 3
	default:
		return 4
	}
}
