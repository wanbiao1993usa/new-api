package model

import (
	"errors"
	"fmt"
	"math"
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
	TotalQuota                     int64                         `json:"total_quota"`
	WalletQuota                    int64                         `json:"wallet_quota"`
	WalletMultiplierOverview       []BillingAnalysisOverviewItem `json:"wallet_multiplier_overview"`
	SubscriptionQuota              int64                         `json:"subscription_quota"`
	SubscriptionMultiplierOverview []BillingAnalysisOverviewItem `json:"subscription_multiplier_overview"`
	TokenCount                     int64                         `json:"token_count"`
	RequestCount                   int64                         `json:"request_count"`
	EffectiveQuotaPer1KTokens      float64                       `json:"effective_quota_per_1k_tokens"`
}

type BillingAnalysisOverviewItem struct {
	Key           string  `json:"key"`
	Label         string  `json:"label"`
	Quota         int64   `json:"quota"`
	OriginalQuota float64 `json:"original_quota"`
	RequestCount  int64   `json:"request_count"`
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
	BillingSource        string   `json:"billing_source"`
	SubscriptionConsumed int64    `json:"subscription_consumed"`
	BillingMode          string   `json:"billing_mode"`
	MatchedTier          string   `json:"matched_tier"`
	ModelRatio           *float64 `json:"model_ratio"`
	GroupRatio           *float64 `json:"group_ratio"`
	UserGroupRatio       *float64 `json:"user_group_ratio"`
	ModelPrice           *float64 `json:"model_price"`
}

type billingAnalysisOverviewMeta struct {
	Key            string
	Label          string
	EffectiveRatio float64
}

type billingAnalysisGroupRatioMeta struct {
	Key   string
	Label string
	Ratio float64
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
	walletOverviewRows := make(map[string]*BillingAnalysisOverviewItem)
	subscriptionOverviewRows := make(map[string]*BillingAnalysisOverviewItem)

	for rows.Next() {
		var log Log
		if err := LOG_DB.ScanRows(rows, &log); err != nil {
			common.SysError("failed to scan billing analysis log: " + err.Error())
			return result, errors.New("查询计费分析数据失败")
		}
		other := parseBillingAnalysisLogOther(log)
		walletQuota, subscriptionQuota := splitBillingAnalysisQuotaWithOther(log, other)
		tokenCount := int64(log.PromptTokens + log.CompletionTokens)
		totalQuota := walletQuota + subscriptionQuota

		addBillingAnalysisSummary(&result.Summary, totalQuota, walletQuota, subscriptionQuota, tokenCount)
		addBillingAnalysisRow(tokenRows, log.TokenName, log.TokenName, 0, 0, totalQuota, walletQuota, subscriptionQuota, tokenCount, log.CreatedAt)
		addBillingAnalysisRow(modelRows, log.ModelName, log.ModelName, 0, 0, totalQuota, walletQuota, subscriptionQuota, tokenCount, log.CreatedAt)
		addBillingAnalysisRow(groupRows, log.Group, log.Group, 0, 0, totalQuota, walletQuota, subscriptionQuota, tokenCount, log.CreatedAt)
		addBillingAnalysisOverviewItem(walletOverviewRows, other, walletQuota)
		addBillingAnalysisOverviewItem(subscriptionOverviewRows, other, subscriptionQuota)

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
	result.Summary.WalletMultiplierOverview = finishBillingAnalysisOverviewItems(walletOverviewRows)
	result.Summary.SubscriptionMultiplierOverview = finishBillingAnalysisOverviewItems(subscriptionOverviewRows)
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
	other := parseBillingAnalysisLogOther(log)
	return splitBillingAnalysisQuotaWithOther(log, other)
}

func parseBillingAnalysisLogOther(log Log) billingAnalysisLogOther {
	var other billingAnalysisLogOther
	if log.Other != "" {
		_ = common.UnmarshalJsonStr(log.Other, &other)
	}
	return other
}

func splitBillingAnalysisQuotaWithOther(log Log, other billingAnalysisLogOther) (walletQuota int64, subscriptionQuota int64) {
	if other.BillingSource == "subscription" {
		subscriptionQuota = other.SubscriptionConsumed
		if subscriptionQuota <= 0 {
			subscriptionQuota = int64(log.Quota)
		}
		return 0, subscriptionQuota
	}
	return int64(log.Quota), 0
}

func addBillingAnalysisOverviewItem(items map[string]*BillingAnalysisOverviewItem, other billingAnalysisLogOther, quota int64) {
	if quota <= 0 {
		return
	}
	meta := getBillingAnalysisOverviewMeta(other)
	item, ok := items[meta.Key]
	if !ok {
		item = &BillingAnalysisOverviewItem{
			Key:   meta.Key,
			Label: meta.Label,
		}
		items[meta.Key] = item
	}
	item.Quota += quota
	if meta.EffectiveRatio > 0 {
		item.OriginalQuota += float64(quota) / meta.EffectiveRatio
	}
	item.RequestCount += 1
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

func finishBillingAnalysisOverviewItems(itemMap map[string]*BillingAnalysisOverviewItem) []BillingAnalysisOverviewItem {
	items := make([]BillingAnalysisOverviewItem, 0, len(itemMap))
	for _, item := range itemMap {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Quota != items[j].Quota {
			return items[i].Quota > items[j].Quota
		}
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		return items[i].Label < items[j].Label
	})
	return items
}

func getBillingAnalysisOverviewMeta(other billingAnalysisLogOther) billingAnalysisOverviewMeta {
	groupMeta := getBillingAnalysisGroupRatioMeta(other)

	if other.BillingMode == "tiered_expr" {
		key := "tiered:" + groupMeta.Key
		label := "阶梯计费"
		if other.MatchedTier != "" {
			key += ":" + other.MatchedTier
			label += " / " + other.MatchedTier
		}
		return billingAnalysisOverviewMeta{
			Key:            key,
			Label:          label + " / " + groupMeta.Label,
			EffectiveRatio: groupMeta.Ratio,
		}
	}

	if other.ModelPrice != nil {
		return billingAnalysisOverviewMeta{
			Key:            "fixed:" + groupMeta.Key,
			Label:          "固定价格 / " + groupMeta.Label,
			EffectiveRatio: groupMeta.Ratio,
		}
	}

	if other.ModelRatio != nil {
		return billingAnalysisOverviewMeta{
			Key:            groupMeta.Key,
			Label:          groupMeta.Label,
			EffectiveRatio: groupMeta.Ratio,
		}
	}

	return billingAnalysisOverviewMeta{
		Key:            "other:" + groupMeta.Key,
		Label:          groupMeta.Label,
		EffectiveRatio: groupMeta.Ratio,
	}
}

func getBillingAnalysisGroupRatioMeta(other billingAnalysisLogOther) billingAnalysisGroupRatioMeta {
	if other.UserGroupRatio != nil && *other.UserGroupRatio != -1 {
		ratio := roundBillingAnalysisRatio(*other.UserGroupRatio)
		return billingAnalysisGroupRatioMeta{
			Key:   "group:" + formatBillingAnalysisRatioKey(ratio),
			Label: "分组倍率 " + formatBillingAnalysisRatioLabel(ratio),
			Ratio: ratio,
		}
	}
	if other.GroupRatio != nil {
		ratio := roundBillingAnalysisRatio(*other.GroupRatio)
		return billingAnalysisGroupRatioMeta{
			Key:   "group:" + formatBillingAnalysisRatioKey(ratio),
			Label: "分组倍率 " + formatBillingAnalysisRatioLabel(ratio),
			Ratio: ratio,
		}
	}
	return billingAnalysisGroupRatioMeta{
		Key:   "group:" + formatBillingAnalysisRatioKey(1),
		Label: "分组倍率 " + formatBillingAnalysisRatioLabel(1),
		Ratio: 1,
	}
}

func roundBillingAnalysisRatio(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}

func formatBillingAnalysisRatioKey(value float64) string {
	return fmt.Sprintf("%.6f", roundBillingAnalysisRatio(value))
}

func formatBillingAnalysisRatioLabel(value float64) string {
	return strconv.FormatFloat(roundBillingAnalysisRatio(value), 'f', -1, 64) + "x"
}

func fillBillingAnalysisEffectiveQuota(target interface{}) {
	const quotaPerMillionTokens = 1000000
	switch v := target.(type) {
	case *BillingAnalysisSummary:
		if v.TokenCount > 0 {
			v.EffectiveQuotaPer1KTokens = float64(v.TotalQuota) * quotaPerMillionTokens / float64(v.TokenCount)
		}
	case *BillingAnalysisRow:
		if v.TokenCount > 0 {
			v.EffectiveQuotaPer1KTokens = float64(v.TotalQuota) * quotaPerMillionTokens / float64(v.TokenCount)
		}
	}
}
