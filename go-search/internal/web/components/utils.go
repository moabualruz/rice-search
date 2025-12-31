package components

import (
	"fmt"
	"math"
	"time"
)

// formatBytes formats bytes into human readable string
func formatBytes(size int64) string {
	if size == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	i := int(math.Floor(math.Log(float64(size)) / math.Log(1024)))
	if i >= len(units) {
		i = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(size)/math.Pow(1024, float64(i)), units[i])
}

// formatRelativeTime formats time as relative string
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	duration := time.Since(t)
	if duration < time.Minute {
		return "just now"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	}
	if duration < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(duration.Hours()/24))
	}
	return t.Format("2006-01-02")
}
