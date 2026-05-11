/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package model

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type SubscriptionAnalysisFilters struct {
	StartTimestamp int64
	EndTimestamp   int64
	Username       string
}

type SubscriptionAnalysisSummary struct {
	PlanCount                        int   `json:"plan_count"`
	ActiveUserCount                  int64 `json:"active_user_count"`
	ActiveSubscriptionCount          int64 `json:"active_subscription_count"`
	HistoricalUsedTotal              int64 `json:"historical_used_total"`
	CurrentUsedTotal                 int64 `json:"current_used_total"`
	CurrentRemainingTotal            int64 `json:"current_remaining_total"`
	UnlimitedActiveSubscriptionCount int64 `json:"unlimited_active_subscription_count"`
}

type SubscriptionAnalysisPlanRow struct {
	PlanId                           int    `json:"plan_id"`
	Title                            string `json:"title"`
	Subtitle                         string `json:"subtitle"`
	UserCount                        int64  `json:"user_count"`
	ActiveUserCount                  int64  `json:"active_user_count"`
	SubscriptionCount                int64  `json:"subscription_count"`
	ActiveSubscriptionCount          int64  `json:"active_subscription_count"`
	HistoricalUsedTotal              int64  `json:"historical_used_total"`
	CurrentUsedTotal                 int64  `json:"current_used_total"`
	CurrentRemainingTotal            int64  `json:"current_remaining_total"`
	UnlimitedActiveSubscriptionCount int64  `json:"unlimited_active_subscription_count"`
}

type SubscriptionAnalysisResult struct {
	Summary SubscriptionAnalysisSummary   `json:"summary"`
	Plans   []SubscriptionAnalysisPlanRow `json:"plans"`
}

func GetSubscriptionAnalysis(filters SubscriptionAnalysisFilters) (SubscriptionAnalysisResult, error) {
	var result SubscriptionAnalysisResult

	rows, activeUserCount, err := buildSubscriptionAnalysisPlanRows(filters)
	if err != nil {
		return result, err
	}

	result.Summary.ActiveUserCount = activeUserCount
	for _, row := range rows {
		result.Summary.ActiveSubscriptionCount += row.ActiveSubscriptionCount
		result.Summary.HistoricalUsedTotal += row.HistoricalUsedTotal
		result.Summary.CurrentUsedTotal += row.CurrentUsedTotal
		result.Summary.CurrentRemainingTotal += row.CurrentRemainingTotal
		result.Summary.UnlimitedActiveSubscriptionCount += row.UnlimitedActiveSubscriptionCount
	}
	result.Summary.PlanCount = len(rows)
	result.Plans = rows
	return result, nil
}

func buildSubscriptionAnalysisPlanRows(filters SubscriptionAnalysisFilters) ([]SubscriptionAnalysisPlanRow, int64, error) {
	username := strings.TrimSpace(filters.Username)
	userID, err := findSubscriptionAnalysisUserID(username)
	if err != nil {
		return nil, 0, err
	}
	if username != "" && userID == 0 {
		return []SubscriptionAnalysisPlanRow{}, 0, nil
	}

	rowMap := make(map[int]*SubscriptionAnalysisPlanRow)
	activeUserSets := make(map[int]map[int]struct{})
	userSets := make(map[int]map[int]struct{})
	globalActiveUserSet := make(map[int]struct{})

	now := common.GetTimestamp()
	query := DB.Model(&UserSubscription{})
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	var subs []UserSubscription
	if err := query.Order("id desc").Find(&subs).Error; err != nil {
		common.SysError("failed to query subscription analysis subscriptions: " + err.Error())
		return nil, 0, errors.New("查询订阅分析数据失败")
	}

	for _, sub := range subs {
		row, err := ensureSubscriptionAnalysisPlanRow(rowMap, sub.PlanId)
		if err != nil {
			return nil, 0, err
		}
		row.SubscriptionCount++
		if sub.UserId > 0 {
			if userSets[sub.PlanId] == nil {
				userSets[sub.PlanId] = make(map[int]struct{})
			}
			userSets[sub.PlanId][sub.UserId] = struct{}{}
		}

		if !isSubscriptionAnalysisActive(sub, now) {
			continue
		}

		row.ActiveSubscriptionCount++
		row.CurrentUsedTotal += sub.AmountUsed
		if sub.AmountTotal > 0 {
			remaining := sub.AmountTotal - sub.AmountUsed
			if remaining < 0 {
				remaining = 0
			}
			row.CurrentRemainingTotal += remaining
		} else {
			row.UnlimitedActiveSubscriptionCount++
		}

		if sub.UserId > 0 {
			if activeUserSets[sub.PlanId] == nil {
				activeUserSets[sub.PlanId] = make(map[int]struct{})
			}
			activeUserSets[sub.PlanId][sub.UserId] = struct{}{}
			globalActiveUserSet[sub.UserId] = struct{}{}
		}
	}

	historicalByPlan, err := getSubscriptionAnalysisHistoricalUsageByPlan(filters, userID)
	if err != nil {
		return nil, 0, err
	}
	for planID, used := range historicalByPlan {
		row, err := ensureSubscriptionAnalysisPlanRow(rowMap, planID)
		if err != nil {
			return nil, 0, err
		}
		row.HistoricalUsedTotal = used
	}

	rows := make([]SubscriptionAnalysisPlanRow, 0, len(rowMap))
	for planID, row := range rowMap {
		row.UserCount = int64(len(userSets[planID]))
		row.ActiveUserCount = int64(len(activeUserSets[planID]))
		rows = append(rows, *row)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].HistoricalUsedTotal != rows[j].HistoricalUsedTotal {
			return rows[i].HistoricalUsedTotal > rows[j].HistoricalUsedTotal
		}
		if rows[i].CurrentUsedTotal != rows[j].CurrentUsedTotal {
			return rows[i].CurrentUsedTotal > rows[j].CurrentUsedTotal
		}
		if rows[i].ActiveUserCount != rows[j].ActiveUserCount {
			return rows[i].ActiveUserCount > rows[j].ActiveUserCount
		}
		return rows[i].PlanId < rows[j].PlanId
	})
	return rows, int64(len(globalActiveUserSet)), nil
}

func ensureSubscriptionAnalysisPlanRow(rowMap map[int]*SubscriptionAnalysisPlanRow, planID int) (*SubscriptionAnalysisPlanRow, error) {
	if row, ok := rowMap[planID]; ok {
		return row, nil
	}

	row := &SubscriptionAnalysisPlanRow{
		PlanId: planID,
		Title:  "-",
	}
	if planID > 0 {
		plan, err := GetSubscriptionPlanById(planID)
		if err == nil && plan != nil {
			row.Title = plan.Title
			row.Subtitle = plan.Subtitle
		}
	}
	rowMap[planID] = row
	return row, nil
}

func isSubscriptionAnalysisActive(sub UserSubscription, now int64) bool {
	if sub.Status != "active" {
		return false
	}
	if sub.EndTime > 0 && sub.EndTime <= now {
		return false
	}
	return true
}

func findSubscriptionAnalysisUserID(username string) (int, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return 0, nil
	}
	var user User
	if err := DB.Select("id").Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		common.SysError("failed to query subscription analysis user: " + err.Error())
		return 0, errors.New("查询订阅分析数据失败")
	}
	return user.Id, nil
}

func getSubscriptionAnalysisHistoricalUsageByPlan(filters SubscriptionAnalysisFilters, userID int) (map[int]int64, error) {
	historicalByPlan := make(map[int]int64)

	tx := LOG_DB.Model(&Log{}).
		Select("id", "user_id", "quota", "other", "type", "created_at").
		Where("type IN ?", []int{LogTypeConsume, LogTypeRefund})

	if userID > 0 {
		tx = tx.Where("user_id = ?", userID)
	} else if filters.Username != "" {
		tx = tx.Where("username = ?", filters.Username)
	}
	if filters.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", filters.StartTimestamp)
	}
	if filters.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", filters.EndTimestamp)
	}

	rows, err := tx.Rows()
	if err != nil {
		common.SysError("failed to query subscription analysis logs: " + err.Error())
		return nil, errors.New("查询订阅分析数据失败")
	}
	defer rows.Close()

	for rows.Next() {
		var log Log
		if err := LOG_DB.ScanRows(rows, &log); err != nil {
			common.SysError("failed to scan subscription analysis log: " + err.Error())
			return nil, errors.New("查询订阅分析数据失败")
		}
		if log.Other == "" {
			continue
		}

		var other subscriptionBillingLogOther
		if err := common.UnmarshalJsonStr(log.Other, &other); err != nil {
			continue
		}
		if other.BillingSource != "subscription" || other.SubscriptionPlanId <= 0 {
			continue
		}

		consumed := other.SubscriptionConsumed
		if consumed <= 0 && log.Quota > 0 {
			consumed = int64(log.Quota)
		}
		if consumed <= 0 {
			continue
		}
		if log.Type == LogTypeRefund {
			consumed = -consumed
		}
		historicalByPlan[other.SubscriptionPlanId] += consumed
	}

	if err := rows.Err(); err != nil {
		common.SysError("failed to iterate subscription analysis logs: " + err.Error())
		return nil, errors.New("查询订阅分析数据失败")
	}
	return historicalByPlan, nil
}
