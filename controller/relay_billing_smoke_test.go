package controller_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const relaySmokeModel = "gpt-5-mini"

type relaySmokeEnv struct {
	db     *gorm.DB
	router *gin.Engine
}

type relaySmokeOptions struct {
	enableRedis       bool
	enableMemoryCache bool
}

func TestRelayBillingSmokeSubscriptionOnlyUsesSubscription(t *testing.T) {
	upstream, hits := newOpenAISmokeUpstream(t)
	env := setupRelayBillingSmokeEnv(t)

	user, token := createRelaySmokeUser(t, env.db, "sub-only", "subsmoke", "wallet_first", 100_000)
	_, sub := createRelaySmokeSubscription(t, env.db, user.Id, relaySmokeModel, 10_000)
	createRelaySmokeChannel(t, "sub-only", upstream.URL)

	recorder := performRelaySmokeRequest(t, env.router, token.Key, relaySmokeModel)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(1), hits.Load())

	var updatedUser model.User
	require.NoError(t, env.db.First(&updatedUser, user.Id).Error)
	require.Equal(t, user.Quota, updatedUser.Quota, "subscription-only group must not deduct wallet quota")

	var updatedSub model.UserSubscription
	require.NoError(t, env.db.First(&updatedSub, sub.Id).Error)
	require.Greater(t, updatedSub.AmountUsed, int64(0))

	var modelUsage model.UserSubscriptionModelUsage
	require.NoError(t, env.db.Where("user_subscription_id = ? AND model_name = ?", sub.Id, relaySmokeModel).First(&modelUsage).Error)
	require.Equal(t, updatedSub.AmountUsed, modelUsage.AmountUsed)

	var preConsume model.SubscriptionPreConsumeRecord
	require.NoError(t, env.db.Where("user_id = ?", user.Id).First(&preConsume).Error)
	require.Equal(t, "consumed", preConsume.Status)
	require.Equal(t, relaySmokeModel, preConsume.ModelName)
	require.Greater(t, preConsume.PreConsumed, int64(0))

	log, other := loadRelaySmokeConsumeLog(t, env.db, user.Id)
	require.Equal(t, "sub-only", log.Group)
	require.Equal(t, service.BillingSourceSubscription, other["billing_source"])
	require.Equal(t, "sub-only", other["billing_group"])
	require.Equal(t, ratio_setting.GroupBillingTypeSubscriptionOnly, other["billing_group_type"])
	require.Equal(t, relaySmokeModel, other["subscription_model_name"])
	require.Equal(t, float64(0), other["wallet_quota_deducted"])
}

func TestRelayBillingSmokeWalletOnlyUsesWalletEvenWithActiveSubscription(t *testing.T) {
	upstream, hits := newOpenAISmokeUpstream(t)
	env := setupRelayBillingSmokeEnv(t)

	user, token := createRelaySmokeUser(t, env.db, "wallet-only", "walletsmoke", "subscription_first", 100_000)
	_, sub := createRelaySmokeSubscription(t, env.db, user.Id, relaySmokeModel, 10_000)
	createRelaySmokeChannel(t, "wallet-only", upstream.URL)

	recorder := performRelaySmokeRequest(t, env.router, token.Key, relaySmokeModel)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(1), hits.Load())

	var updatedUser model.User
	require.NoError(t, env.db.First(&updatedUser, user.Id).Error)
	require.Less(t, updatedUser.Quota, user.Quota, "wallet-only group must deduct wallet quota")

	var updatedSub model.UserSubscription
	require.NoError(t, env.db.First(&updatedSub, sub.Id).Error)
	require.Equal(t, int64(0), updatedSub.AmountUsed)

	var modelUsageCount int64
	require.NoError(t, env.db.Model(&model.UserSubscriptionModelUsage{}).Where("user_subscription_id = ?", sub.Id).Count(&modelUsageCount).Error)
	require.Zero(t, modelUsageCount)

	var preConsumeCount int64
	require.NoError(t, env.db.Model(&model.SubscriptionPreConsumeRecord{}).Where("user_id = ?", user.Id).Count(&preConsumeCount).Error)
	require.Zero(t, preConsumeCount)

	log, other := loadRelaySmokeConsumeLog(t, env.db, user.Id)
	require.Equal(t, "wallet-only", log.Group)
	require.Equal(t, service.BillingSourceWallet, other["billing_source"])
	require.Equal(t, "wallet-only", other["billing_group"])
	require.Equal(t, ratio_setting.GroupBillingTypeWalletOnly, other["billing_group_type"])
	_, hasSubscriptionModel := other["subscription_model_name"]
	require.False(t, hasSubscriptionModel)
}

func TestRelaySmokeUpstreamInsufficientBalanceFallsBackToNextChannel(t *testing.T) {
	oldRetryTimes := common.RetryTimes
	oldErrorLogEnabled := constant.ErrorLogEnabled
	t.Cleanup(func() {
		common.RetryTimes = oldRetryTimes
		constant.ErrorLogEnabled = oldErrorLogEnabled
	})
	common.RetryTimes = 1
	constant.ErrorLogEnabled = true

	failingUpstream, failingHits := newOpenAIInsufficientBalanceSmokeUpstream(t)
	successUpstream, successHits := newOpenAISmokeUpstream(t)
	env := setupRelayBillingSmokeEnv(t)

	user, token := createRelaySmokeUser(t, env.db, "wallet-only", "fallbacksmoke", "subscription_first", 100_000)
	failingChannel := createRelaySmokeChannelWithPriority(t, "wallet-only", failingUpstream.URL, relaySmokeModel, 10)
	successChannel := createRelaySmokeChannelWithPriority(t, "wallet-only", successUpstream.URL, relaySmokeModel, 0)

	recorder := performRelaySmokeRequest(t, env.router, token.Key, relaySmokeModel)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(1), failingHits.Load())
	require.Equal(t, int32(1), successHits.Load())
	require.NotContains(t, recorder.Body.String(), "AiMaMi")
	require.NotContains(t, recorder.Body.String(), "insufficient balance")
	require.NotContains(t, recorder.Body.String(), "127.0.0.1")

	var errLog model.Log
	require.NoError(t, env.db.Where("user_id = ? AND type = ?", user.Id, model.LogTypeError).Order("id desc").First(&errLog).Error)
	require.Equal(t, failingChannel.Id, errLog.ChannelId)
	require.Contains(t, errLog.Content, types.UpstreamInsufficientBalanceInternalMessage)
	require.NotContains(t, errLog.Content, "AiMaMi")
	require.NotContains(t, errLog.Content, "insufficient balance")
	require.NotContains(t, errLog.Content, "127.0.0.1")

	consumeLog, _ := loadRelaySmokeConsumeLog(t, env.db, user.Id)
	require.Equal(t, successChannel.Id, consumeLog.ChannelId)
}

func TestRelayBillingSmokeStreamChatCompletionUsesSubscription(t *testing.T) {
	upstream, hits := newOpenAIStreamSmokeUpstream(t)
	env := setupRelayBillingSmokeEnv(t)

	user, token := createRelaySmokeUser(t, env.db, "sub-only", "streamsmoke", "wallet_first", 100_000)
	_, sub := createRelaySmokeSubscription(t, env.db, user.Id, relaySmokeModel, 10_000)
	createRelaySmokeChannel(t, "sub-only", upstream.URL)

	recorder := performRelaySmokeJSONRequest(t, env.router, token.Key, "/v1/chat/completions", map[string]any{
		"model": relaySmokeModel,
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
		},
		"stream": true,
		"stream_options": map[string]bool{
			"include_usage": true,
		},
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int32(1), hits.Load())
	require.Contains(t, recorder.Body.String(), "data:")
	require.Contains(t, recorder.Body.String(), "[DONE]")

	var updatedSub model.UserSubscription
	require.NoError(t, env.db.First(&updatedSub, sub.Id).Error)
	require.Greater(t, updatedSub.AmountUsed, int64(0))

	log, other := loadRelaySmokeConsumeLog(t, env.db, user.Id)
	require.Equal(t, "sub-only", log.Group)
	require.Equal(t, service.BillingSourceSubscription, other["billing_source"])
	require.Equal(t, relaySmokeModel, other["subscription_model_name"])
}

func TestRelayBillingSmokeOtherOpenAICompatibleRelayTypesUseSubscription(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		path         string
		upstreamPath string
		payload      map[string]any
		response     string
	}{
		{
			name:         "completions",
			path:         "/v1/completions",
			upstreamPath: "/v1/completions",
			payload: map[string]any{
				"model":  relaySmokeModel,
				"prompt": "hello",
			},
			response: `{"id":"cmpl-smoke","object":"text_completion","created":1777377600,"model":"gpt-5-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`,
		},
		{
			name:         "embeddings",
			path:         "/v1/embeddings",
			upstreamPath: "/v1/embeddings",
			payload: map[string]any{
				"model": relaySmokeModel,
				"input": "hello",
			},
			response: `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"gpt-5-mini","usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10}}`,
		},
		{
			name:         "image-generations",
			model:        "gpt-image-1",
			path:         "/v1/images/generations",
			upstreamPath: "/v1/images/generations",
			payload: map[string]any{
				"model":  "gpt-image-1",
				"prompt": "draw a square",
				"n":      1,
				"size":   "1024x1024",
			},
			response: `{"created":1777377600,"data":[{"url":"https://example.test/image.png"}],"usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10}}`,
		},
		{
			name:         "responses",
			path:         "/v1/responses",
			upstreamPath: "/v1/responses",
			payload: map[string]any{
				"model": relaySmokeModel,
				"input": "hello",
			},
			response: `{"id":"resp_smoke","object":"response","created_at":1777377600,"status":"completed","model":"gpt-5-mini","output":[],"usage":{"input_tokens":10,"output_tokens":4,"total_tokens":14}}`,
		},
		{
			name:         "claude-compatible-messages",
			path:         "/v1/messages",
			upstreamPath: "/v1/chat/completions",
			payload: map[string]any{
				"model":      relaySmokeModel,
				"max_tokens": 16,
				"messages": []map[string]string{
					{"role": "user", "content": "hello"},
				},
			},
			response: `{"id":"chatcmpl-claude-smoke","object":"chat.completion","created":1777377600,"model":"gpt-5-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelName := tt.model
			if modelName == "" {
				modelName = relaySmokeModel
			}
			upstream, hits := newOpenAIPathSmokeUpstream(t, tt.upstreamPath, modelName, tt.response)
			env := setupRelayBillingSmokeEnv(t)

			tokenKey := strings.ReplaceAll(tt.name, "-", "") + "smoke"
			user, token := createRelaySmokeUser(t, env.db, "sub-only", tokenKey, "wallet_first", 100_000)
			_, sub := createRelaySmokeSubscription(t, env.db, user.Id, modelName, 10_000)
			createRelaySmokeChannelWithModels(t, "sub-only", upstream.URL, modelName)

			recorder := performRelaySmokeJSONRequest(t, env.router, token.Key, tt.path, tt.payload)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, int32(1), hits.Load())

			var updatedUser model.User
			require.NoError(t, env.db.First(&updatedUser, user.Id).Error)
			require.Equal(t, user.Quota, updatedUser.Quota, "subscription-only relay must not deduct wallet quota")

			var updatedSub model.UserSubscription
			require.NoError(t, env.db.First(&updatedSub, sub.Id).Error)
			require.Greater(t, updatedSub.AmountUsed, int64(0))

			_, other := loadRelaySmokeConsumeLog(t, env.db, user.Id)
			require.Equal(t, service.BillingSourceSubscription, other["billing_source"])
			require.Equal(t, ratio_setting.GroupBillingTypeSubscriptionOnly, other["billing_group_type"])
		})
	}
}

func TestRelayBillingSmokeConcurrentSubscriptionPreConsumeBoundary(t *testing.T) {
	releaseUpstream := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			close(releaseUpstream)
		})
	}
	upstream, hits := newBlockingOpenAISmokeUpstream(t, releaseUpstream)
	t.Cleanup(release)
	env := setupRelayBillingSmokeEnv(t)

	user, token := createRelaySmokeUser(t, env.db, "sub-only", "concurrentsmoke", "wallet_first", 100_000)
	modelRatio, _, _ := ratio_setting.GetModelRatio(relaySmokeModel)
	preConsumePerRequest := int(float64(common.PreConsumedQuota) * modelRatio)
	require.Greater(t, preConsumePerRequest, 0)
	_, sub := createRelaySmokeSubscription(t, env.db, user.Id, relaySmokeModel, int64(preConsumePerRequest*2))
	createRelaySmokeChannel(t, "sub-only", upstream.URL)

	const requests = 3
	start := make(chan struct{})
	results := make(chan int, requests)
	var wg sync.WaitGroup
	wg.Add(requests)
	for i := 0; i < requests; i++ {
		go func() {
			defer wg.Done()
			<-start
			recorder := performRelaySmokeRequest(t, env.router, token.Key, relaySmokeModel)
			results <- recorder.Code
		}()
	}
	close(start)

	require.Eventually(t, func() bool {
		return hits.Load() == 2
	}, 2*time.Second, 10*time.Millisecond)

	earlyCodes := make([]int, 0, 1)
	require.Eventually(t, func() bool {
		select {
		case code := <-results:
			earlyCodes = append(earlyCodes, code)
			return true
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond)
	require.Equal(t, http.StatusForbidden, earlyCodes[0])

	release()
	wg.Wait()
	close(results)

	codes := append([]int{}, earlyCodes...)
	for code := range results {
		codes = append(codes, code)
	}
	successCount := 0
	forbiddenCount := 0
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			successCount++
		case http.StatusForbidden:
			forbiddenCount++
		default:
			t.Fatalf("unexpected relay status code: %d, all codes: %v", code, codes)
		}
	}
	require.Equal(t, 2, successCount)
	require.Equal(t, 1, forbiddenCount)

	var updatedSub model.UserSubscription
	require.NoError(t, env.db.First(&updatedSub, sub.Id).Error)
	require.Greater(t, updatedSub.AmountUsed, int64(0))
	require.LessOrEqual(t, updatedSub.AmountUsed, int64(preConsumePerRequest*2))

	var recordCount int64
	require.NoError(t, env.db.Model(&model.SubscriptionPreConsumeRecord{}).Where("user_id = ? AND status = ?", user.Id, "consumed").Count(&recordCount).Error)
	require.EqualValues(t, 2, recordCount)
}

func TestRelayBillingSmokeWithRedisAndMemoryChannelCache(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_RELAY_REDIS_CONN_STRING")) == "" {
		t.Skip("set TEST_RELAY_REDIS_CONN_STRING to run Redis + memory channel cache smoke test")
	}

	upstream, hits := newOpenAISmokeUpstream(t)
	env := setupRelayBillingSmokeEnvWithOptions(t, relaySmokeOptions{
		enableRedis:       true,
		enableMemoryCache: true,
	})

	subUser, subToken := createRelaySmokeUser(t, env.db, "sub-only", "subcachesmoke", "wallet_first", 100_000)
	_, sub := createRelaySmokeSubscription(t, env.db, subUser.Id, relaySmokeModel, 10_000)
	createRelaySmokeChannel(t, "sub-only", upstream.URL)

	walletUser, walletToken := createRelaySmokeUser(t, env.db, "wallet-only", "walletcachesmoke", "subscription_first", 100_000)
	createRelaySmokeSubscription(t, env.db, walletUser.Id, relaySmokeModel, 10_000)
	createRelaySmokeChannel(t, "wallet-only", upstream.URL)

	model.InitChannelCache()
	require.NoError(t, env.db.Where("channel_id > ?", 0).Delete(&model.Ability{}).Error)

	warmRelaySmokeRedisCache(t, subUser, subToken)
	warmRelaySmokeRedisCache(t, walletUser, walletToken)

	subRecorder := performRelaySmokeRequest(t, env.router, subToken.Key, relaySmokeModel)
	require.Equal(t, http.StatusOK, subRecorder.Code, subRecorder.Body.String())

	walletRecorder := performRelaySmokeRequest(t, env.router, walletToken.Key, relaySmokeModel)
	require.Equal(t, http.StatusOK, walletRecorder.Code, walletRecorder.Body.String())
	require.Equal(t, int32(2), hits.Load())

	var updatedSub model.UserSubscription
	require.NoError(t, env.db.First(&updatedSub, sub.Id).Error)
	require.Equal(t, int64(5), updatedSub.AmountUsed)

	var updatedWalletUser model.User
	require.NoError(t, env.db.First(&updatedWalletUser, walletUser.Id).Error)
	require.Equal(t, 99_995, updatedWalletUser.Quota)

	requireRelaySmokeRedisHashField(t, fmt.Sprintf("user:%d", walletUser.Id), "Quota", "99995")
	requireRelaySmokeRedisHashField(t, "token:"+common.GenerateHMAC(walletToken.Key), "RemainQuota", "99995")

	subLog, subOther := loadRelaySmokeConsumeLog(t, env.db, subUser.Id)
	require.Equal(t, "sub-only", subLog.Group)
	require.Equal(t, service.BillingSourceSubscription, subOther["billing_source"])
	require.Equal(t, ratio_setting.GroupBillingTypeSubscriptionOnly, subOther["billing_group_type"])

	walletLog, walletOther := loadRelaySmokeConsumeLog(t, env.db, walletUser.Id)
	require.Equal(t, "wallet-only", walletLog.Group)
	require.Equal(t, service.BillingSourceWallet, walletOther["billing_source"])
	require.Equal(t, ratio_setting.GroupBillingTypeWalletOnly, walletOther["billing_group_type"])
}

func setupRelayBillingSmokeEnv(t *testing.T) relaySmokeEnv {
	return setupRelayBillingSmokeEnvWithOptions(t, relaySmokeOptions{})
}

func setupRelayBillingSmokeEnvWithOptions(t *testing.T, opts relaySmokeOptions) relaySmokeEnv {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRDB := common.RDB
	oldSQLitePath := common.SQLitePath
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	oldRedisEnabled := common.RedisEnabled
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldIsMasterNode := common.IsMasterNode
	oldLogConsumeEnabled := common.LogConsumeEnabled
	oldPreConsumedQuota := common.PreConsumedQuota
	oldStreamingTimeout := constant.StreamingTimeout
	oldModelRatio := ratio_setting.ModelRatio2JSONString()
	oldGroupRatio := ratio_setting.GroupRatio2JSONString()
	oldGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	oldGroupBillingType := ratio_setting.GroupBillingType2JSONString()
	oldUsableGroups := setting.UserUsableGroups2JSONString()
	oldVisibleGroups := setting.UserVisibleGroups2JSONString()
	oldRedisConnString, hadRedisConnString := os.LookupEnv("REDIS_CONN_STRING")
	groupSetting := ratio_setting.GetGroupRatioSetting()
	oldSpecialUsable := groupSetting.GroupSpecialUsableGroup.ReadAll()
	oldSpecialVisible := groupSetting.GroupSpecialVisibleGroup.ReadAll()
	oldSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	oldLogSQLDSN, hadLogSQLDSN := os.LookupEnv("LOG_SQL_DSN")

	var testDB *gorm.DB
	t.Cleanup(func() {
		if testDB != nil {
			if sqlDB, err := testDB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		if opts.enableRedis && common.RDB != nil {
			_ = common.RDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RDB = oldRDB
		common.SQLitePath = oldSQLitePath
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		common.RedisEnabled = oldRedisEnabled
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.IsMasterNode = oldIsMasterNode
		common.LogConsumeEnabled = oldLogConsumeEnabled
		common.PreConsumedQuota = oldPreConsumedQuota
		constant.StreamingTimeout = oldStreamingTimeout
		_ = ratio_setting.UpdateModelRatioByJSONString(oldModelRatio)
		_ = ratio_setting.UpdateGroupRatioByJSONString(oldGroupRatio)
		_ = ratio_setting.UpdateGroupGroupRatioByJSONString(oldGroupGroupRatio)
		_ = ratio_setting.UpdateGroupBillingTypeByJSONString(oldGroupBillingType)
		_ = setting.UpdateUserUsableGroupsByJSONString(oldUsableGroups)
		_ = setting.UpdateUserVisibleGroupsByJSONString(oldVisibleGroups)
		groupSetting.GroupSpecialUsableGroup.Clear()
		groupSetting.GroupSpecialUsableGroup.AddAll(oldSpecialUsable)
		groupSetting.GroupSpecialVisibleGroup.Clear()
		groupSetting.GroupSpecialVisibleGroup.AddAll(oldSpecialVisible)
		if hadSQLDSN {
			_ = os.Setenv("SQL_DSN", oldSQLDSN)
		} else {
			_ = os.Unsetenv("SQL_DSN")
		}
		if hadLogSQLDSN {
			_ = os.Setenv("LOG_SQL_DSN", oldLogSQLDSN)
		} else {
			_ = os.Unsetenv("LOG_SQL_DSN")
		}
		if hadRedisConnString {
			_ = os.Setenv("REDIS_CONN_STRING", oldRedisConnString)
		} else {
			_ = os.Unsetenv("REDIS_CONN_STRING")
		}
	})

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = opts.enableRedis
	common.MemoryCacheEnabled = opts.enableMemoryCache
	common.IsMasterNode = true
	common.LogConsumeEnabled = true
	common.PreConsumedQuota = 500
	constant.StreamingTimeout = 300
	if relaySmokeDSN := strings.TrimSpace(os.Getenv("TEST_RELAY_SQL_DSN")); relaySmokeDSN != "" {
		require.NoError(t, os.Setenv("SQL_DSN", relaySmokeDSN))
	} else {
		common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", sanitizeRelaySmokeName(t.Name()))
		require.NoError(t, os.Setenv("SQL_DSN", "local"))
	}
	require.NoError(t, os.Unsetenv("LOG_SQL_DSN"))

	require.NoError(t, model.InitDB())
	require.NoError(t, model.InitLogDB())
	testDB = model.DB
	service.InitHttpClient()
	if opts.enableRedis {
		require.NoError(t, os.Setenv("REDIS_CONN_STRING", os.Getenv("TEST_RELAY_REDIS_CONN_STRING")))
		require.NoError(t, common.InitRedisClient())
		common.RedisEnabled = true
	}

	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(ratio_setting.DefaultModelRatio2JSONString()))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"sub-only":1,"wallet-only":1}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateGroupBillingTypeByJSONString(`{"sub-only":"subscription_only","wallet-only":"wallet_only"}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","sub-only":"Subscription Only","wallet-only":"Wallet Only"}`))
	require.NoError(t, setting.UpdateUserVisibleGroupsByJSONString(""))
	groupSetting.GroupSpecialUsableGroup.Clear()
	groupSetting.GroupSpecialVisibleGroup.Clear()
	model.InvalidatePricingCache()

	engine := gin.New()
	engine.Use(middleware.RequestId())
	engine.Use(middleware.I18n())
	router.SetRelayRouter(engine)

	return relaySmokeEnv{db: testDB, router: engine}
}

func newOpenAISmokeUpstream(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	hits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)

		var upstreamRequest map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &upstreamRequest))
		require.Equal(t, relaySmokeModel, upstreamRequest["model"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-smoke","object":"chat.completion","created":1777377600,"model":"gpt-5-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`))
	}))
	t.Cleanup(server.Close)
	return server, hits
}

func newOpenAIInsufficientBalanceSmokeUpstream(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	hits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)

		var upstreamRequest map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &upstreamRequest))
		require.Equal(t, relaySmokeModel, upstreamRequest["model"])

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("unexpected status 403 Forbidden: insufficient balance AiMaMi提示： 上游账户余额可能不足，建议到上游平台确认充值状态, url: http://127.0.0.1:25817/codex/router/v1/responses, cf-ray: a021ad2fa96dd480-NRT"))
	}))
	t.Cleanup(server.Close)
	return server, hits
}

func newOpenAIStreamSmokeUpstream(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	hits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)

		var upstreamRequest map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &upstreamRequest))
		require.Equal(t, relaySmokeModel, upstreamRequest["model"])
		require.Equal(t, true, upstreamRequest["stream"])

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-stream-smoke","object":"chat.completion.chunk","created":1777377600,"model":"gpt-5-mini","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-stream-smoke","object":"chat.completion.chunk","created":1777377600,"model":"gpt-5-mini","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(server.Close)
	return server, hits
}

func newOpenAIPathSmokeUpstream(t *testing.T, path string, expectedModel string, response string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	hits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, path, r.URL.Path)

		var upstreamRequest map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &upstreamRequest))
		require.Equal(t, expectedModel, upstreamRequest["model"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server, hits
}

func newBlockingOpenAISmokeUpstream(t *testing.T, release <-chan struct{}) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	hits := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)

		var upstreamRequest map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &upstreamRequest))
		require.Equal(t, relaySmokeModel, upstreamRequest["model"])

		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-blocking-smoke","object":"chat.completion","created":1777377600,"model":"gpt-5-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`))
	}))
	t.Cleanup(server.Close)
	return server, hits
}

func createRelaySmokeUser(t *testing.T, db *gorm.DB, group, key, billingPreference string, quota int) (*model.User, *model.Token) {
	t.Helper()

	user := &model.User{
		Username: fmt.Sprintf("%s-user", key),
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    quota,
		Group:    group,
		AffCode:  fmt.Sprintf("%s-aff", key),
	}
	user.SetSetting(dto.UserSetting{BillingPreference: billingPreference})
	require.NoError(t, db.Create(user).Error)

	token := &model.Token{
		UserId:         user.Id,
		Key:            key,
		Status:         common.TokenStatusEnabled,
		Name:           fmt.Sprintf("%s token", group),
		ExpiredTime:    -1,
		RemainQuota:    quota,
		UnlimitedQuota: true,
		Group:          group,
	}
	require.NoError(t, db.Create(token).Error)
	return user, token
}

func createRelaySmokeSubscription(t *testing.T, db *gorm.DB, userID int, modelName string, totalAmount int64) (*model.SubscriptionPlan, *model.UserSubscription) {
	t.Helper()

	limitsBytes, err := common.Marshal(map[string]int64{modelName: totalAmount})
	require.NoError(t, err)

	plan := &model.SubscriptionPlan{
		Title:             "Smoke Plan",
		PriceAmount:       0,
		Currency:          "USD",
		DurationUnit:      "month",
		DurationValue:     1,
		Enabled:           true,
		TotalAmount:       totalAmount,
		ModelAmountLimits: string(limitsBytes),
		QuotaResetPeriod:  model.SubscriptionResetNever,
	}
	require.NoError(t, db.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)

	now := common.GetTimestamp()
	sub := &model.UserSubscription{
		UserId:      userID,
		PlanId:      plan.Id,
		AmountTotal: totalAmount,
		AmountUsed:  0,
		StartTime:   now - 60,
		EndTime:     now + 3600,
		Status:      "active",
		Source:      "admin",
	}
	require.NoError(t, db.Create(sub).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	return plan, sub
}

func createRelaySmokeChannel(t *testing.T, group, baseURL string) *model.Channel {
	return createRelaySmokeChannelWithModels(t, group, baseURL, relaySmokeModel)
}

func createRelaySmokeChannelWithModels(t *testing.T, group, baseURL, models string) *model.Channel {
	return createRelaySmokeChannelWithPriority(t, group, baseURL, models, 0)
}

func createRelaySmokeChannelWithPriority(t *testing.T, group, baseURL, models string, priorityValue int64) *model.Channel {
	t.Helper()

	priority := priorityValue
	weight := uint(0)
	autoBan := 0
	channel := &model.Channel{
		Type:     constant.ChannelTypeOpenAI,
		Key:      "upstream-smoke-key",
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("%s smoke channel", group),
		BaseURL:  &baseURL,
		Models:   models,
		Group:    group,
		Priority: &priority,
		Weight:   &weight,
		AutoBan:  &autoBan,
	}
	require.NoError(t, channel.Insert())
	return channel
}

func performRelaySmokeRequest(t *testing.T, engine *gin.Engine, tokenKey, modelName string) *httptest.ResponseRecorder {
	t.Helper()

	return performRelaySmokeJSONRequest(t, engine, tokenKey, "/v1/chat/completions", map[string]any{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
		},
		"stream": false,
	})
}

func performRelaySmokeJSONRequest(t *testing.T, engine *gin.Engine, tokenKey, path string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	body, err := common.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer sk-"+tokenKey)
	req.Header.Set("Content-Type", "application/json")
	if path == "/v1/messages" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}

func warmRelaySmokeRedisCache(t *testing.T, user *model.User, token *model.Token) {
	t.Helper()

	_, err := model.GetUserCache(user.Id)
	require.NoError(t, err)
	_, err = model.GetTokenByKey(token.Key, false)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		if common.RDB == nil {
			return false
		}
		ctx := context.Background()
		keys := []string{
			fmt.Sprintf("user:%d", user.Id),
			"token:" + common.GenerateHMAC(token.Key),
		}
		count, err := common.RDB.Exists(ctx, keys...).Result()
		return err == nil && count == int64(len(keys))
	}, 2*time.Second, 20*time.Millisecond)
}

func requireRelaySmokeRedisHashField(t *testing.T, key, field, expected string) {
	t.Helper()

	require.Eventually(t, func() bool {
		if common.RDB == nil {
			return false
		}
		got, err := common.RDB.HGet(context.Background(), key, field).Result()
		return err == nil && got == expected
	}, 2*time.Second, 20*time.Millisecond)
}

func loadRelaySmokeConsumeLog(t *testing.T, db *gorm.DB, userID int) (model.Log, map[string]interface{}) {
	t.Helper()

	var log model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, model.LogTypeConsume).Order("id desc").First(&log).Error)

	other := map[string]interface{}{}
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	return log, other
}

func sanitizeRelaySmokeName(name string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", "-", "_")
	return replacer.Replace(name)
}
