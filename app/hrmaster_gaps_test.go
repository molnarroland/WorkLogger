package app

import (
	"reflect"
	"testing"

	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/web"

	ts "github.com/molnarroland/WorkLogger/internal/testsupport"
)

// Today is 2026-03-10 (newService). February was never read; March was read on 03-05, in the middle of that day.
func TestHrmasterGaps(t *testing.T) {
	s, _ := newService(t)
	worked := func(day string, sec int) core.DayRecord {
		return core.DayRecord{Day: day, Status: core.DayWorkday, ReportedWorked: core.Ptr(sec), ReadOn: "2026-03-05"}
	}
	must(t, s.Store.ImportTimesheet(core.ParsedTimesheet{
		Sessions: []core.WorkSession{{Day: "2026-03-02", Start: ts.At("2026-03-02", "09:00"), End: ts.At("2026-03-02", "17:00"), Kind: core.SessionWork}},
		Days: []core.DayRecord{
			worked("2026-03-02", 8*3600), worked("2026-03-03", 8*3600), worked("2026-03-04", 8*3600),
			worked("2026-03-05", 3*3600), // read before the day ended
			{Day: "2026-03-06", Status: core.DayLeave},
			{Day: "2026-03-07", Status: core.DayRestDay},
			{Day: "2026-03-08", Status: core.DayRestDay},
			worked("2026-03-09", 0), worked("2026-03-10", 0), // read before they happened
		},
	}))
	feb, mar := web.Month{Year: 2026, Month: 2}, web.Month{Year: 2026, Month: 3}
	for _, c := range []struct {
		from, to string
		want     []web.Month
	}{
		{"2026-02-20", "2026-03-31", []web.Month{feb, mar}},
		{"2026-03-02", "2026-03-04", nil},              // read after they ended
		{"2026-03-05", "2026-03-05", []web.Month{mar}}, // read during the day
		{"2026-03-06", "2026-03-08", nil},              // leave and rest days are final
		{"2026-03-10", "2026-03-31", []web.Month{mar}}, // today is always read
		{"2026-03-11", "2026-03-31", nil},              // the future has nothing to read
	} {
		got, err := s.hrmasterGaps(c.from, c.to)
		must(t, err)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s – %s: %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestHrmasterFinal(t *testing.T) {
	legacy := &core.DayRecord{Day: "2026-03-02", Status: core.DayWorkday, ReportedWorked: core.Ptr(8 * 3600)}
	for _, c := range []struct {
		name        string
		r           *core.DayRecord
		hasSessions bool
		want        bool
	}{
		{"nothing read", nil, false, false},
		{"sessions without a record", nil, true, true},
		{"a record from before the read day was kept", legacy, true, false},
		{"read the day after", &core.DayRecord{Day: "2026-03-02", Status: core.DayWorkday, ReadOn: "2026-03-03"}, true, true},
		{"read the same day", &core.DayRecord{Day: "2026-03-02", Status: core.DayWorkday, ReadOn: "2026-03-02"}, true, false},
		{"a holiday read in advance", &core.DayRecord{Day: "2026-03-02", Status: core.DayPublicHoliday, ReadOn: "2026-02-01"}, false, true},
	} {
		if got := hrmasterFinal("2026-03-02", c.r, c.hasSessions); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
