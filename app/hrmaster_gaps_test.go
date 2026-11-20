package app

import (
	"reflect"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/web"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// Today is 2026-03-10 (newService). February was never read; March was read before 03-03 happened.
func TestHrmasterGaps(t *testing.T) {
	s, _ := newService(t)
	worked := func(day string, sec int) core.DayRecord {
		return core.DayRecord{Day: day, Status: core.DayWorkday, ReportedWorked: core.Ptr(sec)}
	}
	must(t, s.Store.ImportTimesheet(core.ParsedTimesheet{
		Sessions: []core.WorkSession{{Day: "2026-03-02", Start: ts.At("2026-03-02", "09:00"), End: ts.At("2026-03-02", "17:00"), Kind: core.SessionWork}},
		Days: []core.DayRecord{
			worked("2026-03-02", 0), worked("2026-03-03", 0), worked("2026-03-04", 8*3600), worked("2026-03-05", 8*3600),
			{Day: "2026-03-06", Status: core.DayLeave},
			{Day: "2026-03-07", Status: core.DayRestDay},
			{Day: "2026-03-08", Status: core.DayRestDay},
			worked("2026-03-09", 8*3600), worked("2026-03-10", 0),
		},
	}))
	for _, c := range []struct {
		from, to string
		want     []web.Month
	}{
		{"2026-02-20", "2026-03-31", []web.Month{{Year: 2026, Month: 2}, {Year: 2026, Month: 3}}}, // 03-03: a workday read with nothing
		{"2026-03-04", "2026-03-31", nil}, // filled up to yesterday; today waits
		{"2026-03-02", "2026-03-02", nil}, // sessions count as filled
		{"2026-03-11", "2026-03-31", nil}, // the future has nothing to read
	} {
		got, err := s.hrmasterGaps(c.from, c.to)
		must(t, err)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s – %s: %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
