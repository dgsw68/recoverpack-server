package disaster

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type asosResponseData struct {
	Region          string            `json:"region"`
	Date            string            `json:"date"`
	TotalRainMm     float64           `json:"totalRainMm"`
	HasObservations bool              `json:"hasObservations"`
	Observations    []AsosObservation `json:"observations"`
}

// AsosObservationsHandler implements "실측 기상 근거 자동 연결": given a
// region and date, it asks the 기상청_지상(종관,ASOS) 시간자료 조회서비스 for
// the hourly weather actually recorded, so the result can be cited as
// evidence in the damage timeline/PDF. Unlike forecast APIs, this has no
// recency limit. It never fails the request outward - on any lookup problem
// it returns an empty observation list.
func AsosObservationsHandler(client *AsosClient) gin.HandlerFunc {
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

		observations := client.FetchObservations(c.Request.Context(), region, date)

		var totalRain float64
		for _, o := range observations {
			totalRain += o.PrecipitationMM
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": asosResponseData{
				Region:          region,
				Date:            date,
				TotalRainMm:     totalRain,
				HasObservations: len(observations) > 0,
				Observations:    observations,
			},
			"error": nil,
		})
	}
}
