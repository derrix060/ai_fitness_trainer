package main

import (
	"strings"
	"testing"
	"time"
)

const sampleFeed = "BEGIN:VCALENDAR\r\n" +
	"BEGIN:VEVENT\r\n" +
	"DESCRIPTION:Workout type: Swim\\nPlanned Time: 1:02\\nDis\r\n tance Planned: 2700 m\\n\r\n" +
	"DTEND:20261103T090921\r\n" +
	"DTSTART:20261103T080721\r\n" +
	"SUMMARY:Swim: 8h00\r\n" +
	"END:VEVENT\r\n" +
	"BEGIN:VEVENT\r\n" +
	"DESCRIPTION:Workout type: Day Off\\n\r\n" +
	"DTEND;VALUE=DATE:20261105\r\n" +
	"DTSTART;VALUE=DATE:20261104\r\n" +
	"SUMMARY:Day Off: Descanso\r\n" +
	"END:VEVENT\r\n" +
	"BEGIN:VEVENT\r\n" +
	"DTSTART:20261105T170000Z\r\n" +
	"SUMMARY:Run\\, easy\r\n" +
	"END:VEVENT\r\n" +
	"BEGIN:VEVENT\r\n" +
	"DTSTART:garbage\r\n" +
	"SUMMARY:Broken\r\n" +
	"END:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestParseWorkouts(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Lisbon")
	if err != nil {
		t.Fatal(err)
	}
	got := parseWorkouts([]byte(sampleFeed), loc)
	if len(got) != 3 {
		t.Fatalf("got %d workouts, want 3 (unparseable DTSTART must be dropped): %+v", len(got), got)
	}

	swim := got[0]
	if swim.Start != "2026-11-03T08:07:21" || swim.AllDay {
		t.Errorf("floating time: start=%q all_day=%v", swim.Start, swim.AllDay)
	}
	if !strings.Contains(swim.Description, "Distance Planned: 2700 m") || !strings.Contains(swim.Description, "\n") {
		t.Errorf("folded line / \\n escape not handled: %q", swim.Description)
	}

	if off := got[1]; off.Start != "2026-11-04" || !off.AllDay {
		t.Errorf("date-only: start=%q all_day=%v", off.Start, off.AllDay)
	}

	if run := got[2]; run.Start != "2026-11-05T17:00:00" || run.Title != "Run, easy" {
		t.Errorf("UTC converted to local (WET, no DST in Nov): start=%q title=%q", run.Start, run.Title)
	}
}

func TestDayRangeRejectsBadDates(t *testing.T) {
	if _, _, err := dayRange("06/11/2026", "", time.Now()); err == nil || !strings.Contains(err.Error(), "from must be YYYY-MM-DD") {
		t.Errorf("err = %v", err)
	}
	if _, _, err := dayRange("", "tomorrow", time.Now()); err == nil || !strings.Contains(err.Error(), "to must be YYYY-MM-DD") {
		t.Errorf("err = %v", err)
	}
}
