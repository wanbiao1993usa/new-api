package controller

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestIsNonEpayPaymentMethodForEpayCallback(t *testing.T) {
	testCases := []struct {
		name            string
		paymentMethod   string
		expectedBlocked bool
	}{
		{name: "stripe", paymentMethod: model.PaymentMethodStripe, expectedBlocked: true},
		{name: "creem", paymentMethod: model.PaymentMethodCreem, expectedBlocked: true},
		{name: "waffo", paymentMethod: model.PaymentMethodWaffo, expectedBlocked: true},
		{name: "waffo pancake", paymentMethod: model.PaymentMethodWaffoPancake, expectedBlocked: true},
		{name: "alipay", paymentMethod: "alipay", expectedBlocked: false},
		{name: "wxpay", paymentMethod: "wxpay", expectedBlocked: false},
		{name: "custom epay type", paymentMethod: "custom1", expectedBlocked: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if actual := isNonEpayPaymentMethodForEpayCallback(tc.paymentMethod); actual != tc.expectedBlocked {
				t.Fatalf("expected blocked=%v, got %v for payment method %q", tc.expectedBlocked, actual, tc.paymentMethod)
			}
		})
	}
}

func TestGetSubscriptionEpayPayMoneyUsesPaymentExchangeRate(t *testing.T) {
	originalPrice := operation_setting.Price
	originalUSDExchangeRate := operation_setting.USDExchangeRate
	t.Cleanup(func() {
		operation_setting.Price = originalPrice
		operation_setting.USDExchangeRate = originalUSDExchangeRate
	})

	operation_setting.Price = 7.3
	operation_setting.USDExchangeRate = 6.9

	actual := getSubscriptionEpayPayMoney(20)
	if math.Abs(actual-146) > 0.000001 {
		t.Fatalf("expected 20 USD to be submitted as 146 CNY, got %v", actual)
	}
}

func TestGetSubscriptionEpayPayMoneyFallsBackToUSDExchangeRate(t *testing.T) {
	originalPrice := operation_setting.Price
	originalUSDExchangeRate := operation_setting.USDExchangeRate
	t.Cleanup(func() {
		operation_setting.Price = originalPrice
		operation_setting.USDExchangeRate = originalUSDExchangeRate
	})

	operation_setting.Price = 0
	operation_setting.USDExchangeRate = 7.2

	actual := getSubscriptionEpayPayMoney(10)
	if math.Abs(actual-72) > 0.000001 {
		t.Fatalf("expected fallback exchange rate to produce 72, got %v", actual)
	}
}
