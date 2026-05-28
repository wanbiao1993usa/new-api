package controller

import (
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---- Shared types ----

type SubscriptionPlanDTO struct {
	Plan                     model.SubscriptionPlan `json:"plan"`
	VisibleModelAmountLimits map[string]int64       `json:"visible_model_amount_limits,omitempty"`
}

type BillingPreferenceRequest struct {
	BillingPreference string `json:"billing_preference"`
}

func ensureSubscriptionPlanPurchasable(c *gin.Context, plan *model.SubscriptionPlan) bool {
	if plan == nil {
		common.ApiErrorMsg(c, "套餐不存在")
		return false
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return false
	}
	if !plan.UserVisible {
		common.ApiErrorMsg(c, "套餐不可购买")
		return false
	}
	return true
}

// ---- User APIs ----

func GetSubscriptionPlans(c *gin.Context) {
	userId := c.GetInt("id")
	userGroup, _ := model.GetUserGroup(userId, false)
	var plans []model.SubscriptionPlan
	if err := model.DB.Where("enabled = ? AND user_visible = ?", true, true).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		result = append(result, SubscriptionPlanDTO{
			Plan:                     p,
			VisibleModelAmountLimits: buildVisibleModelAmountLimits(&p, userGroup),
		})
	}
	common.ApiSuccess(c, result)
}

func buildVisibleModelAmountLimits(plan *model.SubscriptionPlan, currentUserGroup string) map[string]int64 {
	if plan == nil {
		return nil
	}
	limits, err := plan.GetModelAmountLimitsMap()
	if err != nil || len(limits) == 0 {
		return nil
	}
	targetUserGroup := strings.TrimSpace(plan.UpgradeGroup)
	if targetUserGroup == "" {
		targetUserGroup = currentUserGroup
	}
	usableGroups := service.GetUserUsableGroups(targetUserGroup)
	enabledModels := make(map[string]struct{})
	hasSubscriptionGroup := false
	for groupName := range usableGroups {
		if ratio_setting.GetGroupBillingType(groupName) != ratio_setting.GroupBillingTypeSubscriptionOnly {
			continue
		}
		hasSubscriptionGroup = true
		for _, modelName := range model.GetGroupEnabledModels(groupName) {
			enabledModels[modelName] = struct{}{}
		}
	}
	if !hasSubscriptionGroup {
		return nil
	}
	visible := make(map[string]int64)
	for modelName, amount := range limits {
		if modelName == "*" {
			visible[modelName] = amount
			continue
		}
		if subscriptionModelLimitVisibleForEnabledModels(modelName, enabledModels) {
			visible[modelName] = amount
		}
	}
	if len(visible) == 0 {
		return nil
	}
	return visible
}

func subscriptionModelLimitVisibleForEnabledModels(limitKey string, enabledModels map[string]struct{}) bool {
	limitKey = strings.TrimSpace(limitKey)
	if limitKey == "" {
		return false
	}
	if _, ok := enabledModels[limitKey]; ok {
		return true
	}
	formatted := ratio_setting.FormatMatchingModelName(limitKey)
	if _, ok := enabledModels[formatted]; ok {
		return true
	}
	if !strings.HasSuffix(limitKey, "*") || limitKey == "*" {
		return false
	}
	prefix := strings.TrimSuffix(limitKey, "*")
	if prefix == "" || strings.Contains(prefix, "*") {
		return false
	}
	for enabledModel := range enabledModels {
		if strings.HasPrefix(enabledModel, prefix) {
			return true
		}
		formattedEnabled := ratio_setting.FormatMatchingModelName(enabledModel)
		if formattedEnabled != "" && strings.HasPrefix(formattedEnabled, prefix) {
			return true
		}
		if base, ok := ratio_setting.CompactBaseModelName(enabledModel); ok {
			base = ratio_setting.FormatMatchingModelName(base)
			if base != "" && strings.HasPrefix(base, prefix) {
				return true
			}
		}
	}
	return false
}

func GetSubscriptionSelf(c *gin.Context) {
	userId := c.GetInt("id")
	settingMap, _ := model.GetUserSetting(userId, false)
	pref := common.NormalizeBillingPreference(settingMap.BillingPreference)
	userGroup, _ := model.GetUserGroup(userId, false)
	groupBillingType := ratio_setting.GetGroupBillingType(userGroup)
	forcedBillingPreference := ""
	switch groupBillingType {
	case ratio_setting.GroupBillingTypeSubscriptionOnly:
		forcedBillingPreference = "subscription_only"
	case ratio_setting.GroupBillingTypeWalletOnly:
		forcedBillingPreference = "wallet_only"
	}

	// Get all subscriptions (including expired)
	allSubscriptions, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		allSubscriptions = []model.SubscriptionSummary{}
	}

	// Get active subscriptions for backward compatibility
	activeSubscriptions, err := model.GetAllActiveUserSubscriptions(userId)
	if err != nil {
		activeSubscriptions = []model.SubscriptionSummary{}
	}

	common.ApiSuccess(c, gin.H{
		"billing_preference":           pref,
		"effective_group":              userGroup,
		"effective_group_billing_type": groupBillingType,
		"forced_billing_preference":    forcedBillingPreference,
		"subscriptions":                activeSubscriptions, // active subscriptions
		"all_subscriptions":            allSubscriptions,    // subscriptions including expired
	})
}

func UpdateSubscriptionPreference(c *gin.Context) {
	userId := c.GetInt("id")
	var req BillingPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	pref := common.NormalizeBillingPreference(req.BillingPreference)

	user, err := model.GetUserById(userId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	current := user.GetSetting()
	current.BillingPreference = pref
	user.SetSetting(current)
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"billing_preference": pref})
}

// ---- Admin APIs ----

func AdminListSubscriptionPlans(c *gin.Context) {
	var plans []model.SubscriptionPlan
	if err := model.DB.Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		result = append(result, SubscriptionPlanDTO{
			Plan: p,
		})
	}
	common.ApiSuccess(c, result)
}

type AdminUpsertSubscriptionPlanRequest struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

type adminUpsertSubscriptionPlanProvidedFields struct {
	Enabled     bool
	UserVisible bool
}

func bindAdminUpsertSubscriptionPlanRequest(c *gin.Context, req *AdminUpsertSubscriptionPlanRequest) (adminUpsertSubscriptionPlanProvidedFields, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return adminUpsertSubscriptionPlanProvidedFields{}, err
	}
	if err := common.Unmarshal(body, req); err != nil {
		return adminUpsertSubscriptionPlanProvidedFields{}, err
	}
	var raw struct {
		Plan map[string]interface{} `json:"plan"`
	}
	provided := adminUpsertSubscriptionPlanProvidedFields{}
	if err := common.Unmarshal(body, &raw); err == nil {
		if raw.Plan != nil {
			_, provided.Enabled = raw.Plan["enabled"]
			_, provided.UserVisible = raw.Plan["user_visible"]
		}
	}
	return provided, nil
}

type AdminPlanUserSubscriptionsResponse struct {
	Records         []model.SubscriptionSummaryWithUser        `json:"records"`
	HistoricalUsage model.SubscriptionPlanHistoricalUsageStats `json:"historical_usage"`
}

func AdminCreateSubscriptionPlan(c *gin.Context) {
	var req AdminUpsertSubscriptionPlanRequest
	provided, err := bindAdminUpsertSubscriptionPlanRequest(c, &req)
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Plan.Id = 0
	if !provided.Enabled {
		req.Plan.Enabled = true
	}
	if !provided.UserVisible {
		req.Plan.UserVisible = true
	}
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return
	}
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	req.Plan.Currency = "USD"
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return
	}
	if req.Plan.TotalAmount < 0 {
		common.ApiErrorMsg(c, "总额度不能为负数")
		return
	}
	if err := model.CheckSubscriptionModelAmountLimits(req.Plan.ModelAmountLimits); err != nil {
		common.ApiErrorMsg(c, "模型限额配置错误: "+err.Error())
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return
	}
	enabled := req.Plan.Enabled
	userVisible := req.Plan.UserVisible
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&req.Plan).Error; err != nil {
			return err
		}
		updateMap := map[string]interface{}{}
		if provided.Enabled {
			updateMap["enabled"] = enabled
		}
		if provided.UserVisible {
			updateMap["user_visible"] = userVisible
		}
		if len(updateMap) == 0 {
			return nil
		}
		updateMap["updated_at"] = common.GetTimestamp()
		return tx.Model(&model.SubscriptionPlan{}).Where("id = ?", req.Plan.Id).Updates(updateMap).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	req.Plan.Enabled = enabled
	req.Plan.UserVisible = userVisible
	model.InvalidateSubscriptionPlanCache(req.Plan.Id)
	common.ApiSuccess(c, req.Plan)
}

func AdminUpdateSubscriptionPlan(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpsertSubscriptionPlanRequest
	provided, err := bindAdminUpsertSubscriptionPlanRequest(c, &req)
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var existingPlan model.SubscriptionPlan
	if !provided.Enabled || !provided.UserVisible {
		if err := model.DB.Where("id = ?", id).First(&existingPlan).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		if !provided.Enabled {
			req.Plan.Enabled = existingPlan.Enabled
		}
		if !provided.UserVisible {
			req.Plan.UserVisible = existingPlan.UserVisible
		}
	}
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return
	}
	req.Plan.Id = id
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	req.Plan.Currency = "USD"
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return
	}
	if req.Plan.TotalAmount < 0 {
		common.ApiErrorMsg(c, "总额度不能为负数")
		return
	}
	if err := model.CheckSubscriptionModelAmountLimits(req.Plan.ModelAmountLimits); err != nil {
		common.ApiErrorMsg(c, "模型限额配置错误: "+err.Error())
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// update plan (allow zero values updates with map)
		updateMap := map[string]interface{}{
			"title":                      req.Plan.Title,
			"subtitle":                   req.Plan.Subtitle,
			"price_amount":               req.Plan.PriceAmount,
			"currency":                   req.Plan.Currency,
			"duration_unit":              req.Plan.DurationUnit,
			"duration_value":             req.Plan.DurationValue,
			"custom_seconds":             req.Plan.CustomSeconds,
			"enabled":                    req.Plan.Enabled,
			"user_visible":               req.Plan.UserVisible,
			"sort_order":                 req.Plan.SortOrder,
			"stripe_price_id":            req.Plan.StripePriceId,
			"creem_product_id":           req.Plan.CreemProductId,
			"max_purchase_per_user":      req.Plan.MaxPurchasePerUser,
			"total_amount":               req.Plan.TotalAmount,
			"model_amount_limits":        req.Plan.ModelAmountLimits,
			"upgrade_group":              req.Plan.UpgradeGroup,
			"quota_reset_period":         req.Plan.QuotaResetPeriod,
			"quota_reset_custom_seconds": req.Plan.QuotaResetCustomSeconds,
			"updated_at":                 common.GetTimestamp(),
		}
		if err := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Updates(updateMap).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

type AdminUpdateSubscriptionPlanStatusRequest struct {
	Enabled     *bool `json:"enabled"`
	UserVisible *bool `json:"user_visible"`
}

func AdminUpdateSubscriptionPlanStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpdateSubscriptionPlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || (req.Enabled == nil && req.UserVisible == nil) {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	updateMap := map[string]interface{}{
		"updated_at": common.GetTimestamp(),
	}
	if req.Enabled != nil {
		updateMap["enabled"] = *req.Enabled
	}
	if req.UserVisible != nil {
		updateMap["user_visible"] = *req.UserVisible
	}
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Updates(updateMap).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

type AdminBindSubscriptionRequest struct {
	UserId int `json:"user_id"`
	PlanId int `json:"plan_id"`
}

func AdminBindSubscription(c *gin.Context) {
	var req AdminBindSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	msg, err := model.AdminBindSubscription(req.UserId, req.PlanId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// ---- Admin: user subscription management ----

func AdminListUserSubscriptions(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	subs, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, subs)
}

func AdminListPlanUserSubscriptions(c *gin.Context) {
	planId, _ := strconv.Atoi(c.Param("id"))
	if planId <= 0 {
		common.ApiErrorMsg(c, "无效的套餐ID")
		return
	}
	subs, err := model.GetAllUserSubscriptionsByPlan(planId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	historicalUsage, err := model.GetSubscriptionPlanHistoricalUsageStats(planId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, AdminPlanUserSubscriptionsResponse{
		Records:         subs,
		HistoricalUsage: historicalUsage,
	})
}

type AdminCreateUserSubscriptionRequest struct {
	PlanId int `json:"plan_id"`
}

// AdminCreateUserSubscription creates a new user subscription from a plan (no payment).
func AdminCreateUserSubscription(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	var req AdminCreateUserSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	msg, err := model.AdminBindSubscription(userId, req.PlanId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminInvalidateUserSubscription cancels a user subscription immediately.
func AdminInvalidateUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	msg, err := model.AdminInvalidateUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	msg, err := model.AdminDeleteUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}
