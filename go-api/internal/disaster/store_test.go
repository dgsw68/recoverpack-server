package disaster

import (
	"testing"
	"time"
)

const testCSVPath = "../../data/disaster_alerts.csv"

func TestLoadStoreSkipsKoreanHeaderRow(t *testing.T) {
	store, err := LoadStore(testCSVPath)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if store.Len() == 0 {
		t.Fatal("expected at least one alert to be loaded")
	}
	for _, a := range store.alerts {
		if a.SN == "일련번호" {
			t.Fatalf("Korean header row was loaded as data: %+v", a)
		}
	}
}

func TestMatchByLocationAndDate(t *testing.T) {
	store, err := LoadStore(testCSVPath)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	matches := store.MatchByLocationAndDate("경상남도 김해시", "2023-09-16", 72*time.Hour)
	if len(matches) == 0 {
		t.Fatal("expected matches for 김해시 on 2023-09-16, got none")
	}
	for _, a := range matches {
		if !regionOverlaps("경상남도 김해시", a.Region) {
			t.Errorf("matched alert region %q does not overlap 경상남도 김해시", a.Region)
		}
	}

	// Results must be sorted oldest-first.
	for i := 1; i < len(matches); i++ {
		prev, ok1 := ParseAlertTime(matches[i-1].CreatedAt)
		curr, ok2 := ParseAlertTime(matches[i].CreatedAt)
		if ok1 && ok2 && curr.Before(prev) {
			t.Errorf("matches not sorted chronologically at index %d", i)
		}
	}

	none := store.MatchByLocationAndDate("경상남도 김해시", "2023-01-01", 72*time.Hour)
	if len(none) != 0 {
		t.Errorf("expected no matches far outside the time window, got %d", len(none))
	}
}
