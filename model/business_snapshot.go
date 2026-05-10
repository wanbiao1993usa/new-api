package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type BusinessSnapshotSummary struct {
	TopupPaidUsersCount        int64 `json:"topup_paid_users_count"`
	TopupUsersWithBalanceCount int64 `json:"topup_users_with_balance_count"`
	TopupCurrentBalanceSum     int64 `json:"topup_current_balance_sum"`
}

type BusinessSnapshotDailyRow struct {
	Date           string  `json:"date"`
	UsedUserCount  int64   `json:"used_user_count"`
	NewUserCount   int64   `json:"new_user_count"`
	TotalUserCount int64   `json:"total_user_count"`
	UsedUserRate   float64 `json:"used_user_rate"`
	NewUserRate    float64 `json:"new_user_rate"`
}

type BusinessSnapshotResult struct {
	Summary        BusinessSnapshotSummary    `json:"summary"`
	Daily          []BusinessSnapshotDailyRow `json:"daily"`
	Days           int                        `json:"days"`
	GeneratedAt    int64                      `json:"generated_at"`
	StartTimestamp int64                      `json:"start_timestamp"`
	EndTimestamp   int64                      `json:"end_timestamp"`
}

type BusinessSnapshotPaidUserRow struct {
	Id              int    `json:"id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	Group           string `json:"group"`
	Quota           int    `json:"quota"`
	GrantedQuota    int64  `json:"granted_quota"`
	CreatedAt       int64  `json:"created_at"`
	LastLoginAt     int64  `json:"last_login_at"`
	TopupOrderCount int64  `json:"topup_order_count"`
	RedeemedCount   int64  `json:"redeemed_count"`
}

type businessSnapshotDayCountRow struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type businessSnapshotUserCountRow struct {
	UserId int `gorm:"column:user_id"`
	Count  int `gorm:"column:count"`
}

type businessSnapshotUserQuotaSumRow struct {
	UserId int   `gorm:"column:user_id"`
	Quota  int64 `gorm:"column:quota"`
}

func GetBusinessSnapshot(days int) (BusinessSnapshotResult, error) {
	return getBusinessSnapshotAt(time.Now(), days)
}

func GetBusinessSnapshotByRange(startTimestamp int64, endTimestamp int64) (BusinessSnapshotResult, error) {
	return getBusinessSnapshotByRangeAt(time.Now(), startTimestamp, endTimestamp)
}

func getBusinessSnapshotAt(now time.Time, days int) (BusinessSnapshotResult, error) {
	var result BusinessSnapshotResult
	days = normalizeBusinessSnapshotDays(days)

	location := time.Local
	now = now.In(location)
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -(days - 1))
	endDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, location)
	startTimestamp := startDay.Unix()
	endTimestamp := endDay.Unix()

	summary, err := getBusinessSnapshotSummary()
	if err != nil {
		return result, err
	}
	daily, err := getBusinessSnapshotDailyRows(startDay, startTimestamp, endTimestamp, days)
	if err != nil {
		return result, err
	}

	result.Summary = summary
	result.Daily = daily
	result.Days = days
	result.GeneratedAt = now.Unix()
	result.StartTimestamp = startTimestamp
	result.EndTimestamp = endTimestamp
	return result, nil
}

func getBusinessSnapshotByRangeAt(now time.Time, startTimestamp int64, endTimestamp int64) (BusinessSnapshotResult, error) {
	var result BusinessSnapshotResult
	location := time.Local
	now = now.In(location)

	if startTimestamp <= 0 && endTimestamp <= 0 {
		return getBusinessSnapshotAt(now, 30)
	}
	if endTimestamp <= 0 {
		endTimestamp = now.Unix()
	}
	if startTimestamp <= 0 {
		startTimestamp = time.Unix(endTimestamp, 0).In(location).AddDate(0, 0, -29).Unix()
	}
	if endTimestamp < startTimestamp {
		startTimestamp, endTimestamp = endTimestamp, startTimestamp
	}

	startTime := time.Unix(startTimestamp, 0).In(location)
	endTime := time.Unix(endTimestamp, 0).In(location)
	startDay := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, location)
	endDay := time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 0, location)
	dayCount := int(endDay.Sub(startDay).Hours()/24) + 1
	if dayCount <= 0 {
		dayCount = 1
	}

	summary, err := getBusinessSnapshotSummary()
	if err != nil {
		return result, err
	}
	daily, err := getBusinessSnapshotDailyRows(startDay, startTimestamp, endTimestamp, dayCount)
	if err != nil {
		return result, err
	}

	result.Summary = summary
	result.Daily = daily
	result.Days = dayCount
	result.GeneratedAt = now.Unix()
	result.StartTimestamp = startTimestamp
	result.EndTimestamp = endTimestamp
	return result, nil
}

func normalizeBusinessSnapshotDays(days int) int {
	if days <= 0 {
		return 30
	}
	if days > 90 {
		return 90
	}
	return days
}

func getBusinessSnapshotPaidUserIDs() ([]int, error) {
	paidUserSet := make(map[int]struct{})

	topupUserIDs := make([]int, 0)
	if err := DB.Model(&TopUp{}).
		Where("status = ? AND amount > 0", common.TopUpStatusSuccess).
		Distinct("user_id").
		Pluck("user_id", &topupUserIDs).Error; err != nil {
		return nil, err
	}
	for _, userId := range topupUserIDs {
		if userId > 0 {
			paidUserSet[userId] = struct{}{}
		}
	}

	redemptionUserIDs := make([]int, 0)
	if err := DB.Model(&Redemption{}).
		Where(
			"status = ? AND type = ? AND used_user_id > 0",
			common.RedemptionCodeStatusUsed,
			RedemptionTypeQuota,
		).
		Distinct("used_user_id").
		Pluck("used_user_id", &redemptionUserIDs).Error; err != nil {
		return nil, err
	}
	for _, userId := range redemptionUserIDs {
		if userId > 0 {
			paidUserSet[userId] = struct{}{}
		}
	}

	paidUserIDs := make([]int, 0, len(paidUserSet))
	for userId := range paidUserSet {
		paidUserIDs = append(paidUserIDs, userId)
	}
	return paidUserIDs, nil
}

func getBusinessSnapshotSummary() (BusinessSnapshotSummary, error) {
	var summary BusinessSnapshotSummary

	paidUserIDs, err := getBusinessSnapshotPaidUserIDs()
	if err != nil {
		common.SysError("failed to query paid user ids for business snapshot: " + err.Error())
		return summary, errors.New("查询业务快照失败")
	}
	if len(paidUserIDs) > 0 {
		summary.TopupPaidUsersCount = int64(len(paidUserIDs))

		var users []User
		if err := DB.Select("id", "quota").
			Where("id IN ? AND quota > 0", paidUserIDs).
			Find(&users).Error; err != nil {
			common.SysError("failed to query topup users for business snapshot: " + err.Error())
			return summary, errors.New("查询业务快照失败")
		}

		for _, user := range users {
			summary.TopupUsersWithBalanceCount++
			summary.TopupCurrentBalanceSum += int64(user.Quota)
		}
	}

	return summary, nil
}

func GetBusinessSnapshotUsersWithBalance(pageInfo *common.PageInfo) (*common.PageInfo, error) {
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}

	paidUserIDs, err := getBusinessSnapshotPaidUserIDs()
	if err != nil {
		common.SysError("failed to query paid user ids for business snapshot users: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}

	pageInfo.SetItems([]BusinessSnapshotPaidUserRow{})
	pageInfo.SetTotal(0)
	if len(paidUserIDs) == 0 {
		return pageInfo, nil
	}

	var total int64
	if err := DB.Model(&User{}).
		Where("id IN ? AND quota > 0", paidUserIDs).
		Count(&total).Error; err != nil {
		common.SysError("failed to count paid users with balance: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}

	pageInfo.SetTotal(int(total))
	if total == 0 {
		return pageInfo, nil
	}

	var users []User
	if err := DB.Select("id", "username", "email", "group", "quota", "created_at", "last_login_at").
		Where("id IN ? AND quota > 0", paidUserIDs).
		Order("quota desc, last_login_at desc, id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&users).Error; err != nil {
		common.SysError("failed to query paid users with balance: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}

	if len(users) == 0 {
		return pageInfo, nil
	}

	userIDs := make([]int, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.Id)
	}

	topupCounts := make(map[int]int)
	topupRows := make([]businessSnapshotUserCountRow, 0)
	if err := DB.Model(&TopUp{}).
		Select("user_id, COUNT(*) AS count").
		Where("user_id IN ? AND status = ? AND amount > 0", userIDs, common.TopUpStatusSuccess).
		Group("user_id").
		Scan(&topupRows).Error; err != nil {
		common.SysError("failed to query topup counts for business snapshot users: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}
	for _, row := range topupRows {
		topupCounts[row.UserId] = row.Count
	}

	redemptionCounts := make(map[int]int)
	redemptionRows := make([]businessSnapshotUserCountRow, 0)
	if err := DB.Model(&Redemption{}).
		Select("used_user_id AS user_id, COUNT(*) AS count").
		Where(
			"used_user_id IN ? AND status = ? AND type = ?",
			userIDs,
			common.RedemptionCodeStatusUsed,
			RedemptionTypeQuota,
		).
		Group("used_user_id").
		Scan(&redemptionRows).Error; err != nil {
		common.SysError("failed to query redemption counts for business snapshot users: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}
	for _, row := range redemptionRows {
		redemptionCounts[row.UserId] = row.Count
	}

	grantedQuotaByUser := make(map[int]int64)
	topupQuotaRows := make([]businessSnapshotUserQuotaSumRow, 0)
	if err := DB.Model(&TopUp{}).
		Select(
			"user_id, SUM(CASE WHEN payment_provider = ? THEN CAST(money * ? AS INTEGER) ELSE CAST(amount * ? AS INTEGER) END) AS quota",
			PaymentProviderStripe,
			common.QuotaPerUnit,
			common.QuotaPerUnit,
		).
		Where("user_id IN ? AND status = ? AND amount > 0", userIDs, common.TopUpStatusSuccess).
		Group("user_id").
		Scan(&topupQuotaRows).Error; err != nil {
		common.SysError("failed to query topup quota sum for business snapshot users: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}
	for _, row := range topupQuotaRows {
		grantedQuotaByUser[row.UserId] += row.Quota
	}

	redemptionQuotaRows := make([]businessSnapshotUserQuotaSumRow, 0)
	if err := DB.Model(&Redemption{}).
		Select("used_user_id AS user_id, SUM(quota) AS quota").
		Where(
			"used_user_id IN ? AND status = ? AND type = ?",
			userIDs,
			common.RedemptionCodeStatusUsed,
			RedemptionTypeQuota,
		).
		Group("used_user_id").
		Scan(&redemptionQuotaRows).Error; err != nil {
		common.SysError("failed to query redemption quota sum for business snapshot users: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}
	for _, row := range redemptionQuotaRows {
		grantedQuotaByUser[row.UserId] += row.Quota
	}

	items := make([]BusinessSnapshotPaidUserRow, 0, len(users))
	for _, user := range users {
		items = append(items, BusinessSnapshotPaidUserRow{
			Id:              user.Id,
			Username:        user.Username,
			Email:           user.Email,
			Group:           user.Group,
			Quota:           user.Quota,
			GrantedQuota:    grantedQuotaByUser[user.Id],
			CreatedAt:       user.CreatedAt,
			LastLoginAt:     user.LastLoginAt,
			TopupOrderCount: int64(topupCounts[user.Id]),
			RedeemedCount:   int64(redemptionCounts[user.Id]),
		})
	}

	pageInfo.SetItems(items)
	return pageInfo, nil
}

func getBusinessSnapshotDailyRows(startDay time.Time, startTimestamp int64, endTimestamp int64, days int) ([]BusinessSnapshotDailyRow, error) {
	dayExpr := getBusinessSnapshotDayExpr("created_at")

	newUserRows := make([]businessSnapshotDayCountRow, 0)
	if err := DB.Model(&User{}).
		Select(dayExpr+" AS day, COUNT(*) AS count").
		Where("created_at >= ? AND created_at <= ?", startTimestamp, endTimestamp).
		Group(dayExpr).
		Order(dayExpr).
		Scan(&newUserRows).Error; err != nil {
		common.SysError("failed to query new users for business snapshot: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}

	usedUserRows := make([]businessSnapshotDayCountRow, 0)
	if err := LOG_DB.Model(&Log{}).
		Select(dayExpr+" AS day, COUNT(DISTINCT user_id) AS count").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, startTimestamp, endTimestamp).
		Group(dayExpr).
		Order(dayExpr).
		Scan(&usedUserRows).Error; err != nil {
		common.SysError("failed to query used users for business snapshot: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}

	var baseUserCount int64
	if err := DB.Model(&User{}).Where("created_at < ?", startTimestamp).Count(&baseUserCount).Error; err != nil {
		common.SysError("failed to query base user count for business snapshot: " + err.Error())
		return nil, errors.New("查询业务快照失败")
	}

	newUserByDay := make(map[string]int64, len(newUserRows))
	for _, row := range newUserRows {
		newUserByDay[row.Day] = row.Count
	}
	usedUserByDay := make(map[string]int64, len(usedUserRows))
	for _, row := range usedUserRows {
		usedUserByDay[row.Day] = row.Count
	}

	rows := make([]BusinessSnapshotDailyRow, 0, days)
	totalUserCount := baseUserCount
	for i := 0; i < days; i++ {
		day := startDay.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		newUserCount := newUserByDay[key]
		usedUserCount := usedUserByDay[key]
		totalUserCount += newUserCount

		row := BusinessSnapshotDailyRow{
			Date:           key,
			UsedUserCount:  usedUserCount,
			NewUserCount:   newUserCount,
			TotalUserCount: totalUserCount,
		}
		if totalUserCount > 0 {
			row.UsedUserRate = float64(usedUserCount) / float64(totalUserCount)
			row.NewUserRate = float64(newUserCount) / float64(totalUserCount)
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func getBusinessSnapshotDayExpr(column string) string {
	switch {
	case common.UsingSQLite:
		return "strftime('%Y-%m-%d', " + column + ", 'unixepoch', 'localtime')"
	case common.UsingPostgreSQL:
		return "TO_CHAR(TO_TIMESTAMP(" + column + "), 'YYYY-MM-DD')"
	default:
		return "DATE(FROM_UNIXTIME(" + column + "))"
	}
}
