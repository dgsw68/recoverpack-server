// Package disaster loads 행정안전부 긴급재난문자 (Ministry of the Interior and
// Safety emergency disaster text message) data from a local CSV export and
// serves it in memory until the public API approval comes through.
package disaster

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Alert mirrors one row of the 행정안전부_긴급재난문자 CSV export.
type Alert struct {
	SN            string `json:"sn"`
	CreatedAt     string `json:"createdAt"`
	Message       string `json:"message"`
	Region        string `json:"region"`
	EmergencyStep string `json:"emergencyStep"`
	DisasterType  string `json:"disasterType"`
	RegisteredAt  string `json:"registeredAt"`
	ModifiedAt    string `json:"modifiedAt"`
}

// Store holds the alerts loaded from the CSV file in memory.
type Store struct {
	alerts []Alert
}

var columnOrder = []string{"SN", "CRT_DT", "MSG_CN", "RCPTN_RGN_NM", "EMRG_STEP_NM", "DST_SE_NM", "REG_YMD", "MDFCN_YMD"}

// LoadStore reads the CSV file at path and returns a Store with its rows in memory.
func LoadStore(path string) (*Store, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open disaster alert csv: %w", err)
	}
	raw = bytes.TrimPrefix(raw, utf8BOM)

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = len(columnOrder)

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read disaster alert csv header: %w", err)
	}
	index := make(map[string]int, len(header))
	for i, col := range header {
		index[strings.TrimSpace(col)] = i
	}
	for _, want := range columnOrder {
		if _, ok := index[want]; !ok {
			return nil, fmt.Errorf("disaster alert csv missing expected column %q", want)
		}
	}

	var alerts []Alert
	first := true
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read disaster alert csv row: %w", err)
		}

		// The export has a second, Korean-language header row (e.g. SN="일련번호")
		// right after the English column header row. Skip it.
		if first {
			first = false
			if record[index["SN"]] == "일련번호" {
				continue
			}
		}

		alerts = append(alerts, Alert{
			SN:            record[index["SN"]],
			CreatedAt:     record[index["CRT_DT"]],
			Message:       record[index["MSG_CN"]],
			Region:        record[index["RCPTN_RGN_NM"]],
			EmergencyStep: record[index["EMRG_STEP_NM"]],
			DisasterType:  record[index["DST_SE_NM"]],
			RegisteredAt:  record[index["REG_YMD"]],
			ModifiedAt:    record[index["MDFCN_YMD"]],
		})
	}

	return &Store{alerts: alerts}, nil
}

// Query filters options for listing alerts.
type Query struct {
	Region       string
	DisasterType string
	Offset       int
	Limit        int
}

// List returns alerts matching the query, newest first (CSV export order), plus the total match count.
func (s *Store) List(q Query) ([]Alert, int) {
	region := strings.TrimSpace(q.Region)
	disasterType := strings.TrimSpace(q.DisasterType)

	matched := make([]Alert, 0, len(s.alerts))
	for _, a := range s.alerts {
		if region != "" && !strings.Contains(a.Region, region) {
			continue
		}
		if disasterType != "" && !strings.Contains(a.DisasterType, disasterType) {
			continue
		}
		matched = append(matched, a)
	}

	total := len(matched)
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	limit := q.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return matched[offset:end], total
}

// Len returns the number of alerts loaded.
func (s *Store) Len() int {
	return len(s.alerts)
}

var alertTimeLayouts = []string{
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02 15:04",
	"2006/01/02",
}

// ParseAlertTime parses a date/time string using the layouts understood by
// this package (CSV export format plus common ISO variants).
func ParseAlertTime(value string) (time.Time, bool) {
	return parseLooseTime(value)
}

func parseLooseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range alertTimeLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func normalizeRegionToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "전체")
	return strings.TrimSpace(value)
}

func regionOverlaps(location, alertRegion string) bool {
	loc := normalizeRegionToken(location)
	if loc == "" {
		return false
	}
	for _, part := range strings.Split(alertRegion, ",") {
		part = normalizeRegionToken(part)
		if part == "" {
			continue
		}
		if strings.Contains(loc, part) || strings.Contains(part, loc) {
			return true
		}
	}
	return false
}

// MatchByLocationAndDate finds alerts whose recipient region overlaps with
// location and whose sent time falls within window of occurredAt. If
// occurredAt cannot be parsed, only the region filter is applied. Results are
// sorted oldest-first, matching timeline order.
func (s *Store) MatchByLocationAndDate(location, occurredAt string, window time.Duration) []Alert {
	target, hasTarget := parseLooseTime(occurredAt)

	var matches []Alert
	for _, a := range s.alerts {
		if !regionOverlaps(location, a.Region) {
			continue
		}
		if hasTarget {
			sentAt, ok := parseLooseTime(a.CreatedAt)
			if ok {
				diff := sentAt.Sub(target)
				if diff < -window || diff > window {
					continue
				}
			}
		}
		matches = append(matches, a)
	}

	sort.Slice(matches, func(i, j int) bool {
		ti, _ := parseLooseTime(matches[i].CreatedAt)
		tj, _ := parseLooseTime(matches[j].CreatedAt)
		return ti.Before(tj)
	})
	return matches
}
