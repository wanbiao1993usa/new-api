package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetBusinessSnapshot(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if startTimestamp > 0 || endTimestamp > 0 {
		result, err := model.GetBusinessSnapshotByRange(startTimestamp, endTimestamp)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, result)
		return
	}

	days, _ := strconv.Atoi(c.Query("days"))
	result, err := model.GetBusinessSnapshot(days)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}
