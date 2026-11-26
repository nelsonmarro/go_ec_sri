package sri

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTodayAt(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{"21:00 in Ecuador is already tomorrow in UTC", time.Date(2026, 11, 26, 2, 0, 0, 0, time.UTC), "2026-11-25"},
		{"noon UTC is the same day", time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC), "2026-11-26"},
		{"new year in UTC is still the old year", time.Date(2027, 1, 1, 4, 59, 0, 0, time.UTC), "2026-12-31"},
		{"05:00 UTC is midnight in Ecuador", time.Date(2027, 1, 1, 5, 0, 0, 0, time.UTC), "2027-01-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := todayAt(tt.now)
			assert.Equal(t, tt.want, got.Format("2006-01-02"))
			h, m, s := got.Clock()
			assert.Zero(t, h+m+s+got.Nanosecond())
			_, off := got.Zone()
			assert.Equal(t, -5*60*60, off)
		})
	}
}

func TestTodayIsEcuadorDate(t *testing.T) {
	before := time.Now().In(time.FixedZone("x", -5*3600)).Format("2006-01-02")
	got := Today().Format("2006-01-02")
	after := time.Now().In(time.FixedZone("x", -5*3600)).Format("2006-01-02")
	assert.Contains(t, []string{before, after}, got)
}
