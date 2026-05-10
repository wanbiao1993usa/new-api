package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertBusinessSnapshotUser(t *testing.T, user User) {
	t.Helper()
	require.NoError(t, DB.Create(&user).Error)
}

func insertBusinessSnapshotTopUp(t *testing.T, topUp TopUp) {
	t.Helper()
	require.NoError(t, DB.Create(&topUp).Error)
}

func insertBusinessSnapshotRedemption(t *testing.T, redemption Redemption) {
	t.Helper()
	require.NoError(t, DB.Create(&redemption).Error)
}

func insertBusinessSnapshotLog(t *testing.T, log Log) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&log).Error)
}

func TestGetBusinessSnapshotSummariesAndDailyRows(t *testing.T) {
	truncateTables(t)

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.Local)
	day1 := time.Date(2026, 5, 8, 10, 0, 0, 0, time.Local).Unix()
	day2 := time.Date(2026, 5, 9, 10, 0, 0, 0, time.Local).Unix()
	day3 := time.Date(2026, 5, 10, 10, 0, 0, 0, time.Local).Unix()

	insertBusinessSnapshotUser(t, User{Id: 101, Username: "alice", AffCode: "ba1", Quota: 120, CreatedAt: day1})
	insertBusinessSnapshotUser(t, User{Id: 102, Username: "bob", AffCode: "ba2", Quota: 80, CreatedAt: day2})
	insertBusinessSnapshotUser(t, User{Id: 103, Username: "carol", AffCode: "ba3", Quota: 10, CreatedAt: day2})
	insertBusinessSnapshotUser(t, User{Id: 104, Username: "dave", AffCode: "ba4", Quota: 50, CreatedAt: day3})
	insertBusinessSnapshotUser(t, User{Id: 105, Username: "erin", AffCode: "ba5", Quota: 70, CreatedAt: day3})

	insertBusinessSnapshotTopUp(t, TopUp{
		Id:           1,
		UserId:       101,
		Amount:       500,
		Money:        10,
		TradeNo:      "topup-a",
		Status:       common.TopUpStatusSuccess,
		CompleteTime: day3,
	})
	insertBusinessSnapshotTopUp(t, TopUp{
		Id:           2,
		UserId:       102,
		Amount:       500,
		Money:        20,
		TradeNo:      "topup-b",
		Status:       common.TopUpStatusSuccess,
		CompleteTime: day3,
	})
	insertBusinessSnapshotTopUp(t, TopUp{
		Id:           3,
		UserId:       103,
		Amount:       500,
		Money:        30,
		TradeNo:      "topup-c",
		Status:       common.TopUpStatusSuccess,
		CompleteTime: day3,
	})
	insertBusinessSnapshotRedemption(t, Redemption{
		Id:           1,
		Key:          "snapshot-redemption-1",
		Status:       common.RedemptionCodeStatusUsed,
		Name:         "snapshot quota redemption",
		Quota:        70,
		Type:         RedemptionTypeQuota,
		CreatedTime:  day3,
		RedeemedTime: day3,
		UsedUserId:   105,
	})

	insertBusinessSnapshotLog(t, Log{
		Id:        1,
		UserId:    101,
		Username:  "alice",
		CreatedAt: day2,
		Type:      LogTypeConsume,
	})
	insertBusinessSnapshotLog(t, Log{
		Id:        2,
		UserId:    102,
		Username:  "bob",
		CreatedAt: day3,
		Type:      LogTypeConsume,
	})
	insertBusinessSnapshotLog(t, Log{
		Id:        3,
		UserId:    102,
		Username:  "bob",
		CreatedAt: day3,
		Type:      LogTypeConsume,
	})

	result, err := getBusinessSnapshotAt(now, 3)
	require.NoError(t, err)

	assert.EqualValues(t, 4, result.Summary.TopupPaidUsersCount)
	assert.EqualValues(t, 4, result.Summary.TopupUsersWithBalanceCount)
	assert.EqualValues(t, 280, result.Summary.TopupCurrentBalanceSum)

	require.Len(t, result.Daily, 3)
	assert.Equal(t, "2026-05-08", result.Daily[0].Date)
	assert.EqualValues(t, 0, result.Daily[0].UsedUserCount)
	assert.EqualValues(t, 1, result.Daily[0].NewUserCount)
	assert.EqualValues(t, 1, result.Daily[0].TotalUserCount)
	assert.InDelta(t, 1.0, result.Daily[0].NewUserRate, 0.0001)

	assert.Equal(t, "2026-05-09", result.Daily[1].Date)
	assert.EqualValues(t, 1, result.Daily[1].UsedUserCount)
	assert.EqualValues(t, 2, result.Daily[1].NewUserCount)
	assert.EqualValues(t, 3, result.Daily[1].TotalUserCount)
	assert.InDelta(t, 1.0/3.0, result.Daily[1].UsedUserRate, 0.0001)
	assert.InDelta(t, 2.0/3.0, result.Daily[1].NewUserRate, 0.0001)

	assert.Equal(t, "2026-05-10", result.Daily[2].Date)
	assert.EqualValues(t, 1, result.Daily[2].UsedUserCount)
	assert.EqualValues(t, 2, result.Daily[2].NewUserCount)
	assert.EqualValues(t, 5, result.Daily[2].TotalUserCount)
	assert.InDelta(t, 0.2, result.Daily[2].UsedUserRate, 0.0001)
	assert.InDelta(t, 0.4, result.Daily[2].NewUserRate, 0.0001)
}

func TestGetBusinessSnapshotByRangeDailyRows(t *testing.T) {
	truncateTables(t)

	now := time.Date(2026, 5, 10, 18, 0, 0, 0, time.Local)
	beforeRange := time.Date(2026, 5, 8, 10, 0, 0, 0, time.Local).Unix()
	rangeStart := time.Date(2026, 5, 9, 12, 0, 0, 0, time.Local).Unix()
	rangeEnd := time.Date(2026, 5, 10, 12, 0, 0, 0, time.Local).Unix()
	firstDayBeforeStart := time.Date(2026, 5, 9, 8, 0, 0, 0, time.Local).Unix()
	firstDayWithinRange := time.Date(2026, 5, 9, 13, 0, 0, 0, time.Local).Unix()
	secondDayWithinRange := time.Date(2026, 5, 10, 9, 0, 0, 0, time.Local).Unix()

	insertBusinessSnapshotUser(t, User{Id: 201, Username: "base", AffCode: "bb1", CreatedAt: beforeRange})
	insertBusinessSnapshotUser(t, User{Id: 202, Username: "carry", AffCode: "bb2", CreatedAt: firstDayBeforeStart})
	insertBusinessSnapshotUser(t, User{Id: 203, Username: "new-a", AffCode: "bb3", CreatedAt: firstDayWithinRange})
	insertBusinessSnapshotUser(t, User{Id: 204, Username: "new-b", AffCode: "bb4", CreatedAt: secondDayWithinRange})

	insertBusinessSnapshotLog(t, Log{
		Id:        11,
		UserId:    202,
		Username:  "carry",
		CreatedAt: firstDayWithinRange,
		Type:      LogTypeConsume,
	})
	insertBusinessSnapshotLog(t, Log{
		Id:        12,
		UserId:    203,
		Username:  "new-a",
		CreatedAt: firstDayWithinRange,
		Type:      LogTypeConsume,
	})
	insertBusinessSnapshotLog(t, Log{
		Id:        13,
		UserId:    204,
		Username:  "new-b",
		CreatedAt: secondDayWithinRange,
		Type:      LogTypeConsume,
	})

	result, err := getBusinessSnapshotByRangeAt(now, rangeStart, rangeEnd)
	require.NoError(t, err)

	assert.EqualValues(t, 2, result.Days)
	assert.EqualValues(t, rangeStart, result.StartTimestamp)
	assert.EqualValues(t, rangeEnd, result.EndTimestamp)

	require.Len(t, result.Daily, 2)
	assert.Equal(t, "2026-05-09", result.Daily[0].Date)
	assert.EqualValues(t, 2, result.Daily[0].UsedUserCount)
	assert.EqualValues(t, 1, result.Daily[0].NewUserCount)
	assert.EqualValues(t, 3, result.Daily[0].TotalUserCount)
	assert.InDelta(t, 2.0/3.0, result.Daily[0].UsedUserRate, 0.0001)
	assert.InDelta(t, 1.0/3.0, result.Daily[0].NewUserRate, 0.0001)

	assert.Equal(t, "2026-05-10", result.Daily[1].Date)
	assert.EqualValues(t, 1, result.Daily[1].UsedUserCount)
	assert.EqualValues(t, 1, result.Daily[1].NewUserCount)
	assert.EqualValues(t, 4, result.Daily[1].TotalUserCount)
	assert.InDelta(t, 0.25, result.Daily[1].UsedUserRate, 0.0001)
	assert.InDelta(t, 0.25, result.Daily[1].NewUserRate, 0.0001)
}
