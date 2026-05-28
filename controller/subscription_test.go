package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func setupSubscriptionControllerTestDB(t *testing.T) {
	t.Helper()

	db := openTokenControllerTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.SubscriptionPlan{}, &model.UserSubscription{}); err != nil {
		t.Fatalf("failed to migrate subscription plan table: %v", err)
	}
}

func seedSubscriptionControllerUser(t *testing.T, id int) {
	t.Helper()

	user := &model.User{
		Id:       id,
		Username: "subscription-controller-user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "subscription-controller-user",
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
}

func seedSubscriptionControllerPlan(t *testing.T, id int, title string, enabled bool, userVisible bool) {
	t.Helper()

	plan := &model.SubscriptionPlan{
		Id:            id,
		Title:         title,
		PriceAmount:   1,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       enabled,
		UserVisible:   userVisible,
		TotalAmount:   1000,
	}
	if err := model.DB.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed subscription plan: %v", err)
	}
	if err := model.DB.Model(&model.SubscriptionPlan{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"enabled": enabled, "user_visible": userVisible}).Error; err != nil {
		t.Fatalf("failed to set subscription plan booleans: %v", err)
	}
	model.InvalidateSubscriptionPlanCache(id)
}

func seedSubscriptionControllerActiveSubscription(t *testing.T, id int, userId int, planId int) {
	t.Helper()

	now := common.GetTimestamp()
	subscription := &model.UserSubscription{
		Id:          id,
		UserId:      userId,
		PlanId:      planId,
		AmountTotal: 1000,
		AmountUsed:  0,
		StartTime:   now - 60,
		EndTime:     now + 3600,
		Status:      "active",
		Source:      "admin",
	}
	if err := model.DB.Create(subscription).Error; err != nil {
		t.Fatalf("failed to seed user subscription: %v", err)
	}
}

func TestAdminCreateSubscriptionPlanPreservesExplicitFalseBooleans(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/admin/plans", map[string]any{
		"id": 123,
		"plan": map[string]any{
			"title":          "hidden disabled plan",
			"price_amount":   1,
			"currency":       "USD",
			"duration_unit":  model.SubscriptionDurationMonth,
			"duration_value": 1,
			"enabled":        false,
			"user_visible":   false,
			"total_amount":   1000,
		},
	}, 1)

	AdminCreateSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected create success, got message: %s", response.Message)
	}
	var created model.SubscriptionPlan
	if err := common.Unmarshal(response.Data, &created); err != nil {
		t.Fatalf("failed to decode created plan: %v", err)
	}

	var stored model.SubscriptionPlan
	if err := model.DB.Where("id = ?", created.Id).First(&stored).Error; err != nil {
		t.Fatalf("failed to load created plan: %v", err)
	}
	if stored.Enabled {
		t.Fatalf("expected enabled=false to persist")
	}
	if stored.UserVisible {
		t.Fatalf("expected user_visible=false to persist")
	}
}

func TestAdminCreateSubscriptionPlanDefaultsBooleansToTrueWhenOmitted(t *testing.T) {
	setupSubscriptionControllerTestDB(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/admin/plans", map[string]any{
		"plan": map[string]any{
			"title":          "default visible enabled plan",
			"price_amount":   1,
			"currency":       "USD",
			"duration_unit":  model.SubscriptionDurationMonth,
			"duration_value": 1,
			"total_amount":   1000,
		},
	}, 1)

	AdminCreateSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected create success, got message: %s", response.Message)
	}
	var created model.SubscriptionPlan
	if err := common.Unmarshal(response.Data, &created); err != nil {
		t.Fatalf("failed to decode created plan: %v", err)
	}

	var stored model.SubscriptionPlan
	if err := model.DB.Where("id = ?", created.Id).First(&stored).Error; err != nil {
		t.Fatalf("failed to load created plan: %v", err)
	}
	if !stored.Enabled {
		t.Fatalf("expected omitted enabled to default true")
	}
	if !stored.UserVisible {
		t.Fatalf("expected omitted user_visible to default true")
	}
}

func TestGetSubscriptionPlansReturnsOnlyEnabledUserVisiblePlans(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	seedSubscriptionControllerUser(t, 1)
	seedSubscriptionControllerPlan(t, 101, "visible plan", true, true)
	seedSubscriptionControllerPlan(t, 102, "hidden plan", true, false)
	seedSubscriptionControllerPlan(t, 103, "disabled plan", false, true)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/subscription/plans", nil, 1)

	GetSubscriptionPlans(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected list success, got message: %s", response.Message)
	}
	var plans []SubscriptionPlanDTO
	if err := common.Unmarshal(response.Data, &plans); err != nil {
		t.Fatalf("failed to decode subscription plans: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 visible plan, got %d", len(plans))
	}
	if plans[0].Plan.Id != 101 {
		t.Fatalf("expected visible plan id 101, got %d", plans[0].Plan.Id)
	}
}

func TestGetSubscriptionSelfReturnsHiddenActiveSubscriptions(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	seedSubscriptionControllerUser(t, 1)
	seedSubscriptionControllerPlan(t, 301, "hidden active plan", true, false)
	seedSubscriptionControllerActiveSubscription(t, 401, 1, 301)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/subscription/self", nil, 1)

	GetSubscriptionSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected self success, got message: %s", response.Message)
	}
	var data struct {
		Subscriptions    []model.SubscriptionSummary `json:"subscriptions"`
		AllSubscriptions []model.SubscriptionSummary `json:"all_subscriptions"`
	}
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode subscription self response: %v", err)
	}
	if len(data.Subscriptions) != 1 {
		t.Fatalf("expected hidden active subscription to stay in active self state, got %d", len(data.Subscriptions))
	}
	if data.Subscriptions[0].Plan == nil || data.Subscriptions[0].Plan.Id != 301 {
		t.Fatalf("expected hidden plan 301 in active self state, got %#v", data.Subscriptions[0].Plan)
	}
	if len(data.AllSubscriptions) != 1 {
		t.Fatalf("expected hidden subscription to stay in all self state, got %d", len(data.AllSubscriptions))
	}
}

func TestSubscriptionPaymentRejectsHiddenPlans(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	seedSubscriptionControllerPlan(t, 501, "hidden checkout plan", true, false)

	tests := []struct {
		name    string
		target  string
		body    map[string]any
		handler func(*gin.Context)
	}{
		{
			name:    "stripe",
			target:  "/api/subscription/stripe/pay",
			body:    map[string]any{"plan_id": 501},
			handler: SubscriptionRequestStripePay,
		},
		{
			name:    "epay",
			target:  "/api/subscription/epay/pay",
			body:    map[string]any{"plan_id": 501, "payment_method": "alipay"},
			handler: SubscriptionRequestEpay,
		},
		{
			name:    "creem",
			target:  "/api/subscription/creem/pay",
			body:    map[string]any{"plan_id": 501},
			handler: SubscriptionRequestCreemPay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodPost, tt.target, tt.body, 1)

			tt.handler(ctx)

			response := decodeAPIResponse(t, recorder)
			if response.Success {
				t.Fatalf("expected hidden plan purchase to fail")
			}
			if !strings.Contains(response.Message, "不可购买") {
				t.Fatalf("expected hidden plan purchase message, got %q", response.Message)
			}
		})
	}
}

func TestAdminUpdateSubscriptionPlanStatusCanToggleUserVisible(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	seedSubscriptionControllerPlan(t, 201, "toggle plan", true, true)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPatch, "/api/subscription/admin/plans/201", map[string]any{
		"user_visible": false,
	}, 1)
	ctx.Params = gin.Params{{Key: "id", Value: "201"}}

	AdminUpdateSubscriptionPlanStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected status update success, got message: %s", response.Message)
	}
	var stored model.SubscriptionPlan
	if err := model.DB.Where("id = ?", 201).First(&stored).Error; err != nil {
		t.Fatalf("failed to load updated plan: %v", err)
	}
	if !stored.Enabled {
		t.Fatalf("expected enabled to stay true")
	}
	if stored.UserVisible {
		t.Fatalf("expected user_visible=false after patch")
	}
}
