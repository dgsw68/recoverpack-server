package disaster

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListAlertsHandler serves the in-memory 긴급재난문자 alerts, filterable by
// region and disaster type, as a stand-in until the public data API approval
// (data.go.kr) comes through.
func ListAlertsHandler(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		offset, _ := strconv.Atoi(c.Query("offset"))
		limit, _ := strconv.Atoi(c.Query("limit"))

		alerts, total := store.List(Query{
			Region:       c.Query("region"),
			DisasterType: c.Query("type"),
			Offset:       offset,
			Limit:        limit,
		})

		c.JSON(http.StatusOK, gin.H{
			"alerts": alerts,
			"total":  total,
			"offset": offset,
			"count":  len(alerts),
			"source": "행정안전부_긴급재난문자 (local CSV import, pending data.go.kr API approval)",
		})
	}
}
