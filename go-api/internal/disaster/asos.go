// ASOS (지상 종관기상관측) hourly observation integration: unlike the
// forecast-only 단기예보 service, this returns actual recorded weather, so it
// has no "recent days only" limitation and can support evidence for damage
// reported long after the fact. Reuses the same KMA API key and
// regionStationCodes lookup as weather.go.
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

// AsosObservation mirrors one hourly row returned by the
// 기상청_지상(종관, ASOS) 시간자료 조회서비스.
type AsosObservation struct {
	Time            string  `json:"time"`
	StationName     string  `json:"stationName"`
	TemperatureC    float64 `json:"temperatureC"`
	PrecipitationMM float64 `json:"precipitationMm"`
	WindSpeedMs     float64 `json:"windSpeedMs"`
	HumidityPct     float64 `json:"humidityPct"`
}

const asosObservationSource = "기상청_지상(종관,ASOS) 시간자료 조회서비스"

// AsosClient calls the 공공데이터포털 기상청_지상(종관, ASOS) 시간자료
// 조회서비스 (AsosHourlyInfoService).
type AsosClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewAsosClient builds a client from KMA_SPECIAL_WEATHER_API_KEY (shared
// general auth key across KMA services) / KMA_ASOS_API_BASE_URL. An empty
// apiKey is allowed: FetchObservations then returns an empty result instead
// of failing, so the rest of the service is unaffected while the key is
// pending.
func NewAsosClient(apiKey, baseURL string) *AsosClient {
	if baseURL == "" {
		baseURL = "https://apis.data.go.kr/1360000/AsosHourlyInfoService"
	}
	return &AsosClient{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Enabled reports whether an API key has been configured.
func (c *AsosClient) Enabled() bool {
	return c.apiKey != ""
}

type asosListResponse struct {
	Response struct {
		Header struct {
			ResultCode string `json:"resultCode"`
			ResultMsg  string `json:"resultMsg"`
		} `json:"header"`
		Body struct {
			Items struct {
				Item []asosItem `json:"item"`
			} `json:"items"`
		} `json:"body"`
	} `json:"response"`
}

// asosItem mirrors one row of the getWthrDataList operation. Field names
// follow the 공공데이터포털 spec: tm is 관측시각(yyyy-MM-dd HH:mm), ta is
// 기온(°C), rn is 시간강수량(mm), ws is 풍속(m/s), hm is 습도(%).
type asosItem struct {
	Tm    string `json:"tm"`
	StnNm string `json:"stnNm"`
	Ta    string `json:"ta"`
	Rn    string `json:"rn"`
	Ws    string `json:"ws"`
	Hm    string `json:"hm"`
}

// FetchObservations returns hourly ASOS observations for the given date
// (YYYY-MM-DD) at the station nearest the given free-text region. On any
// failure (missing key, network error, unexpected response, unmapped
// region) it returns an empty, non-nil slice and logs the cause, so callers
// can treat "no data found" and "lookup failed" the same way and never
// block the rest of the request.
func (c *AsosClient) FetchObservations(ctx context.Context, region, date string) []AsosObservation {
	observations, err := c.fetchObservations(ctx, region, date)
	if err != nil {
		log.Printf("[ASOS] lookup failed for region=%q date=%q: %v", region, date, err)
		return []AsosObservation{}
	}
	return observations
}

func (c *AsosClient) fetchObservations(ctx context.Context, region, date string) ([]AsosObservation, error) {
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
	q.Set("numOfRows", "24")
	q.Set("dataType", "JSON")
	q.Set("dataCd", "ASOS")
	q.Set("dateCd", "HR")
	q.Set("startDt", day.Format("20060102"))
	q.Set("startHh", "00")
	q.Set("endDt", day.Format("20060102"))
	q.Set("endHh", "23")
	q.Set("stnIds", stnId)

	reqURL := c.baseURL + "/getWthrDataList?" + q.Encode()
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

	var parsed asosListResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode KMA response: %w (body: %.200s)", err, string(body))
	}
	if parsed.Response.Header.ResultCode != "" && parsed.Response.Header.ResultCode != "00" {
		return nil, fmt.Errorf("KMA API error %s: %s", parsed.Response.Header.ResultCode, parsed.Response.Header.ResultMsg)
	}

	observations := make([]AsosObservation, 0, len(parsed.Response.Body.Items.Item))
	for _, item := range parsed.Response.Body.Items.Item {
		observations = append(observations, AsosObservation{
			Time:            item.Tm,
			StationName:     item.StnNm,
			TemperatureC:    parseFloatOrZero(item.Ta),
			PrecipitationMM: parseFloatOrZero(item.Rn),
			WindSpeedMs:     parseFloatOrZero(item.Ws),
			HumidityPct:     parseFloatOrZero(item.Hm),
		})
	}
	return observations, nil
}

func parseFloatOrZero(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return f
}
