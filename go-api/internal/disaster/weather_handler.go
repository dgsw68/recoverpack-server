package disaster

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type weatherAlertResponseData struct {
	Region          string         `json:"region"`
	Date            string         `json:"date"`
	HasWeatherAlert bool           `json:"hasWeatherAlert"`
	Alerts          []WeatherAlert `json:"alerts"`
}

// WeatherAlertsHandler implements "공식 재난상황 근거 자동 연결": given a
// region and date, it asks the 기상청_기상특보 조회서비스 whether an official
// heavy-rain/typhoon/heavy-snow etc. advisory was in effect, so the result
// can be cited as evidence in the damage timeline/PDF. It never fails the
// request outward — on any lookup problem it returns an empty alert list.
func WeatherAlertsHandler(client *WeatherClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		region := c.Query("region")
		date := c.Query("date")
		if region == "" || date == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"data":    nil,
				"error":   "region and date query parameters are required",
			})
			return
		}

		alerts := client.FetchAlerts(c.Request.Context(), region, date)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": weatherAlertResponseData{
				Region:          region,
				Date:            date,
				HasWeatherAlert: len(alerts) > 0,
				Alerts:          alerts,
			},
			"error": nil,
		})
	}
}
