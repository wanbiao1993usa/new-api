package ratio_setting

import "testing"

func TestGetModelPriceFallsBackFromCompactAliasToBaseModel(t *testing.T) {
	resetRatioMaps(t)
	modelPriceMap.Set("gpt-5.5", 5)

	price, ok := GetModelPrice(WithCompactModelSuffix("gpt-5.5"), false)
	if !ok {
		t.Fatal("expected compact alias to use base model price")
	}
	if price != 5 {
		t.Fatalf("expected base model price 5, got %f", price)
	}
}

func TestGetModelPriceUsesCompactWildcardBeforeBaseModel(t *testing.T) {
	resetRatioMaps(t)
	modelPriceMap.Set("gpt-5.5", 5)
	modelPriceMap.Set(CompactWildcardModelKey, 2)

	price, ok := GetModelPrice(WithCompactModelSuffix("gpt-5.5"), false)
	if !ok {
		t.Fatal("expected compact alias to use compact wildcard price")
	}
	if price != 2 {
		t.Fatalf("expected compact wildcard price 2, got %f", price)
	}
}

func TestGetModelRatioFallsBackFromCompactAliasToBaseModel(t *testing.T) {
	resetRatioMaps(t)
	modelRatioMap.Set("gpt-5.5", 2.5)

	ratio, ok, matchName := GetModelRatio(WithCompactModelSuffix("gpt-5.5"))
	if !ok {
		t.Fatal("expected compact alias to use base model ratio")
	}
	if ratio != 2.5 {
		t.Fatalf("expected base model ratio 2.5, got %f", ratio)
	}
	if matchName != "gpt-5.5" {
		t.Fatalf("expected base match name, got %q", matchName)
	}
}

func TestCompactAliasFallsBackToBaseCacheRatios(t *testing.T) {
	resetRatioMaps(t)
	cacheRatioMap.Set("gpt-5.5", 0.1)
	createCacheRatioMap.Set("gpt-5.5", 1.25)

	cacheRatio, ok := GetCacheRatio(WithCompactModelSuffix("gpt-5.5"))
	if !ok || cacheRatio != 0.1 {
		t.Fatalf("expected base cache ratio 0.1, got ratio=%f ok=%v", cacheRatio, ok)
	}

	createCacheRatio, ok := GetCreateCacheRatio(WithCompactModelSuffix("gpt-5.5"))
	if !ok || createCacheRatio != 1.25 {
		t.Fatalf("expected base create cache ratio 1.25, got ratio=%f ok=%v", createCacheRatio, ok)
	}
}

func resetRatioMaps(t *testing.T) {
	t.Helper()
	modelPriceBackup := modelPriceMap.ReadAll()
	modelRatioBackup := modelRatioMap.ReadAll()
	cacheRatioBackup := cacheRatioMap.ReadAll()
	createCacheRatioBackup := createCacheRatioMap.ReadAll()

	modelPriceMap.Clear()
	modelRatioMap.Clear()
	cacheRatioMap.Clear()
	createCacheRatioMap.Clear()

	t.Cleanup(func() {
		modelPriceMap.Clear()
		modelPriceMap.AddAll(modelPriceBackup)
		modelRatioMap.Clear()
		modelRatioMap.AddAll(modelRatioBackup)
		cacheRatioMap.Clear()
		cacheRatioMap.AddAll(cacheRatioBackup)
		createCacheRatioMap.Clear()
		createCacheRatioMap.AddAll(createCacheRatioBackup)
	})
}
