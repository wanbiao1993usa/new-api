package model

import (
	"errors"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

type BillingAnalysisFilters struct {
	StartTimestamp int64
	EndTimestamp   int64
	Username       string
	UserId         int
	TokenName      string
	ModelName      string
	Channel        int
	Group          string
}

type BillingAnalysisSummary struct {
	TotalQuota                int64   `json:"total_quota"`
	WalletQuota               int64   `json:"wallet_quota"`
	SubscriptionQuota         int64   `json:"subscription_quota"`
	TokenCount                int64   `json:"token_count"`
	RequestCount              int64   `json:"request_count"`
	EffectiveQuotaPer1KTokens float64 `json:"effective_quota_per_1k_tokens"`
}

type BillingAnalysisRow struct {
	Key                       string  `json:"key"`
	Name                      string  `json:"name"`
	UserId                    int     `json:"user_id,omitempty"`
	ChannelId                 int     `json:"channel_id,omitempty"`
	TotalQuota                int64   `json:"total_quota"`
	WalletQuota               int64   `json:"wallet_quota"`
	SubscriptionQuota         int64   `json:"subscription_quota"`
	TokenCount                int64   `json:"token_count"`
	RequestCount              int64   `json:"request_count"`
	EffectiveQuotaPer1KTokens float64 `json:"effective_quota_per_1k_tokens"`
	LastUsedAt                int64   `json:"last_used_at"`
}

type BillingAnalysisResult struct {
	Summary  BillingAnalysisSummary `json:"summary"`
	Users    []BillingAnalysisRow   `json:"users,omitempty"`
	Tokens   []BillingAnalysisRow   `json:"tokens"`
	Models   []BillingAnalysisRow   `json:"models"`
	Channels []BillingAnalysisRow   `json:"channels,omitempty"`
	Groups   []BillingAnalysisRow   `json:"groups"`
}

type billingAnalysisLogOther struct {
	BillingSource        string `json:"billing_source"`
	SubscriptionConsumed int64  `json:"subscription_consumed"`
}

func GetBillingAnalysis(filters BillingAnalysisFilters, includeAdminDimensions bool) (BillingAnalysisResult, error) {
	var result BillingAnalysisResult

	tx := LOG_DB.Model(&Log{}).
		Where("type = ?", LogTypeConsume)

	if filters.UserId > 0 {
		tx = tx.Where("user_id = ?", filters.UserId)
	}
	if filters.Username != "" {
		tx = tx.Where("username = ?", filters.Username)
	}
	if filters.TokenName != "" {
		tx = tx.Where("token_name = ?", filters.TokenName)
	}
	if filters.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", filters.StartTimestamp)
	}
	if filters.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", filters.EndTimestamp)
	}
	if filters.ModelName != "" {
		modelNamePattern, err := sanitizeLikePattern(filters.ModelName)
		if err != nil {
			return result, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if filters.Channel != 0 {
		tx = tx.Where("channel_id = ?", filters.Channel)
	}
	if filters.Group != "" {
		tx = tx.Where(logGroupCol+" = ?", filters.Group)
	}

	rows, err := tx.Rows()
	if err != nil {
		common.SysError("failed to query billing analysis logs: " + err.Error())
		return result, errors.New("查询计费分析数据失败")
	}
	defer rows.Close()

	userRows := make(map[string]*BillingAnalysisRow)
	tokenRows := make(map[string]*BillingAnalysisRow)
	modelRows := make(map[string]*BillingAnalysisRow)
	channelRows := make(map[string]*BillingAnalysisRow)
	groupRows := make(map[string]*BillingAnalysisRow)

	for rows.Next() {
		var log Log
		if err := LOG_DB.ScanRows(rows, &log); err != nil {
			common.SysError("failed to scan billing analysis log: " + err.Error())
			return result, errors.New("查询计费分析数据失败")
		}
		walletQuota, subscriptionQuota := splitBillingAnalysisQuota(log)
		tokenCount := int64(log.PromptTokens + log.CompletionTokens)
		totalQuota := walletQuota + subscriptionQuota

		addBillingAnalysisSummary(&result.Summary, totalQuota, walletQuota, subscriptionQuota, tokenCount)
		addBillingAnalysisRow(tokenRows, log.TokenName, log.TokenName, 0, 0, totalQuota, walletQuota, subscriptionQuota, tokenCount, log.CreatedAt)
		addBillingAnalysisRow(modelRows, log.ModelName, log.ModelName, 0, 0, totalQuota, walletQuota, subscriptionQuota, tokenCount, log.CreatedAt)
		addBillingAnalysisRow(groupRows, log.Group, log.Group, 0, 0, totalQuota, walletQuota, subscriptionQuota, tokenCount, log.CreatedAt)

		if includeAdminDimensions {
			userKey := strconv.Itoa(log.UserId)
			userName := log.Username
			if userName == "" {
				userName = userKey
			}
			addBillingAnalysisRow(userRows, userKey, userName, log.UserId, 0, totalQuota, walletQuota, subscriptionQuota, tokenCount, log.CreatedAt)

			channelKey := strconv.Itoa(log.ChannelId)
			addBillingAnalysisRow(channelRows, channelKey, channelKey, 0, log.ChannelId, totalQuota, walletQuota, subscriptionQuota, tokenCount, log.CreatedAt)
		}
	}
	if err := rows.Err(); err != nil {
		common.SysError("failed to iterate billing analysis logs: " + err.Error())
		return result, errors.New("查询计费分析数据失败")
	}

	fillBillingAnalysisEffectiveQuota(&result.Summary)
	if includeAdminDimensions {
		result.Users = finishBillingAnalysisRows(userRows)
		result.Channels = finishBillingAnalysisRows(channelRows)
	}
	result.Tokens = finishBillingAnalysisRows(tokenRows)
	result.Models = finishBillingAnalysisRows(modelRows)
	result.Groups = finishBillingAnalysisRows(groupRows)

	return result, nil
}

func splitBillingAnalysisQuota(log Log) (walletQuota int64, subscriptionQuota int64) {
	var other billingAnalysisLogOther
	if log.Other != "" {
		_ = common.UnmarshalJsonStr(log.Other, &other)
	}
	if other.BillingSource == "subscription" {
		subscriptionQuota = other.SubscriptionConsumed
		if subscriptionQuota <= 0 {
			subscriptionQuota = int64(log.Quota)
		}
		return 0, subscriptionQuota
	}
	return int64(log.Quota), 0
}

func addBillingAnalysisSummary(summary *BillingAnalysisSummary, totalQuota int64, walletQuota int64, subscriptionQuota int64, tokenCount int64) {
	summary.TotalQuota += totalQuota
	summary.WalletQuota += walletQuota
	summary.SubscriptionQuota += subscriptionQuota
	summary.TokenCount += tokenCount
	summary.RequestCount += 1
}

func addBillingAnalysisRow(rows map[string]*BillingAnalysisRow, key string, name string, userId int, channelId int, totalQuota int64, walletQuota int64, subscriptionQuota int64, tokenCount int64, lastUsedAt int64) {
	if key == "" {
		key = "-"
	}
	if name == "" {
		name = "-"
	}
	row, ok := rows[key]
	if !ok {
		row = &BillingAnalysisRow{
			Key:       key,
			Name:      name,
			UserId:    userId,
			ChannelId: channelId,
		}
		rows[key] = row
	}
	row.TotalQuota += totalQuota
	row.WalletQuota += walletQuota
	row.SubscriptionQuota += subscriptionQuota
	row.TokenCount += tokenCount
	row.RequestCount += 1
	if lastUsedAt > row.LastUsedAt {
		row.LastUsedAt = lastUsedAt
	}
}

func finishBillingAnalysisRows(rowMap map[string]*BillingAnalysisRow) []BillingAnalysisRow {
	rows := make([]BillingAnalysisRow, 0, len(rowMap))
	for _, row := range rowMap {
		fillBillingAnalysisEffectiveQuota(row)
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalQuota != rows[j].TotalQuota {
			return rows[i].TotalQuota > rows[j].TotalQuota
		}
		if rows[i].RequestCount != rows[j].RequestCount {
			return rows[i].RequestCount > rows[j].RequestCount
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func fillBillingAnalysisEffectiveQuota(target interface{}) {
	switch v := target.(type) {
	case *BillingAnalysisSummary:
		if v.TokenCount > 0 {
			v.EffectiveQuotaPer1KTokens = float64(v.TotalQuota) * 1000 / float64(v.TokenCount)
		}
	case *BillingAnalysisRow:
		if v.TokenCount > 0 {
			v.EffectiveQuotaPer1KTokens = float64(v.TotalQuota) * 1000 / float64(v.TokenCount)
		}
	}
}
