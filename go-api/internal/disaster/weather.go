// Weather alert integration: a second 공식 재난상황 근거 source alongside the
// 긴급재난문자 CSV in store.go. Kept in its own file/type (WeatherClient) so
// additional public-data sources can be added later without touching the
// CSV-backed Store.
package disaster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WeatherAlert mirrors one 특보 entry returned by the 기상청_기상특보 조회서비스.
type WeatherAlert struct {
	Title       string `json:"title"`
	Area        string `json:"area"`
	AlertType   string `json:"alertType"`
	Level       string `json:"level"`
	AnnouncedAt string `json:"announcedAt"`
	EffectiveAt string `json:"effectiveAt"`
	Content     string `json:"content"`
	Source      string `json:"source"`
}

const weatherAlertSource = "기상청_기상특보 조회서비스"

// regionStationCodes maps a handful of major-city region names to their KMA
// synoptic station ID (지점번호). MVP coverage only; extend as needed.
var regionStationCodes = map[string]string{
	"서울": "108",
	"인천": "112",
	"대전": "133",
	"대구": "143",
	"울산": "152",
	"광주": "156",
	"부산": "159",
	"제주": "184",
}

// StationCodeForRegion returns the KMA station ID for a free-text region
// string (e.g. "대구광역시 중구" matches "대구"), and whether one was found.
func StationCodeForRegion(region string) (string, bool) {
	region = strings.TrimSpace(region)
	for name, code := range regionStationCodes {
		if strings.Contains(region, name) {
			return code, true
		}
	}
	return "", false
}

// WeatherClient calls the 공공데이터포털 기상청_기상특보 조회서비스 (WthrWrnInfoService).
type WeatherClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewWeatherClient builds a client from KMA_SPECIAL_WEATHER_API_KEY /
// KMA_API_BASE_URL. An empty apiKey is allowed: FetchAlerts will then return
// an empty result instead of failing, so the rest of the service keeps
// working while the key is pending.
func NewWeatherClient(apiKey, baseURL string) *WeatherClient {
	if baseURL == "" {
		baseURL = "https://apis.data.go.kr/1360000/WthrWrnInfoService"
	}
	return &WeatherClient{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Enabled reports whether an API key has been configured.
func (c *WeatherClient) Enabled() bool {
	return c.apiKey != ""
}

type wthrWrnListResponse struct {
	Response struct {
		Header struct {
			ResultCode string `json:"resultCode"`
			ResultMsg  string `json:"resultMsg"`
		} `json:"header"`
		Body struct {
			Items struct {
				Item []wthrWrnItem `json:"item"`
			} `json:"items"`
		} `json:"body"`
	} `json:"response"`
}

// wthrWrnItem mirrors one row of the getWthrWrnList operation. Field names
// follow the 공공데이터포털 spec: t1-t7 are the free-text 특보내용 lines,
// cmd is 발표(1)/연장(2)/변경(3)/해제(4), title is e.g. "호우주의보".
type wthrWrnItem struct {
	StnId string `json:"stnId"`
	TmFc  string `json:"tmFc"` // 발표시각, yyyyMMddHHmm
	TmSeq string `json:"tmSeq"`
	Cmd   string `json:"cmd"`
	Title string `json:"title"`
	T1    string `json:"t1"`
	T2    string `json:"t2"`
	T3    string `json:"t3"`
	T4    string `json:"t4"`
	T5    string `json:"t5"`
	T6    string `json:"t6"`
	T7    string `json:"t7"`
}

// FetchAlerts returns official 기상특보 entries announced on the given date
// (YYYY-MM-DD) for the given free-text region. On any failure (missing key,
// network error, unexpected response, unmapped region) it returns an empty,
// non-nil slice and a nil error so callers can treat "no evidence found" and
// "lookup failed" the same way and never block the rest of the request.
func (c *WeatherClient) FetchAlerts(ctx context.Context, region, date string) []WeatherAlert {
	alerts, err := c.fetchAlerts(ctx, region, date)
	if err != nil {
		log.Printf("[WEATHER] lookup failed for region=%q date=%q: %v", region, date, err)
		return []WeatherAlert{}
	}
	return alerts
}

func (c *WeatherClient) fetchAlerts(ctx context.Context, region, date string) ([]WeatherAlert, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("KMA_SPECIAL_WEATHER_API_KEY not configured")
	}
	day, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", date, err)
	}
	stnId, ok := StationCodeForRegion(region)
	if !ok {
		return nil, fmt.Errorf("no KMA station mapping for region %q", region)
	}

	q := url.Values{}
	q.Set("serviceKey", c.apiKey)
	q.Set("pageNo", "1")
	q.Set("numOfRows", "100")
	q.Set("dataType", "JSON")
	q.Set("stnId", stnId)
	q.Set("fromTmFc", day.Format("20060102")+"0000")
	q.Set("toTmFc", day.Format("20060102")+"2359")

	reqURL := c.baseURL + "/getWthrWrnList?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("KMA API returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed wthrWrnListResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode KMA response: %w (body: %.200s)", err, string(body))
	}
	if parsed.Response.Header.ResultCode != "" && parsed.Response.Header.ResultCode != "00" {
		return nil, fmt.Errorf("KMA API error %s: %s", parsed.Response.Header.ResultCode, parsed.Response.Header.ResultMsg)
	}

	alerts := make([]WeatherAlert, 0, len(parsed.Response.Body.Items.Item))
	for _, item := range parsed.Response.Body.Items.Item {
		announcedAt := formatKMATime(item.TmFc)
		alertType, level := splitAlertTitle(item.Title)
		content := joinNonEmpty(item.T1, item.T2, item.T3, item.T4, item.T5, item.T6, item.T7)
		if content == "" {
			content = item.Title
		}
		alerts = append(alerts, WeatherAlert{
			Title:       item.Title,
			Area:        region,
			AlertType:   alertType,
			Level:       level,
			AnnouncedAt: announcedAt,
			EffectiveAt: announcedAt,
			Content:     content,
			Source:      weatherAlertSource,
		})
	}
	return alerts, nil
}

// splitAlertTitle splits a title like "호우주의보" into ("호우", "주의보") or
// "태풍경보" into ("태풍", "경보"). Falls back to (title, "") if no known
// level suffix is found.
func splitAlertTitle(title string) (alertType, level string) {
	for _, suffix := range []string{"경보", "주의보"} {
		if strings.HasSuffix(title, suffix) {
			return strings.TrimSuffix(title, suffix), suffix
		}
	}
	return title, ""
}

func joinNonEmpty(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, " ")
}

// formatKMATime converts a KMA "yyyyMMddHHmm" timestamp to ISO-ish
// "YYYY-MM-DDTHH:MM:SS". Returns the raw value if it doesn't parse.
func formatKMATime(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 12 {
		return value
	}
	year, err1 := strconv.Atoi(value[0:4])
	month, err2 := strconv.Atoi(value[4:6])
	dayNum, err3 := strconv.Atoi(value[6:8])
	hour, err4 := strconv.Atoi(value[8:10])
	minute, err5 := strconv.Atoi(value[10:12])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil {
		return value
	}
	t := time.Date(year, time.Month(month), dayNum, hour, minute, 0, 0, time.Local)
	return t.Format("2006-01-02T15:04:05")
}
