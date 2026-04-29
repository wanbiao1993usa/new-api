package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/samber/hot"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Subscription duration units
const (
	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"
)

// Subscription quota reset period
const (
	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetCustom  = "custom"
)

var (
	ErrSubscriptionOrderNotFound          = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid     = errors.New("subscription order status invalid")
	ErrNoActiveSubscription               = errors.New("no active subscription")
	ErrSubscriptionQuotaInsufficient      = errors.New("subscription quota insufficient")
	ErrSubscriptionModelQuotaInsufficient = errors.New("subscription model quota insufficient")
)

const (
	subscriptionPlanCacheNamespace     = "new-api:subscription_plan:v1"
	subscriptionPlanInfoCacheNamespace = "new-api:subscription_plan_info:v1"
)

var (
	subscriptionPlanCacheOnce     sync.Once
	subscriptionPlanInfoCacheOnce sync.Once

	subscriptionPlanCache     *cachex.HybridCache[SubscriptionPlan]
	subscriptionPlanInfoCache *cachex.HybridCache[SubscriptionPlanInfo]
)

func subscriptionPlanCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanInfoCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_TTL", 120)
	if ttlSeconds <= 0 {
		ttlSeconds = 120
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_CAP", 5000)
	if capacity <= 0 {
		capacity = 5000
	}
	return capacity
}

func subscriptionPlanInfoCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_CAP", 10000)
	if capacity <= 0 {
		capacity = 10000
	}
	return capacity
}

func getSubscriptionPlanCache() *cachex.HybridCache[SubscriptionPlan] {
	subscriptionPlanCacheOnce.Do(func() {
		ttl := subscriptionPlanCacheTTL()
		subscriptionPlanCache = cachex.NewHybridCache[SubscriptionPlan](cachex.HybridCacheConfig[SubscriptionPlan]{
			Namespace: cachex.Namespace(subscriptionPlanCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlan]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlan] {
				return hot.NewHotCache[string, SubscriptionPlan](hot.LRU, subscriptionPlanCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanCache
}

func getSubscriptionPlanInfoCache() *cachex.HybridCache[SubscriptionPlanInfo] {
	subscriptionPlanInfoCacheOnce.Do(func() {
		ttl := subscriptionPlanInfoCacheTTL()
		subscriptionPlanInfoCache = cachex.NewHybridCache[SubscriptionPlanInfo](cachex.HybridCacheConfig[SubscriptionPlanInfo]{
			Namespace: cachex.Namespace(subscriptionPlanInfoCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlanInfo]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlanInfo] {
				return hot.NewHotCache[string, SubscriptionPlanInfo](hot.LRU, subscriptionPlanInfoCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanInfoCache
}

func subscriptionPlanCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func InvalidateSubscriptionPlanCache(planId int) {
	if planId <= 0 {
		return
	}
	cache := getSubscriptionPlanCache()
	_, _ = cache.DeleteMany([]string{subscriptionPlanCacheKey(planId)})
	infoCache := getSubscriptionPlanInfoCache()
	_ = infoCache.Purge()
}

// Subscription plan
type SubscriptionPlan struct {
	Id int `json:"id"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

	// Display money amount (follow existing code style: float64 for money)
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
	Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`

	DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
	DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
	CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

	Enabled   bool `json:"enabled" gorm:"default:true"`
	SortOrder int  `json:"sort_order" gorm:"type:int;default:0"`

	StripePriceId  string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
	CreemProductId string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`

	// Max purchases per user (0 = unlimited)
	MaxPurchasePerUser int `json:"max_purchase_per_user" gorm:"type:int;default:0"`

	// Upgrade user group after purchase (empty = no change)
	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

	// Total quota (amount in quota units, 0 = unlimited)
	TotalAmount int64 `json:"total_amount" gorm:"type:bigint;not null;default:0"`

	// Per-model quota limits, JSON map: model name -> quota units, "*" is default.
	ModelAmountLimits string `json:"model_amount_limits" gorm:"type:text"`

	// Quota reset period for plan
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

func ParseSubscriptionModelAmountLimits(raw string) (map[string]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]int64{}, nil
	}
	limits := make(map[string]int64)
	if err := common.UnmarshalJsonStr(raw, &limits); err != nil {
		return nil, err
	}
	return limits, nil
}

func CheckSubscriptionModelAmountLimits(raw string) error {
	limits, err := ParseSubscriptionModelAmountLimits(raw)
	if err != nil {
		return err
	}
	for modelName, amount := range limits {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			return errors.New("model name cannot be empty")
		}
		if strings.Contains(modelName, "*") && modelName != "*" {
			if !strings.HasSuffix(modelName, "*") || strings.Contains(strings.TrimSuffix(modelName, "*"), "*") {
				return fmt.Errorf("model wildcard only supports a single trailing *: %s", modelName)
			}
		}
		if amount < 0 {
			return fmt.Errorf("model amount limit cannot be negative: %s", modelName)
		}
	}
	return nil
}

func (p *SubscriptionPlan) GetModelAmountLimitsMap() (map[string]int64, error) {
	if p == nil {
		return map[string]int64{}, nil
	}
	return ParseSubscriptionModelAmountLimits(p.ModelAmountLimits)
}

func getSubscriptionModelLimitMatch(plan *SubscriptionPlan, modelName string) (map[string]int64, string, int64, bool, error) {
	limits, err := plan.GetModelAmountLimitsMap()
	if err != nil {
		return nil, "", 0, false, err
	}
	if len(limits) == 0 {
		return limits, "", 0, false, nil
	}
	limitKey := matchSubscriptionModelLimitKey(limits, modelName)
	if limitKey == "" {
		return limits, "", 0, false, nil
	}
	return limits, limitKey, limits[limitKey], true, nil
}

func findSubscriptionModelLimit(plan *SubscriptionPlan, modelName string) (int64, bool, error) {
	_, _, limit, matched, err := getSubscriptionModelLimitMatch(plan, modelName)
	return limit, matched, err
}

func subscriptionModelLimitCandidates(modelName string) []string {
	modelName = strings.TrimSpace(modelName)
	raw := make([]string, 0, 3)
	if modelName != "" {
		raw = append(raw, modelName)
	}
	if formatted := ratio_setting.FormatMatchingModelName(modelName); formatted != "" {
		raw = append(raw, formatted)
	}
	if base, ok := ratio_setting.CompactBaseModelName(modelName); ok {
		base = ratio_setting.FormatMatchingModelName(base)
		if base != "" {
			raw = append(raw, base)
		}
	}
	seen := make(map[string]struct{}, len(raw))
	candidates := make([]string, 0, len(raw))
	for _, candidate := range raw {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func subscriptionModelLimitWildcardPrefix(limitKey string) (string, bool) {
	limitKey = strings.TrimSpace(limitKey)
	if limitKey == "*" || !strings.HasSuffix(limitKey, "*") {
		return "", false
	}
	prefix := strings.TrimSuffix(limitKey, "*")
	if prefix == "" || strings.Contains(prefix, "*") {
		return "", false
	}
	return prefix, true
}

// Subscription order (payment -> webhook -> create UserSubscription)
type SubscriptionOrder struct {
	Id     int     `json:"id"`
	UserId int     `json:"user_id" gorm:"index"`
	PlanId int     `json:"plan_id" gorm:"index"`
	Money  float64 `json:"money"`

	TradeNo         string `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider string `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	Status          string `json:"status"`
	CreateTime      int64  `json:"create_time"`
	CompleteTime    int64  `json:"complete_time"`

	ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}

func (o *SubscriptionOrder) Insert() error {
	if o.CreateTime == 0 {
		o.CreateTime = common.GetTimestamp()
	}
	return DB.Create(o).Error
}

func (o *SubscriptionOrder) Update() error {
	return DB.Save(o).Error
}

func GetSubscriptionOrderByTradeNo(tradeNo string) *SubscriptionOrder {
	if tradeNo == "" {
		return nil
	}
	var order SubscriptionOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

// User subscription instance
type UserSubscription struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
	PlanId int `json:"plan_id" gorm:"index"`

	AmountTotal int64 `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed  int64 `json:"amount_used" gorm:"type:bigint;not null;default:0"`

	StartTime int64  `json:"start_time" gorm:"bigint"`
	EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"` // active/expired/cancelled

	Source string `json:"source" gorm:"type:varchar(32);default:'order'"` // order/admin

	LastResetTime int64 `json:"last_reset_time" gorm:"type:bigint;default:0"`
	NextResetTime int64 `json:"next_reset_time" gorm:"type:bigint;default:0;index"`

	UpgradeGroup  string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	PrevUserGroup string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

type UserSubscriptionModelUsage struct {
	Id                 int    `json:"id"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"uniqueIndex:idx_user_subscription_model_usage,priority:1;index"`
	UserId             int    `json:"user_id" gorm:"index"`
	ModelName          string `json:"model_name" gorm:"type:varchar(255);uniqueIndex:idx_user_subscription_model_usage,priority:2"`
	AmountUsed         int64  `json:"amount_used" gorm:"type:bigint;not null;default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint"`
}

func (u *UserSubscriptionModelUsage) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	u.CreatedAt = now
	u.UpdatedAt = now
	return nil
}

func (u *UserSubscriptionModelUsage) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = common.GetTimestamp()
	return nil
}

func (s *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *UserSubscription) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

type SubscriptionSummary struct {
	Subscription           *UserSubscription `json:"subscription"`
	Plan                   *SubscriptionPlan `json:"plan,omitempty"`
	ModelAmountLimits      map[string]int64  `json:"model_amount_limits,omitempty"`
	ModelAmountUsages      map[string]int64  `json:"model_amount_usages,omitempty"`
	ModelAmountLimitUsages map[string]int64  `json:"model_amount_limit_usages,omitempty"`
}

type SubscriptionUserInfo struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Group       string `json:"group,omitempty"`
}

type SubscriptionSummaryWithUser struct {
	SubscriptionSummary
	User *SubscriptionUserInfo `json:"user,omitempty"`
}

func calcPlanEndTime(start time.Time, plan *SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func NormalizeResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case SubscriptionResetDaily, SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetCustom:
		return strings.TrimSpace(period)
	default:
		return SubscriptionResetNever
	}
}

func calcNextResetTime(base time.Time, plan *SubscriptionPlan, endUnix int64) int64 {
	if plan == nil {
		return 0
	}
	period := NormalizeResetPeriod(plan.QuotaResetPeriod)
	if period == SubscriptionResetNever {
		return 0
	}
	var next time.Time
	switch period {
	case SubscriptionResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 1)
	case SubscriptionResetWeekly:
		// Align to next Monday 00:00
		weekday := int(base.Weekday()) // Sunday=0
		// Convert to Monday=1..Sunday=7
		if weekday == 0 {
			weekday = 7
		}
		daysUntil := 8 - weekday
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, daysUntil)
	case SubscriptionResetMonthly:
		// Align to first day of next month 00:00
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).
			AddDate(0, 1, 0)
	case SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 {
			return 0
		}
		next = base.Add(time.Duration(plan.QuotaResetCustomSeconds) * time.Second)
	default:
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
}

func GetSubscriptionPlanById(id int) (*SubscriptionPlan, error) {
	return getSubscriptionPlanByIdTx(nil, id)
}

func getSubscriptionPlanByIdTx(tx *gorm.DB, id int) (*SubscriptionPlan, error) {
	if id <= 0 {
		return nil, errors.New("invalid plan id")
	}
	key := subscriptionPlanCacheKey(id)
	if key != "" {
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
			return &cached, nil
		}
	}
	var plan SubscriptionPlan
	query := DB
	if tx != nil {
		query = tx
	}
	if err := query.Where("id = ?", id).First(&plan).Error; err != nil {
		return nil, err
	}
	_ = getSubscriptionPlanCache().SetWithTTL(key, plan, subscriptionPlanCacheTTL())
	return &plan, nil
}

func CountUserSubscriptionsByPlan(userId int, planId int) (int64, error) {
	if userId <= 0 || planId <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userId, planId).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func getUserGroupByIdTx(tx *gorm.DB, userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	if tx == nil {
		tx = DB
	}
	var group string
	if err := tx.Model(&User{}).Where("id = ?", userId).Select(commonGroupCol).Find(&group).Error; err != nil {
		return "", err
	}
	return group, nil
}

func downgradeUserGroupForSubscriptionTx(tx *gorm.DB, sub *UserSubscription, now int64) (string, error) {
	if tx == nil || sub == nil {
		return "", errors.New("invalid downgrade args")
	}
	upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
	if upgradeGroup == "" {
		return "", nil
	}
	currentGroup, err := getUserGroupByIdTx(tx, sub.UserId)
	if err != nil {
		return "", err
	}
	if currentGroup != upgradeGroup {
		return "", nil
	}
	var activeSub UserSubscription
	activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND id <> ? AND upgrade_group <> ''",
		sub.UserId, "active", now, sub.Id).
		Order("end_time desc, id desc").
		Limit(1).
		Find(&activeSub)
	if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
		return "", nil
	}
	prevGroup := strings.TrimSpace(sub.PrevUserGroup)
	if prevGroup == "" {
		var prevSource UserSubscription
		prevQuery := tx.Where("user_id = ? AND upgrade_group = ? AND prev_user_group <> '' AND id <> ?",
			sub.UserId, upgradeGroup, sub.Id).
			Order("end_time desc, id desc").
			Limit(1).
			Find(&prevSource)
		if prevQuery.Error != nil {
			return "", prevQuery.Error
		}
		if prevQuery.RowsAffected > 0 {
			prevGroup = strings.TrimSpace(prevSource.PrevUserGroup)
		}
	}
	if prevGroup == "" || prevGroup == currentGroup {
		return "", nil
	}
	if err := tx.Model(&User{}).Where("id = ?", sub.UserId).
		Update("group", prevGroup).Error; err != nil {
		return "", err
	}
	return prevGroup, nil
}

func CreateUserSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if plan.MaxPurchasePerUser > 0 {
		var count int64
		if err := tx.Model(&UserSubscription{}).
			Where("user_id = ? AND plan_id = ?", userId, plan.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("已达到该套餐购买上限")
		}
	}
	nowUnix := GetDBTimestamp()
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	resetBase := now
	nextReset := calcNextResetTime(resetBase, plan, endUnix)
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	prevGroup := ""
	if upgradeGroup != "" {
		currentGroup, err := getUserGroupByIdTx(tx, userId)
		if err != nil {
			return nil, err
		}
		if currentGroup != upgradeGroup {
			prevGroup = currentGroup
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", upgradeGroup).Error; err != nil {
				return nil, err
			}
		} else {
			var activeSub UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group = ? AND prev_user_group <> ''",
				userId, "active", nowUnix, upgradeGroup).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error != nil {
				return nil, activeQuery.Error
			}
			if activeQuery.RowsAffected > 0 {
				prevGroup = strings.TrimSpace(activeSub.PrevUserGroup)
			}
		}
	}
	sub := &UserSubscription{
		UserId:        userId,
		PlanId:        plan.Id,
		AmountTotal:   plan.TotalAmount,
		AmountUsed:    0,
		StartTime:     now.Unix(),
		EndTime:       endUnix,
		Status:        "active",
		Source:        source,
		LastResetTime: lastReset,
		NextResetTime: nextReset,
		UpgradeGroup:  upgradeGroup,
		PrevUserGroup: prevGroup,
		CreatedAt:     common.GetTimestamp(),
		UpdatedAt:     common.GetTimestamp(),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

// Complete a subscription order (idempotent). Creates a UserSubscription snapshot from the plan.
// expectedPaymentProvider guards against cross-gateway callback attacks (empty skips the check).
// actualPaymentMethod updates the order's PaymentMethod to reflect the real payment type used (empty skips update).
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	var logUserId int
	var logPlanTitle string
	var logMoney float64
	var logPaymentMethod string
	var upgradeGroup string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		plan, err := GetSubscriptionPlanById(order.PlanId)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			// still allow completion for already purchased orders
		}
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		_, err = CreateUserSubscriptionFromPlanTx(tx, order.UserId, plan, "order")
		if err != nil {
			return err
		}
		if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return err
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" && order.PaymentMethod != actualPaymentMethod {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		logUserId = order.UserId
		logPlanTitle = plan.Title
		logMoney = order.Money
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if upgradeGroup != "" && logUserId > 0 {
		_ = UpdateUserGroupCache(logUserId, upgradeGroup)
	}
	if logUserId > 0 {
		msg := fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", logPlanTitle, logMoney, logPaymentMethod)
		RecordLog(logUserId, LogTypeTopup, msg)
	}
	return nil
}

func upsertSubscriptionTopUpTx(tx *gorm.DB, order *SubscriptionOrder) error {
	if tx == nil || order == nil {
		return errors.New("invalid subscription order")
	}
	now := common.GetTimestamp()
	var topup TopUp
	if err := tx.Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			topup = TopUp{
				UserId:        order.UserId,
				Amount:        0,
				Money:         order.Money,
				TradeNo:       order.TradeNo,
				PaymentMethod: order.PaymentMethod,
				CreateTime:    order.CreateTime,
				CompleteTime:  now,
				Status:        common.TopUpStatusSuccess,
			}
			return tx.Create(&topup).Error
		}
		return err
	}
	topup.Money = order.Money
	if topup.PaymentMethod == "" {
		topup.PaymentMethod = order.PaymentMethod
	} else if topup.PaymentMethod != order.PaymentMethod {
		return ErrPaymentMethodMismatch
	}
	if topup.CreateTime == 0 {
		topup.CreateTime = order.CreateTime
	}
	topup.CompleteTime = now
	topup.Status = common.TopUpStatusSuccess
	return tx.Save(&topup).Error
}

func ExpireSubscriptionOrder(tradeNo string, expectedPaymentProvider string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		order.Status = common.TopUpStatusExpired
		order.CompleteTime = common.GetTimestamp()
		return tx.Save(&order).Error
	})
}

// Admin bind (no payment). Creates a UserSubscription from a plan.
func AdminBindSubscription(userId int, planId int, sourceNote string) (string, error) {
	if userId <= 0 || planId <= 0 {
		return "", errors.New("invalid userId or planId")
	}
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return "", err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		return err
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(plan.UpgradeGroup) != "" {
		_ = UpdateUserGroupCache(userId, plan.UpgradeGroup)
		return fmt.Sprintf("用户分组将升级到 %s", plan.UpgradeGroup), nil
	}
	return "", nil
}

// GetAllActiveUserSubscriptions returns all active subscriptions for a user.
func GetAllActiveUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var subs []UserSubscription
	err := DB.Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// HasActiveUserSubscription returns whether the user has any active subscription.
// This is a lightweight existence check to avoid heavy pre-consume transactions.
func HasActiveUserSubscription(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAllUserSubscriptions returns all subscriptions (active and expired) for a user.
func GetAllUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var subs []UserSubscription
	err := DB.Where("user_id = ?", userId).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// GetAllUserSubscriptionsByPlan returns all subscriptions for a plan with safe user metadata.
func GetAllUserSubscriptionsByPlan(planId int) ([]SubscriptionSummaryWithUser, error) {
	if planId <= 0 {
		return nil, errors.New("invalid planId")
	}
	var subs []UserSubscription
	err := DB.Where("plan_id = ?", planId).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		return []SubscriptionSummaryWithUser{}, nil
	}

	userIds := make([]int, 0, len(subs))
	seenUsers := make(map[int]struct{})
	for _, sub := range subs {
		if sub.UserId <= 0 {
			continue
		}
		if _, ok := seenUsers[sub.UserId]; ok {
			continue
		}
		seenUsers[sub.UserId] = struct{}{}
		userIds = append(userIds, sub.UserId)
	}

	userMap := make(map[int]SubscriptionUserInfo, len(userIds))
	if len(userIds) > 0 {
		var users []User
		if err := DB.Where("id IN ?", userIds).Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			userMap[user.Id] = SubscriptionUserInfo{
				Id:          user.Id,
				Username:    user.Username,
				DisplayName: user.DisplayName,
				Email:       user.Email,
				Group:       user.Group,
			}
		}
	}

	summaries := buildSubscriptionSummaries(subs)
	result := make([]SubscriptionSummaryWithUser, 0, len(summaries))
	for _, summary := range summaries {
		item := SubscriptionSummaryWithUser{
			SubscriptionSummary: summary,
		}
		if summary.Subscription != nil {
			if user, ok := userMap[summary.Subscription.UserId]; ok {
				userCopy := user
				item.User = &userCopy
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func buildSubscriptionSummaries(subs []UserSubscription) []SubscriptionSummary {
	if len(subs) == 0 {
		return []SubscriptionSummary{}
	}
	planIds := make([]int, 0, len(subs))
	subIds := make([]int, 0, len(subs))
	seenPlans := make(map[int]struct{})
	for _, sub := range subs {
		subIds = append(subIds, sub.Id)
		if sub.PlanId <= 0 {
			continue
		}
		if _, ok := seenPlans[sub.PlanId]; ok {
			continue
		}
		seenPlans[sub.PlanId] = struct{}{}
		planIds = append(planIds, sub.PlanId)
	}

	planMap := make(map[int]SubscriptionPlan)
	if len(planIds) > 0 {
		var plans []SubscriptionPlan
		if err := DB.Where("id IN ?", planIds).Find(&plans).Error; err == nil {
			for _, plan := range plans {
				planMap[plan.Id] = plan
			}
		}
	}

	modelUsageMap := make(map[int]map[string]int64)
	if len(subIds) > 0 {
		var usages []UserSubscriptionModelUsage
		if err := DB.Where("user_subscription_id IN ?", subIds).Find(&usages).Error; err == nil {
			for _, usage := range usages {
				if strings.TrimSpace(usage.ModelName) == "" {
					continue
				}
				if _, ok := modelUsageMap[usage.UserSubscriptionId]; !ok {
					modelUsageMap[usage.UserSubscriptionId] = make(map[string]int64)
				}
				modelUsageMap[usage.UserSubscriptionId][usage.ModelName] += usage.AmountUsed
			}
		}
	}

	result := make([]SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		summary := SubscriptionSummary{
			Subscription: &subCopy,
		}
		if plan, ok := planMap[sub.PlanId]; ok {
			planCopy := plan
			summary.Plan = &planCopy
			if limits, err := planCopy.GetModelAmountLimitsMap(); err == nil && len(limits) > 0 {
				summary.ModelAmountLimits = limits
				summary.ModelAmountLimitUsages = buildSubscriptionModelLimitUsages(limits, modelUsageMap[sub.Id])
			}
		}
		if usages := modelUsageMap[sub.Id]; len(usages) > 0 {
			summary.ModelAmountUsages = usages
		}
		result = append(result, summary)
	}
	return result
}

func buildSubscriptionModelLimitUsages(limits map[string]int64, usages map[string]int64) map[string]int64 {
	if len(limits) == 0 {
		return nil
	}
	result := make(map[string]int64, len(limits))
	for limitKey := range limits {
		result[limitKey] = 0
	}
	for modelName, used := range usages {
		if used == 0 {
			continue
		}
		if limitKey := matchSubscriptionModelLimitKey(limits, modelName); limitKey != "" {
			result[limitKey] += used
		}
	}
	return result
}

func matchSubscriptionModelLimitKey(limits map[string]int64, modelName string) string {
	if len(limits) == 0 {
		return ""
	}
	candidates := subscriptionModelLimitCandidates(modelName)
	for _, candidate := range candidates {
		if _, ok := limits[candidate]; ok {
			return candidate
		}
	}
	wildcardKey := ""
	wildcardPrefixLen := -1
	for limitKey := range limits {
		prefix, ok := subscriptionModelLimitWildcardPrefix(limitKey)
		if !ok {
			continue
		}
		for _, candidate := range candidates {
			if !strings.HasPrefix(candidate, prefix) {
				continue
			}
			if len(prefix) > wildcardPrefixLen || (len(prefix) == wildcardPrefixLen && limitKey < wildcardKey) {
				wildcardKey = limitKey
				wildcardPrefixLen = len(prefix)
			}
			break
		}
	}
	if wildcardKey != "" {
		return wildcardKey
	}
	if _, ok := limits["*"]; ok {
		return "*"
	}
	return ""
}

func getSubscriptionModelLimitUsedTx(tx *gorm.DB, subId int, limits map[string]int64, limitKey string) (int64, error) {
	if tx == nil {
		return 0, errors.New("tx is nil")
	}
	if subId <= 0 || len(limits) == 0 || strings.TrimSpace(limitKey) == "" {
		return 0, nil
	}
	var usages []UserSubscriptionModelUsage
	if err := tx.Where("user_subscription_id = ?", subId).Find(&usages).Error; err != nil {
		return 0, err
	}
	var used int64
	for _, usage := range usages {
		if matchSubscriptionModelLimitKey(limits, usage.ModelName) == limitKey {
			used += usage.AmountUsed
		}
	}
	return used, nil
}

// AdminInvalidateUserSubscription marks a user subscription as cancelled and ends it immediately.
func AdminInvalidateUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"status":     "cancelled",
			"end_time":   now,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		if err := tx.Where("id = ?", userSubscriptionId).Delete(&UserSubscription{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

type SubscriptionPreConsumeResult struct {
	UserSubscriptionId    int
	PreConsumed           int64
	AmountTotal           int64
	AmountUsedBefore      int64
	AmountUsedAfter       int64
	ModelName             string
	ModelLimitMatched     bool
	ModelAmountLimit      int64
	ModelAmountUsedBefore int64
	ModelAmountUsedAfter  int64
}

// ExpireDueSubscriptions marks expired subscriptions and handles group downgrade.
func ExpireDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("status = ? AND end_time > 0 AND end_time <= ?", "active", now).
		Order("end_time asc, id asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	expiredCount := 0
	userIds := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.UserId > 0 {
			userIds[sub.UserId] = struct{}{}
		}
	}
	for userId := range userIds {
		cacheGroup := ""
		err := DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND status = ? AND end_time > 0 AND end_time <= ?", userId, "active", now).
				Updates(map[string]interface{}{
					"status":     "expired",
					"updated_at": common.GetTimestamp(),
				})
			if res.Error != nil {
				return res.Error
			}
			expiredCount += int(res.RowsAffected)

			// If there's an active upgraded subscription, keep current group.
			var activeSub UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group <> ''",
				userId, "active", now).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
				return nil
			}

			// No active upgraded subscription, downgrade to previous group if needed.
			var lastExpired UserSubscription
			expiredQuery := tx.Where("user_id = ? AND status = ? AND upgrade_group <> '' AND prev_user_group <> ''",
				userId, "expired").
				Order("end_time desc, id desc").
				Limit(1).
				Find(&lastExpired)
			if expiredQuery.Error != nil || expiredQuery.RowsAffected == 0 {
				return nil
			}
			upgradeGroup := strings.TrimSpace(lastExpired.UpgradeGroup)
			prevGroup := strings.TrimSpace(lastExpired.PrevUserGroup)
			if upgradeGroup == "" || prevGroup == "" {
				return nil
			}
			currentGroup, err := getUserGroupByIdTx(tx, userId)
			if err != nil {
				return err
			}
			if currentGroup != upgradeGroup || currentGroup == prevGroup {
				return nil
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", prevGroup).Error; err != nil {
				return err
			}
			cacheGroup = prevGroup
			return nil
		})
		if err != nil {
			return expiredCount, err
		}
		if cacheGroup != "" {
			_ = UpdateUserGroupCache(userId, cacheGroup)
		}
	}
	return expiredCount, nil
}

// SubscriptionPreConsumeRecord stores idempotent pre-consume operations per request.
type SubscriptionPreConsumeRecord struct {
	Id                 int    `json:"id"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId             int    `json:"user_id" gorm:"index"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	ModelName          string `json:"model_name" gorm:"type:varchar(255);index"`
	PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);index"` // consumed/refunded
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	if sub.NextResetTime > 0 && sub.NextResetTime > now {
		return nil
	}
	if NormalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever {
		return nil
	}
	baseUnix := sub.LastResetTime
	if baseUnix <= 0 {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextResetTime(base, plan, sub.EndTime)
	advanced := false
	for next > 0 && next <= now {
		advanced = true
		base = time.Unix(next, 0)
		next = calcNextResetTime(base, plan, sub.EndTime)
	}
	if !advanced {
		if sub.NextResetTime == 0 && next > 0 {
			sub.NextResetTime = next
			sub.LastResetTime = base.Unix()
			return tx.Save(sub).Error
		}
		return nil
	}
	sub.AmountUsed = 0
	if err := tx.Model(&UserSubscriptionModelUsage{}).
		Where("user_subscription_id = ?", sub.Id).
		Update("amount_used", 0).Error; err != nil {
		return err
	}
	sub.LastResetTime = base.Unix()
	sub.NextResetTime = next
	return tx.Save(sub).Error
}

func normalizedSubscriptionModelName(modelName string) string {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return ""
	}
	return modelName
}

func ensureSubscriptionModelUsageTx(tx *gorm.DB, sub *UserSubscription, modelName string) (*UserSubscriptionModelUsage, error) {
	if tx == nil || sub == nil {
		return nil, errors.New("invalid model usage args")
	}
	modelName = normalizedSubscriptionModelName(modelName)
	if modelName == "" {
		return nil, nil
	}
	seed := &UserSubscriptionModelUsage{
		UserSubscriptionId: sub.Id,
		UserId:             sub.UserId,
		ModelName:          modelName,
		AmountUsed:         0,
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_subscription_id"}, {Name: "model_name"}},
		DoNothing: true,
	}).Create(seed).Error; err != nil {
		return nil, err
	}
	var usage UserSubscriptionModelUsage
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("user_subscription_id = ? AND model_name = ?", sub.Id, modelName).
		First(&usage).Error; err != nil {
		return nil, err
	}
	return &usage, nil
}

func applySubscriptionModelUsageDeltaTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, modelName string, delta int64, enforceLimit bool) (bool, int64, int64, int64, error) {
	modelName = normalizedSubscriptionModelName(modelName)
	if modelName == "" {
		return false, 0, 0, 0, nil
	}
	usage, err := ensureSubscriptionModelUsageTx(tx, sub, modelName)
	if err != nil {
		return false, 0, 0, 0, err
	}
	actualUsedBefore := usage.AmountUsed
	actualUsedAfter := actualUsedBefore + delta
	if actualUsedAfter < 0 {
		actualUsedAfter = 0
	}
	effectiveDelta := actualUsedAfter - actualUsedBefore
	usedBefore := actualUsedBefore
	limits, limitKey, limit, matched, err := getSubscriptionModelLimitMatch(plan, modelName)
	if err != nil {
		return false, 0, 0, 0, err
	}
	if matched {
		usedBefore, err = getSubscriptionModelLimitUsedTx(tx, sub.Id, limits, limitKey)
		if err != nil {
			return false, 0, 0, 0, err
		}
	}
	newUsed := usedBefore + effectiveDelta
	if newUsed < 0 {
		newUsed = 0
	}
	if enforceLimit && matched && newUsed > limit {
		return matched, limit, usedBefore, usedBefore, fmt.Errorf("%w, model=%s need=%d limit=%d used=%d", ErrSubscriptionModelQuotaInsufficient, modelName, delta, limit, usedBefore)
	}
	usage.AmountUsed = actualUsedAfter
	if err := tx.Save(usage).Error; err != nil {
		return false, 0, 0, 0, err
	}
	return matched, limit, usedBefore, newUsed, nil
}

func shouldRetrySubscriptionSQLiteTx(err error) bool {
	if err == nil || !common.UsingSQLite {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "database table is locked") ||
		strings.Contains(errMsg, "database is locked") ||
		strings.Contains(errMsg, "database is deadlocked")
}

// PreConsumeUserSubscription pre-consumes from any active subscription total quota.
func PreConsumeUserSubscription(requestId string, userId int, modelName string, quotaType int, amount int64) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount < 0 {
		return nil, errors.New("amount must be >= 0")
	}
	modelName = normalizedSubscriptionModelName(modelName)
	now := GetDBTimestamp()

	var returnValue *SubscriptionPreConsumeResult
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		returnValue = &SubscriptionPreConsumeResult{}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var existing SubscriptionPreConsumeRecord
			query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
			if query.Error != nil {
				return query.Error
			}
			if query.RowsAffected > 0 {
				if existing.Status == "refunded" {
					return errors.New("subscription pre-consume already refunded")
				}
				var sub UserSubscription
				if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&sub).Error; err != nil {
					return err
				}
				returnValue.UserSubscriptionId = sub.Id
				returnValue.PreConsumed = existing.PreConsumed
				returnValue.AmountTotal = sub.AmountTotal
				returnValue.AmountUsedBefore = sub.AmountUsed
				returnValue.AmountUsedAfter = sub.AmountUsed
				returnValue.ModelName = existing.ModelName
				if existing.ModelName != "" {
					plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
					if err != nil {
						return err
					}
					limits, limitKey, limit, matched, err := getSubscriptionModelLimitMatch(plan, existing.ModelName)
					if err != nil {
						return err
					}
					returnValue.ModelLimitMatched = matched
					returnValue.ModelAmountLimit = limit
					if matched {
						used, err := getSubscriptionModelLimitUsedTx(tx, sub.Id, limits, limitKey)
						if err != nil {
							return err
						}
						returnValue.ModelAmountUsedBefore = used
						returnValue.ModelAmountUsedAfter = used
					}
				}
				return nil
			}

			var subs []UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
				Order("end_time asc, id asc").
				Find(&subs).Error; err != nil {
				return ErrNoActiveSubscription
			}
			if len(subs) == 0 {
				return ErrNoActiveSubscription
			}
			modelQuotaInsufficient := false
			for _, candidate := range subs {
				sub := candidate
				plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
				if err != nil {
					return err
				}
				if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
					return err
				}
				usedBefore := sub.AmountUsed
				if sub.AmountTotal > 0 {
					remain := sub.AmountTotal - usedBefore
					if remain < amount {
						continue
					}
					if usedBefore > sub.AmountTotal {
						continue
					}
				}
				var modelLimitMatched bool
				var modelAmountLimit int64
				var modelUsedBefore int64
				var modelUsedAfter int64
				var modelUsage *UserSubscriptionModelUsage
				if modelName != "" {
					var limits map[string]int64
					var modelLimitKey string
					limits, modelLimitKey, modelAmountLimit, modelLimitMatched, err = getSubscriptionModelLimitMatch(plan, modelName)
					if err != nil {
						return err
					}
					modelUsage, err = ensureSubscriptionModelUsageTx(tx, &sub, modelName)
					if err != nil {
						return err
					}
					if modelLimitMatched {
						modelUsedBefore, err = getSubscriptionModelLimitUsedTx(tx, sub.Id, limits, modelLimitKey)
						if err != nil {
							return err
						}
					} else {
						modelUsedBefore = modelUsage.AmountUsed
					}
					modelUsedAfter = modelUsedBefore + amount
					if modelLimitMatched && modelUsedAfter > modelAmountLimit {
						modelQuotaInsufficient = true
						continue
					}
				}
				record := &SubscriptionPreConsumeRecord{
					RequestId:          requestId,
					UserId:             userId,
					UserSubscriptionId: sub.Id,
					ModelName:          modelName,
					PreConsumed:        amount,
					Status:             "consumed",
				}
				if err := tx.Create(record).Error; err != nil {
					var dup SubscriptionPreConsumeRecord
					if err2 := tx.Where("request_id = ?", requestId).First(&dup).Error; err2 == nil {
						if dup.Status == "refunded" {
							return errors.New("subscription pre-consume already refunded")
						}
						returnValue.UserSubscriptionId = dup.UserSubscriptionId
						returnValue.PreConsumed = dup.PreConsumed
						returnValue.AmountTotal = sub.AmountTotal
						returnValue.AmountUsedBefore = sub.AmountUsed
						returnValue.AmountUsedAfter = sub.AmountUsed
						returnValue.ModelName = dup.ModelName
						returnValue.ModelLimitMatched = modelLimitMatched
						returnValue.ModelAmountLimit = modelAmountLimit
						returnValue.ModelAmountUsedBefore = modelUsedBefore
						returnValue.ModelAmountUsedAfter = modelUsedAfter
						return nil
					}
					return err
				}
				if amount > 0 {
					sub.AmountUsed += amount
					if err := tx.Save(&sub).Error; err != nil {
						return err
					}
					if modelUsage != nil {
						modelUsage.AmountUsed += amount
						if err := tx.Save(modelUsage).Error; err != nil {
							return err
						}
					}
				}
				returnValue.UserSubscriptionId = sub.Id
				returnValue.PreConsumed = amount
				returnValue.AmountTotal = sub.AmountTotal
				returnValue.AmountUsedBefore = usedBefore
				returnValue.AmountUsedAfter = sub.AmountUsed
				returnValue.ModelName = modelName
				returnValue.ModelLimitMatched = modelLimitMatched
				returnValue.ModelAmountLimit = modelAmountLimit
				returnValue.ModelAmountUsedBefore = modelUsedBefore
				returnValue.ModelAmountUsedAfter = modelUsedAfter
				return nil
			}
			if modelQuotaInsufficient {
				return fmt.Errorf("%w, model=%s need=%d", ErrSubscriptionModelQuotaInsufficient, modelName, amount)
			}
			return fmt.Errorf("%w, need=%d", ErrSubscriptionQuotaInsufficient, amount)
		})
		if err == nil {
			return returnValue, nil
		}
		if !shouldRetrySubscriptionSQLiteTx(err) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return nil, err
}

// RefundSubscriptionPreConsume is idempotent and refunds pre-consumed subscription quota by requestId.
func RefundSubscriptionPreConsume(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return nil
		}
		if record.PreConsumed > 0 {
			if err := postConsumeUserSubscriptionModelDeltaTx(tx, record.UserSubscriptionId, record.ModelName, -record.PreConsumed, false); err != nil {
				return err
			}
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
}

func postConsumeUserSubscriptionModelDeltaTx(tx *gorm.DB, userSubscriptionId int, modelName string, delta int64, enforceLimit bool) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	var sub UserSubscription
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("id = ?", userSubscriptionId).
		First(&sub).Error; err != nil {
		return err
	}
	var plan *SubscriptionPlan
	trackModelUsage := normalizedSubscriptionModelName(modelName) != "" && sub.PlanId > 0
	if trackModelUsage {
		var err error
		plan, err = getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			if enforceLimit {
				return err
			}
			trackModelUsage = false
		}
	}
	newUsed := sub.AmountUsed + delta
	if newUsed < 0 {
		newUsed = 0
	}
	if enforceLimit && sub.AmountTotal > 0 && newUsed > sub.AmountTotal {
		return fmt.Errorf("%w, used=%d total=%d", ErrSubscriptionQuotaInsufficient, newUsed, sub.AmountTotal)
	}
	if trackModelUsage {
		_, _, _, _, err := applySubscriptionModelUsageDeltaTx(tx, &sub, plan, modelName, delta, enforceLimit)
		if err != nil {
			return err
		}
	}
	sub.AmountUsed = newUsed
	return tx.Save(&sub).Error
}

func PostConsumeUserSubscriptionModelDelta(userSubscriptionId int, modelName string, delta int64, enforceLimit bool) error {
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return postConsumeUserSubscriptionModelDeltaTx(tx, userSubscriptionId, modelName, delta, enforceLimit)
	})
}

// ResetDueSubscriptions resets subscriptions whose next_reset_time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", now, "active").
		Order("next_reset_time asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	resetCount := 0
	for _, sub := range subs {
		subCopy := sub
		plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
		if err != nil || plan == nil {
			continue
		}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND next_reset_time > 0 AND next_reset_time <= ?", subCopy.Id, now).
				First(&locked).Error; err != nil {
				return nil
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now); err != nil {
				return err
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// CleanupSubscriptionPreConsumeRecords removes old idempotency records to keep table small.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ?", cutoff).Delete(&SubscriptionPreConsumeRecord{})
	return res.RowsAffected, res.Error
}

type SubscriptionPlanInfo struct {
	PlanId    int
	PlanTitle string
}

func GetSubscriptionPlanInfoByUserSubscriptionId(userSubscriptionId int) (*SubscriptionPlanInfo, error) {
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	cacheKey := fmt.Sprintf("sub:%d", userSubscriptionId)
	if cached, found, err := getSubscriptionPlanInfoCache().Get(cacheKey); err == nil && found {
		return &cached, nil
	}
	var sub UserSubscription
	if err := DB.Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
	if err != nil {
		return nil, err
	}
	info := &SubscriptionPlanInfo{
		PlanId:    sub.PlanId,
		PlanTitle: plan.Title,
	}
	_ = getSubscriptionPlanInfoCache().SetWithTTL(cacheKey, *info, subscriptionPlanInfoCacheTTL())
	return info, nil
}

// Update subscription used amount by delta (positive consume more, negative refund).
func PostConsumeUserSubscriptionDelta(userSubscriptionId int, delta int64) error {
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	return PostConsumeUserSubscriptionModelDelta(userSubscriptionId, "", delta, false)
}
