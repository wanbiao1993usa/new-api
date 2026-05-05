package model

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// QuotaData 柱状图数据
type QuotaData struct {
	Id        int    `json:"id"`
	UserID    int    `json:"user_id" gorm:"index"`
	Username  string `json:"username" gorm:"index:idx_qdt_model_user_name,priority:2;size:64;default:''"`
	ModelName string `json:"model_name" gorm:"index:idx_qdt_model_user_name,priority:1;size:64;default:''"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index:idx_qdt_created_at,priority:2"`
	TokenUsed int    `json:"token_used" gorm:"default:0"`
	Count     int    `json:"count" gorm:"default:0"`
	Quota     int    `json:"quota" gorm:"default:0"`
}

type dashboardQuotaGroupMode int

const (
	dashboardQuotaGroupByModel dashboardQuotaGroupMode = iota
	dashboardQuotaGroupByUser
)

type dashboardQuotaDataFilters struct {
	StartTime   int64
	EndTime     int64
	UserID      int
	Username    string
	DefaultTime string
	GroupMode   dashboardQuotaGroupMode
}

func UpdateQuotaData() {
	for {
		if common.DataExportEnabled {
			common.SysLog("正在更新数据看板数据...")
			SaveQuotaDataCache()
		}
		time.Sleep(time.Duration(common.DataExportInterval) * time.Minute)
	}
}

var CacheQuotaData = make(map[string]*QuotaData)
var CacheQuotaDataLock = sync.Mutex{}

func logQuotaDataCache(userId int, username string, modelName string, quota int, createdAt int64, tokenUsed int) {
	key := fmt.Sprintf("%d-%s-%s-%d", userId, username, modelName, createdAt)
	quotaData, ok := CacheQuotaData[key]
	if ok {
		quotaData.Count += 1
		quotaData.Quota += quota
		quotaData.TokenUsed += tokenUsed
	} else {
		quotaData = &QuotaData{
			UserID:    userId,
			Username:  username,
			ModelName: modelName,
			CreatedAt: createdAt,
			Count:     1,
			Quota:     quota,
			TokenUsed: tokenUsed,
		}
	}
	CacheQuotaData[key] = quotaData
}

func LogQuotaData(userId int, username string, modelName string, quota int, createdAt int64, tokenUsed int) {
	// 只精确到小时
	createdAt = createdAt - (createdAt % 3600)

	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	logQuotaDataCache(userId, username, modelName, quota, createdAt, tokenUsed)
}

func SaveQuotaDataCache() {
	CacheQuotaDataLock.Lock()
	defer CacheQuotaDataLock.Unlock()
	size := len(CacheQuotaData)
	// 如果缓存中有数据，就保存到数据库中
	// 1. 先查询数据库中是否有数据
	// 2. 如果有数据，就更新数据
	// 3. 如果没有数据，就插入数据
	for _, quotaData := range CacheQuotaData {
		quotaDataDB := &QuotaData{}
		DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and created_at = ?",
			quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.CreatedAt).First(quotaDataDB)
		if quotaDataDB.Id > 0 {
			//quotaDataDB.Count += quotaData.Count
			//quotaDataDB.Quota += quotaData.Quota
			//DB.Table("quota_data").Save(quotaDataDB)
			increaseQuotaData(quotaData.UserID, quotaData.Username, quotaData.ModelName, quotaData.Count, quotaData.Quota, quotaData.CreatedAt, quotaData.TokenUsed)
		} else {
			DB.Table("quota_data").Create(quotaData)
		}
	}
	CacheQuotaData = make(map[string]*QuotaData)
	common.SysLog(fmt.Sprintf("保存数据看板数据成功，共保存%d条数据", size))
}

func increaseQuotaData(userId int, username string, modelName string, count int, quota int, createdAt int64, tokenUsed int) {
	err := DB.Table("quota_data").Where("user_id = ? and username = ? and model_name = ? and created_at = ?",
		userId, username, modelName, createdAt).Updates(map[string]interface{}{
		"count":      gorm.Expr("count + ?", count),
		"quota":      gorm.Expr("quota + ?", quota),
		"token_used": gorm.Expr("token_used + ?", tokenUsed),
	}).Error
	if err != nil {
		common.SysLog(fmt.Sprintf("increaseQuotaData error: %s", err))
	}
}

func GetQuotaDataByUsername(username string, startTime int64, endTime int64, defaultTime ...string) (quotaData []*QuotaData, err error) {
	return getDashboardQuotaDataFromLogs(dashboardQuotaDataFilters{
		StartTime:   startTime,
		EndTime:     endTime,
		Username:    username,
		DefaultTime: firstDashboardDefaultTime(defaultTime),
		GroupMode:   dashboardQuotaGroupByModel,
	})
}

func GetQuotaDataByUserId(userId int, startTime int64, endTime int64, defaultTime ...string) (quotaData []*QuotaData, err error) {
	return getDashboardQuotaDataFromLogs(dashboardQuotaDataFilters{
		StartTime:   startTime,
		EndTime:     endTime,
		UserID:      userId,
		DefaultTime: firstDashboardDefaultTime(defaultTime),
		GroupMode:   dashboardQuotaGroupByModel,
	})
}

func GetQuotaDataGroupByUser(startTime int64, endTime int64, defaultTime ...string) (quotaData []*QuotaData, err error) {
	return getDashboardQuotaDataFromLogs(dashboardQuotaDataFilters{
		StartTime:   startTime,
		EndTime:     endTime,
		DefaultTime: firstDashboardDefaultTime(defaultTime),
		GroupMode:   dashboardQuotaGroupByUser,
	})
}

func GetAllQuotaDates(startTime int64, endTime int64, username string, defaultTime ...string) (quotaData []*QuotaData, err error) {
	if username != "" {
		return GetQuotaDataByUsername(username, startTime, endTime, defaultTime...)
	}
	return getDashboardQuotaDataFromLogs(dashboardQuotaDataFilters{
		StartTime:   startTime,
		EndTime:     endTime,
		DefaultTime: firstDashboardDefaultTime(defaultTime),
		GroupMode:   dashboardQuotaGroupByModel,
	})
}

func firstDashboardDefaultTime(defaultTime []string) string {
	if len(defaultTime) == 0 || defaultTime[0] == "" {
		return common.DataExportDefaultTime
	}
	return defaultTime[0]
}

func dashboardBucketSize(defaultTime string) int64 {
	switch defaultTime {
	case "week":
		return 604800
	case "day":
		return 86400
	default:
		return 3600
	}
}

func dashboardBucketTimestamp(createdAt int64, defaultTime string) int64 {
	bucketSize := dashboardBucketSize(defaultTime)
	if bucketSize <= 0 {
		return createdAt
	}
	return createdAt - (createdAt % bucketSize)
}

func getDashboardQuotaDataFromLogs(filters dashboardQuotaDataFilters) ([]*QuotaData, error) {
	tx := LOG_DB.Model(&Log{}).
		Select([]string{"user_id", "username", "model_name", "created_at", "quota", "prompt_tokens", "completion_tokens", "other"}).
		Where("type = ?", LogTypeConsume)

	if filters.UserID > 0 {
		tx = tx.Where("user_id = ?", filters.UserID)
	}
	if filters.Username != "" {
		tx = tx.Where("username = ?", filters.Username)
	}
	if filters.StartTime != 0 {
		tx = tx.Where("created_at >= ?", filters.StartTime)
	}
	if filters.EndTime != 0 {
		tx = tx.Where("created_at <= ?", filters.EndTime)
	}

	rows, err := tx.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := make(map[string]*QuotaData)
	for rows.Next() {
		var log Log
		if err := LOG_DB.ScanRows(rows, &log); err != nil {
			return nil, err
		}
		bucketAt := dashboardBucketTimestamp(log.CreatedAt, filters.DefaultTime)
		keyName := log.ModelName
		if filters.GroupMode == dashboardQuotaGroupByUser {
			keyName = log.Username
		}
		key := fmt.Sprintf("%s-%d", keyName, bucketAt)
		row, ok := grouped[key]
		if !ok {
			row = &QuotaData{
				UserID:    log.UserId,
				Username:  log.Username,
				ModelName: log.ModelName,
				CreatedAt: bucketAt,
			}
			if filters.GroupMode == dashboardQuotaGroupByUser {
				row.ModelName = ""
			}
			grouped[key] = row
		}

		walletQuota, subscriptionQuota := splitBillingAnalysisQuota(log)
		row.Count += 1
		row.Quota += int(walletQuota + subscriptionQuota)
		row.TokenUsed += log.PromptTokens + log.CompletionTokens
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	quotaData := make([]*QuotaData, 0, len(grouped))
	for _, row := range grouped {
		quotaData = append(quotaData, row)
	}
	sort.Slice(quotaData, func(i, j int) bool {
		if quotaData[i].CreatedAt != quotaData[j].CreatedAt {
			return quotaData[i].CreatedAt < quotaData[j].CreatedAt
		}
		if filters.GroupMode == dashboardQuotaGroupByUser {
			return quotaData[i].Username < quotaData[j].Username
		}
		return quotaData[i].ModelName < quotaData[j].ModelName
	})
	return quotaData, nil
}
