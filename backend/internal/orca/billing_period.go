package orca

import (
	"errors"
	"time"
)

type BillingAnchor struct {
	Month time.Month
	Day   int
}

func AnchorFor(start time.Time, interval string, location *time.Location) BillingAnchor {
	if location == nil {
		location = time.UTC
	}
	local := start.In(location)
	if interval == "yearly" {
		return BillingAnchor{Month: local.Month(), Day: local.Day()}
	}
	return BillingAnchor{Month: local.Month(), Day: local.Day()}
}

// NextPeriod preserves the original calendar anchor when a month is shorter
// than that anchor, e.g. Jan 31 -> Feb 28 -> Mar 31.
func NextPeriod(start time.Time, interval string, anchor BillingAnchor, location *time.Location) (time.Time, error) {
	if location == nil {
		location = time.UTC
	}
	local := start.In(location)
	switch interval {
	case "daily":
		return local.AddDate(0, 0, 1).UTC(), nil
	case "weekly":
		return local.AddDate(0, 0, 7).UTC(), nil
	case "monthly":
		y, m := local.Year(), local.Month()+1
		if m > 12 {
			y++
			m = 1
		}
		day := minDay(y, m, anchor.Day)
		return time.Date(y, m, day, local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), location).UTC(), nil
	case "yearly":
		y := local.Year() + 1
		day := minDay(y, anchor.Month, anchor.Day)
		return time.Date(y, anchor.Month, day, local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), location).UTC(), nil
	default:
		return time.Time{}, errors.New("unsupported billing interval")
	}
}

func minDay(year int, month time.Month, day int) int {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		return last
	}
	if day < 1 {
		return 1
	}
	return day
}

func ParseLocalDate(value, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, errors.New("invalid business timezone")
	}
	d, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, errors.New("date must use YYYY-MM-DD")
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc).UTC(), nil
}
