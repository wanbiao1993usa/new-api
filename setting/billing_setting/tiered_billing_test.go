package billing_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func TestCompactAliasFallsBackToBaseTieredBillingSettings(t *testing.T) {
	resetBillingSettings(t)
	billingSetting.BillingMode["gpt-5.5"] = BillingModeTieredExpr
	billingSetting.BillingExpr["gpt-5.5"] = "input * 1"

	compactModelName := ratio_setting.WithCompactModelSuffix("gpt-5.5")
	if mode := GetBillingMode(compactModelName); mode != BillingModeTieredExpr {
		t.Fatalf("expected compact alias to use base billing mode, got %q", mode)
	}
	expr, ok := GetBillingExpr(compactModelName)
	if !ok || expr != "input * 1" {
		t.Fatalf("expected compact alias to use base billing expr, got %q ok=%v", expr, ok)
	}
}

func resetBillingSettings(t *testing.T) {
	t.Helper()
	modeBackup := make(map[string]string, len(billingSetting.BillingMode))
	for key, value := range billingSetting.BillingMode {
		modeBackup[key] = value
	}
	exprBackup := make(map[string]string, len(billingSetting.BillingExpr))
	for key, value := range billingSetting.BillingExpr {
		exprBackup[key] = value
	}

	billingSetting.BillingMode = make(map[string]string)
	billingSetting.BillingExpr = make(map[string]string)

	t.Cleanup(func() {
		billingSetting.BillingMode = modeBackup
		billingSetting.BillingExpr = exprBackup
	})
}
