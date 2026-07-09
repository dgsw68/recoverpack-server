package disaster

import (
	"context"
	"testing"
)

func TestStationCodeForRegion(t *testing.T) {
	code, ok := StationCodeForRegion("대구광역시 중구")
	if !ok || code != "143" {
		t.Fatalf("expected 대구 -> 143, got %q ok=%v", code, ok)
	}
	if _, ok := StationCodeForRegion("전라남도 순천시"); ok {
		t.Fatal("expected no station mapping for unmapped region")
	}
}

func TestSplitAlertTitle(t *testing.T) {
	cases := map[string][2]string{
		"호우주의보": {"호우", "주의보"},
		"태풍경보":  {"태풍", "경보"},
		"이상제목":  {"이상제목", ""},
	}
	for title, want := range cases {
		gotType, gotLevel := splitAlertTitle(title)
		if gotType != want[0] || gotLevel != want[1] {
			t.Errorf("splitAlertTitle(%q) = (%q, %q), want (%q, %q)", title, gotType, gotLevel, want[0], want[1])
		}
	}
}

func TestWeatherClientWithoutKeyReturnsEmpty(t *testing.T) {
	client := NewWeatherClient("", "")
	if client.Enabled() {
		t.Fatal("expected client to be disabled without an API key")
	}
	alerts := client.FetchAlerts(context.Background(), "대구", "2026-07-09")
	if alerts == nil || len(alerts) != 0 {
		t.Fatalf("expected empty non-nil slice, got %#v", alerts)
	}
}

func TestWeatherClientUnmappedRegionReturnsEmpty(t *testing.T) {
	client := NewWeatherClient("dummy-key", "")
	alerts := client.FetchAlerts(context.Background(), "존재하지않는지역", "2026-07-09")
	if alerts == nil || len(alerts) != 0 {
		t.Fatalf("expected empty non-nil slice for unmapped region, got %#v", alerts)
	}
}
