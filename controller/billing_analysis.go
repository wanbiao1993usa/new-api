package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetBillingAnalysis(c *gin.Context) {
	filters := getBillingAnalysisFilters(c)
	result, err := model.GetBillingAnalysis(filters, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetBillingAnalysisSelf(c *gin.Context) {
	filters := getBillingAnalysisFilters(c)
	filters.UserId = c.GetInt("id")
	filters.Username = ""
	result, err := model.GetBillingAnalysis(filters, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func getBillingAnalysisFilters(c *gin.Context) model.BillingAnalysisFilters {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel, _ := strconv.Atoi(c.Query("channel"))

	return model.BillingAnalysisFilters{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Username:       c.Query("username"),
		TokenName:      c.Query("token_name"),
		ModelName:      c.Query("model_name"),
		Channel:        channel,
		Group:          c.Query("group"),
	}
}
