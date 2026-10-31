package orca

import (
	"testing"
	"time"
)

func TestMonthlyAnchorReturnsToDay31(t *testing.T) {
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal(err)
	}
	jan31 := time.Date(2025, time.January, 31, 9, 30, 0, 0, location)
	anchor := AnchorFor(jan31, "monthly", location)
	feb28, err := NextPeriod(jan31, "monthly", anchor, location)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := feb28.In(location).Format("2006-01-02"), "2025-02-28"; got != want {
		t.Fatalf("February boundary = %s, want %s", got, want)
	}
	mar31, err := NextPeriod(feb28, "monthly", anchor, location)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := mar31.In(location).Format("2006-01-02"), "2025-03-31"; got != want {
		t.Fatalf("March boundary = %s, want %s", got, want)
	}
}

func TestAnchorForNilLocationUsesUTC(t *testing.T) {
	start := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	anchor := AnchorFor(start, "monthly", nil)
	if anchor.Day != 31 || anchor.Month != time.January {
		t.Fatalf("unexpected anchor: %+v", anchor)
	}
}
