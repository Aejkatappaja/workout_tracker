package views

import (
	"fmt"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

// optInt renders a *int as its value or an empty string when nil.
func optInt(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

// optFloat renders a *float64 as a compact string or empty when nil.
func optFloat(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'g', -1, 64)
}

// zeroBlank renders 0 as an empty string (nicer for optional number inputs).
func zeroBlank(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func workoutPath(id int) string     { return fmt.Sprintf("/app/workouts/%d", id) }
func workoutEditPath(id int) string { return fmt.Sprintf("/app/workouts/%d/edit", id) }
