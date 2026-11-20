package core

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ParsedTimesheet struct {
	Sessions []WorkSession
	Days     []DayRecord
	Warnings []string
}

func (t ParsedTimesheet) DayKeys() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range t.Days {
		if !seen[d.Day] {
			seen[d.Day] = true
			out = append(out, d.Day)
		}
	}
	for _, s := range t.Sessions {
		if !seen[s.Day] {
			seen[s.Day] = true
			out = append(out, s.Day)
		}
	}
	sort.Strings(out)
	return out
}

// HRmasterParser parses HRmaster's monthly "Munkaidő" table (Datacenter → Calendar) as HTML, through its stable
// `data-e2e-testing` attributes.
type HRmasterParser struct {
	FallbackYear, FallbackMonth int
	Days                        Days
}

const hrRowMarker = `data-e2e-testing="Datacenter.Employee.Calendar.RowRepeater"`

var (
	hrPeriod      = regexp.MustCompile(`data-e2e-testing="Datacenter\.Calendar\.PeriodSelect"[^>]*>\s*(\d{4})\.?\s*([^\s<]+)`)
	hrDay         = regexp.MustCompile(`data-e2e-testing="Datacenter\.Calendar\.Day"[^>]*>\s*(\d{1,2})`)
	hrStatusType  = regexp.MustCompile(`Type_CalendarDayStatus_([A-Za-z]+)`)
	hrStatusLabel = regexp.MustCompile(`data-e2e-testing="Datacenter\.Calendar\.Day\.Status\.[A-Za-z]+"[^>]*>\s*<div[^>]*>([^<]*)`)
	hrWorked      = regexp.MustCompile(`FactWorkedHoursSum"[^>]*>\s*(\d{1,3}):(\d{2})`)
	hrUnpaid      = regexp.MustCompile(`UnpaidRestBreaksSum"[^>]*>\s*(\d{1,3}):(\d{2})`)
	hrCell        = regexp.MustCompile(`data-e2e-testing="Datacenter\.(?:Employee\.Calendar\.(WorkScheduleRepeater|RestBreaksRepeater)|Calendar\.Actual\.PaidRestBreak\.Column)"`)
	hrPicker      = regexp.MustCompile(`data-e2e-testing="Evolution\.TimepickerSwitchInput"[^>]*>([^<]*)<`)
	hrInputValue  = regexp.MustCompile(`value="(\d{1,2}:\d{2})"`)
	hrTime        = regexp.MustCompile(`^\s*(\d{1,2}):(\d{2})(?::\d{2})?\s*$`)
	hrEntity      = regexp.MustCompile(`&#(x?)([0-9a-fA-F]+);`)
)

var hrMonths = map[string]int{
	"január": 1, "február": 2, "március": 3, "április": 4, "május": 5, "június": 6,
	"július": 7, "augusztus": 8, "szeptember": 9, "október": 10, "november": 11, "december": 12,
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
	"july": 7, "august": 8, "september": 9, "october": 10,
}

func HRmasterMatches(raw string) bool { return strings.Contains(raw, hrRowMarker) }

func MonthNumber(s string) (int, bool) {
	key := strings.Trim(strings.ToLower(s), ". ")
	if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= 12 {
		return n, true
	}
	n, ok := hrMonths[key]
	return n, ok
}

func (p HRmasterParser) Parse(raw string) ParsedTimesheet {
	var result ParsedTimesheet
	type monthBlock struct {
		text        string
		year, month int
	}
	var blocks []monthBlock
	periods := hrPeriod.FindAllStringSubmatchIndex(raw, -1)
	if len(periods) == 0 {
		blocks = []monthBlock{{raw, p.FallbackYear, p.FallbackMonth}}
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("No month header found; assumed %d-%02d.", p.FallbackYear, p.FallbackMonth))
	} else {
		// Rows before the first period header use the fallback month.
		if periods[0][0] > 0 {
			blocks = append(blocks, monthBlock{raw[:periods[0][0]], p.FallbackYear, p.FallbackMonth})
		}
		for i, m := range periods {
			end := len(raw)
			if i+1 < len(periods) {
				end = periods[i+1][0]
			}
			year, err := strconv.Atoi(raw[m[2]:m[3]])
			if err != nil {
				year = p.FallbackYear
			}
			monthText := raw[m[4]:m[5]]
			month, ok := MonthNumber(monthText)
			if !ok {
				month = p.FallbackMonth
				result.Warnings = append(result.Warnings, fmt.Sprintf("Unknown month “%s”; assumed %d.", monthText, month))
			}
			blocks = append(blocks, monthBlock{raw[m[0]:end], year, month})
		}
	}
	for _, b := range blocks {
		for _, chunk := range strings.Split(b.text, hrRowMarker)[1:] {
			p.parseRow(chunk, b.year, b.month, &result)
		}
	}
	sort.SliceStable(result.Sessions, func(i, j int) bool { return result.Sessions[i].Start.Before(result.Sessions[j].Start) })
	sort.SliceStable(result.Days, func(i, j int) bool { return result.Days[i].Day < result.Days[j].Day })
	return result
}

func hrDuration(re *regexp.Regexp, row string) *int {
	m := re.FindStringSubmatch(row)
	if m == nil {
		return nil
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	return Ptr((h*60 + mi) * 60)
}

func (p HRmasterParser) parseRow(row string, year, month int, result *ParsedTimesheet) {
	dm := hrDay.FindStringSubmatch(row)
	if dm == nil {
		return
	}
	dayNumber, _ := strconv.Atoi(dm[1])
	date := time.Date(year, time.Month(month), dayNumber, 0, 0, 0, 0, p.Days.Loc)
	if int(date.Month()) != month || dayNumber < 1 {
		return
	}
	day := p.Days.Key(date)

	status := DayOther
	if m := hrStatusType.FindStringSubmatch(row); m != nil {
		status = DayStatusFromHRmaster(m[1])
	}
	label := ""
	if m := hrStatusLabel.FindStringSubmatch(row); m != nil {
		label = strings.TrimSpace(DecodeEntities(m[1]))
	}
	record := DayRecord{
		Day: day, Status: status, Label: label,
		ReportedWorked: hrDuration(hrWorked, row), ReportedBreaks: hrDuration(hrUnpaid, row),
	}
	result.Days = append(result.Days, record)

	// Cells: each marker starts a cell that runs until the next marker.
	cells := hrCell.FindAllStringSubmatchIndex(row, -1)
	workIndex, breakIndex := 0, 0
	var daySessions []WorkSession
	for i, cell := range cells {
		if cell[2] < 0 {
			continue // paid-break column
		}
		isWork := row[cell[2]:cell[3]] == "WorkScheduleRepeater"
		end := len(row)
		if i+1 < len(cells) {
			end = cells[i+1][0]
		}
		segment := row[cell[1]:end]
		if isWork {
			workIndex++
		} else {
			breakIndex++
		}
		from, to, ok := hrTimes(segment)
		if !ok {
			continue
		}
		s, ok1 := p.hrDate(day, from)
		e, ok2 := p.hrDate(day, to)
		if !ok1 || !ok2 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: could not read %s–%s.", day, from, to))
			continue
		}
		if !e.After(s) {
			e = e.AddDate(0, 0, 1) // past midnight
		}
		ws := WorkSession{Day: day, Start: s, End: e, Kind: SessionWork, Label: fmt.Sprintf("Munkaidő %d", workIndex)}
		if !isWork {
			ws.Kind, ws.Label = SessionPause, fmt.Sprintf("Szünet %d", breakIndex)
		}
		daySessions = append(daySessions, ws)
	}
	result.Sessions = append(result.Sessions, daySessions...)

	// Cross-check against the totals the HR system shows.
	if work := intervals(daySessions, SessionWork); len(work) > 0 && record.ReportedWorked != nil {
		net := int(Total(Subtract(work, intervals(daySessions, SessionPause))).Seconds())
		if d := net - *record.ReportedWorked; d > 60 || d < -60 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: sessions add up to %s, HRmaster says %s.",
				day, FormatDuration(net), FormatDuration(*record.ReportedWorked)))
		}
	}
}

// hrTimes is a cell's From and To, either as rendered text or as an input's value (edit mode).
func hrTimes(segment string) (string, string, bool) {
	var values []string
	for _, m := range hrPicker.FindAllStringSubmatch(segment, -1) {
		if v := strings.TrimSpace(m[1]); v != "" {
			values = append(values, v)
		}
	}
	if len(values) < 2 {
		values = nil
		for _, m := range hrInputValue.FindAllStringSubmatch(segment, -1) {
			values = append(values, m[1])
		}
	}
	if len(values) < 2 {
		return "", "", false
	}
	return values[0], values[1], true
}

func (p HRmasterParser) hrDate(day, hm string) (time.Time, bool) {
	m := hrTime.FindStringSubmatch(hm)
	if m == nil {
		return time.Time{}, false
	}
	h, _ := strconv.Atoi(m[1])
	mi, _ := strconv.Atoi(m[2])
	if h == 24 && mi == 0 { // "24:00" = midnight at the end of the day
		return p.Days.AddDays(p.Days.Date(day), 1), true
	}
	return p.Days.At(day, h, mi)
}

// DecodeEntities handles named and numeric HTML entities (&nbsp;, &#233;, &#x151;…) in cell text.
func DecodeEntities(s string) string {
	for _, kv := range [][2]string{
		{"&nbsp;", " "},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", "\""},
		{"&#39;", "'"},
		{"&apos;", "'"},
		{"&ndash;", "–"},
		{"&mdash;", "—"},
		{"&amp;", "&"},
	} {
		s = strings.ReplaceAll(s, kv[0], kv[1])
	}
	return hrEntity.ReplaceAllStringFunc(s, func(e string) string {
		m := hrEntity.FindStringSubmatch(e)
		base := 10
		if m[1] == "x" {
			base = 16
		}
		code, err := strconv.ParseUint(m[2], base, 32)
		if err != nil || code > 0x10FFFF || (code >= 0xD800 && code <= 0xDFFF) {
			return e
		}
		return string(rune(code))
	})
}

// MergeTimesheets combines the timesheets read from several HRmaster profiles. Each day comes whole (its sessions
// and its record) from the first timesheet that has something on it: sessions or reported hours, else just a
// record. So a day off at one company doesn't hide the day worked at another.
func MergeTimesheets(list []ParsedTimesheet) ParsedTimesheet {
	if len(list) == 1 {
		return list[0]
	}
	has := func(t ParsedTimesheet, day string) bool {
		for _, s := range t.Sessions {
			if s.Day == day {
				return true
			}
		}
		for _, d := range t.Days {
			if d.Day == day && d.ReportedWorked != nil && *d.ReportedWorked > 0 {
				return true
			}
		}
		return false
	}
	source := map[string]int{}
	var order []string
	for i, t := range list {
		for _, day := range t.DayKeys() {
			j, seen := source[day]
			if !seen {
				order = append(order, day)
				source[day] = i
			} else if !has(list[j], day) && has(t, day) {
				source[day] = i
			}
		}
	}
	var out ParsedTimesheet
	for _, day := range order {
		t := list[source[day]]
		for _, s := range t.Sessions {
			if s.Day == day {
				out.Sessions = append(out.Sessions, s)
			}
		}
		for _, d := range t.Days {
			if d.Day == day {
				out.Days = append(out.Days, d)
			}
		}
	}
	for _, t := range list {
		out.Warnings = append(out.Warnings, t.Warnings...)
	}
	return out
}
