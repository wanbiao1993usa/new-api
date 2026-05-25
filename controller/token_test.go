package controller

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type tokenAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type tokenPageResponse struct {
	Items []tokenResponseItem `json:"items"`
}

type tokenResponseItem struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Key    string `json:"key"`
	Status int    `json:"status"`
}

type tokenKeyResponse struct {
	Key string `json:"key"`
}

type sqliteColumnInfo struct {
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
}

type legacyToken struct {
	Id                 int    `gorm:"primaryKey"`
	UserId             int    `gorm:"index"`
	Key                string `gorm:"column:key;type:char(48);uniqueIndex"`
	Status             int    `gorm:"default:1"`
	Name               string `gorm:"index"`
	CreatedTime        int64  `gorm:"bigint"`
	AccessedTime       int64  `gorm:"bigint"`
	ExpiredTime        int64  `gorm:"bigint;default:-1"`
	RemainQuota        int    `gorm:"default:0"`
	UnlimitedQuota     bool
	ModelLimitsEnabled bool
	ModelLimits        string  `gorm:"type:text"`
	AllowIps           *string `gorm:"default:''"`
	UsedQuota          int     `gorm:"default:0"`
	Group              string  `gorm:"column:group;default:''"`
	CrossGroupRetry    bool
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (legacyToken) TableName() string {
	return "tokens"
}

func openTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func migrateTokenControllerTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.AutoMigrate(&model.User{}, &model.Token{}); err != nil {
		t.Fatalf("failed to migrate token table: %v", err)
	}
}

func setupTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := openTokenControllerTestDB(t)
	migrateTokenControllerTestDB(t, db)
	return db
}

func openTokenControllerExternalDB(t *testing.T, dialect string, dsn string) (*gorm.DB, *bool) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	common.UsingSQLite = false
	common.UsingMySQL = dialect == "mysql"
	common.UsingPostgreSQL = dialect == "postgres"

	var (
		db  *gorm.DB
		err error
	)
	switch dialect {
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported dialect %q", dialect)
	}
	if err != nil {
		t.Fatalf("failed to open %s db: %v", dialect, err)
	}

	model.DB = db
	model.LOG_DB = db

	if db.Migrator().HasTable("tokens") {
		t.Skipf("refusing to run %s migration compatibility test against external database because tokens table already exists", dialect)
	}

	managedTokensTable := new(bool)

	t.Cleanup(func() {
		if *managedTokensTable && db.Migrator().HasTable("tokens") {
			_ = db.Migrator().DropTable("tokens")
		}
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db, managedTokensTable
}

func seedToken(t *testing.T, db *gorm.DB, userID int, name string, rawKey string) *model.Token {
	t.Helper()
	user := &model.User{
		Id:       userID,
		Username: fmt.Sprintf("token-user-%d", userID),
		Group:    "default",
		Status:   common.UserStatusEnabled,
		AffCode:  fmt.Sprintf("tu%d", userID),
	}
	if err := db.Where("id = ?", userID).FirstOrCreate(user).Error; err != nil {
		t.Fatalf("failed to create token owner: %v", err)
	}

	token := &model.Token{
		UserId:         userID,
		Name:           name,
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}

func withTokenControllerGroupSettings(t *testing.T, usable string, visible string) {
	t.Helper()
	oldUsable := setting.UserUsableGroups2JSONString()
	oldVisible := setting.UserVisibleGroups2JSONString()
	requireNoError := func(err error) {
		if err != nil {
			t.Fatalf("failed to update group settings: %v", err)
		}
	}
	requireNoError(setting.UpdateUserUsableGroupsByJSONString(usable))
	requireNoError(setting.UpdateUserVisibleGroupsByJSONString(visible))
	t.Cleanup(func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(oldUsable)
		_ = setting.UpdateUserVisibleGroupsByJSONString(oldVisible)
	})
}

func newAuthenticatedContext(t *testing.T, method string, target string, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("id", userID)
	return ctx, recorder
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenAPIResponse {
	t.Helper()

	var response tokenAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func getSQLiteColumnType(t *testing.T, db *gorm.DB, tableName string, columnName string) string {
	t.Helper()

	var columns []sqliteColumnInfo
	if err := db.Raw("PRAGMA table_info(" + tableName + ")").Scan(&columns).Error; err != nil {
		t.Fatalf("failed to inspect %s schema: %v", tableName, err)
	}

	for _, column := range columns {
		if column.Name == columnName {
			return strings.ToLower(column.Type)
		}
	}

	t.Fatalf("column %s not found in %s schema", columnName, tableName)
	return ""
}

func getTokenKeyColumnType(t *testing.T, db *gorm.DB, dialect string) string {
	t.Helper()

	switch dialect {
	case "sqlite":
		return getSQLiteColumnType(t, db, "tokens", "key")
	case "mysql":
		var columnType string
		if err := db.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			"tokens", "key").Scan(&columnType).Error; err != nil {
			t.Fatalf("failed to inspect mysql token key column: %v", err)
		}
		return strings.ToLower(columnType)
	case "postgres":
		var dataType string
		var maxLength sql.NullInt64
		if err := db.Raw(`SELECT data_type, character_maximum_length
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			"tokens", "key").Row().Scan(&dataType, &maxLength); err != nil {
			t.Fatalf("failed to inspect postgres token key column: %v", err)
		}
		switch strings.ToLower(dataType) {
		case "character varying":
			return fmt.Sprintf("varchar(%d)", maxLength.Int64)
		case "character":
			return fmt.Sprintf("char(%d)", maxLength.Int64)
		default:
			if maxLength.Valid {
				return fmt.Sprintf("%s(%d)", strings.ToLower(dataType), maxLength.Int64)
			}
			return strings.ToLower(dataType)
		}
	default:
		t.Fatalf("unsupported dialect %q", dialect)
		return ""
	}
}

func runTokenMigrationCompatibilityTest(t *testing.T, db *gorm.DB, dialect string, managedTokensTable *bool) {
	t.Helper()

	legacyKey := strings.Repeat("a", 48)
	longKey := strings.Repeat("b", 64)

	if err := db.AutoMigrate(&legacyToken{}); err != nil {
		t.Fatalf("failed to create legacy token schema: %v", err)
	}
	if managedTokensTable != nil {
		*managedTokensTable = true
	}
	if err := db.Create(&legacyToken{
		UserId:             7,
		Key:                legacyKey,
		Status:             common.TokenStatusEnabled,
		Name:               "legacy-token",
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        100,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           common.GetPointer(""),
		UsedQuota:          0,
		Group:              "default",
		CrossGroupRetry:    false,
	}).Error; err != nil {
		t.Fatalf("failed to seed legacy token row: %v", err)
	}

	if got := getTokenKeyColumnType(t, db, dialect); got != "char(48)" {
		t.Fatalf("expected legacy key column type char(48), got %q", got)
	}

	migrateTokenControllerTestDB(t, db)

	if got := getTokenKeyColumnType(t, db, dialect); got != "varchar(128)" {
		t.Fatalf("expected migrated key column type varchar(128), got %q", got)
	}

	var migratedToken model.Token
	if err := db.First(&migratedToken, "name = ?", "legacy-token").Error; err != nil {
		t.Fatalf("failed to load migrated token row: %v", err)
	}
	if migratedToken.Key != legacyKey {
		t.Fatalf("expected migrated token key %q, got %q", legacyKey, migratedToken.Key)
	}
	if migratedToken.Name != "legacy-token" {
		t.Fatalf("expected migrated token name to be preserved, got %q", migratedToken.Name)
	}

	inserted := model.Token{
		UserId:             8,
		Name:               "long-token",
		Key:                longKey,
		Status:             common.TokenStatusEnabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        200,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           common.GetPointer(""),
		UsedQuota:          0,
		Group:              "default",
		CrossGroupRetry:    false,
	}
	if err := db.Create(&inserted).Error; err != nil {
		t.Fatalf("failed to insert long token after migration: %v", err)
	}

	var fetched model.Token
	if err := db.First(&fetched, "id = ?", inserted.Id).Error; err != nil {
		t.Fatalf("failed to fetch long token after migration: %v", err)
	}
	if fetched.Key != longKey {
		t.Fatalf("expected long token key %q, got %q", longKey, fetched.Key)
	}
}

func TestTokenAutoMigrateUsesVarchar128KeyColumn(t *testing.T) {
	db := setupTokenControllerTestDB(t)

	if got := getTokenKeyColumnType(t, db, "sqlite"); got != "varchar(128)" {
		t.Fatalf("expected key column type varchar(128), got %q", got)
	}
}

func TestTokenMigrationFromChar48ToVarchar128(t *testing.T) {
	db := openTokenControllerTestDB(t)
	runTokenMigrationCompatibilityTest(t, db, "sqlite", nil)
}

func TestTokenMigrationFromChar48ToVarchar128MySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run mysql migration compatibility test")
	}

	db, managedTokensTable := openTokenControllerExternalDB(t, "mysql", dsn)
	runTokenMigrationCompatibilityTest(t, db, "mysql", managedTokensTable)
}

func TestTokenMigrationFromChar48ToVarchar128Postgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run postgres migration compatibility test")
	}

	db, managedTokensTable := openTokenControllerExternalDB(t, "postgres", dsn)
	runTokenMigrationCompatibilityTest(t, db, "postgres", managedTokensTable)
}

func TestGetAllTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "list-token", "abcd1234efgh5678")
	seedToken(t, db, 2, "other-user-token", "zzzz1234yyyy5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode token page response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one token, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("list response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestSearchTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "searchable-token", "ijkl1234mnop5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/search?keyword=searchable-token&p=1&size=10", nil, 1)
	SearchTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one search result, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked search key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("search response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "detail-token", "qrst1234uvwx5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token detail response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked detail key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("detail response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestUpdateTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "editable-token", "yzab1234cdef5678")

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "updated-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token update response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked update key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("update response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestAddTokenRejectsHiddenButAccessibleGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	withTokenControllerGroupSettings(t, `{"default":"Default","vip":"VIP"}`, `{"vip":"VIP"}`)
	if err := db.Create(&model.User{Id: 10, Username: "visible-user", Group: "vip", Status: common.UserStatusEnabled, AffCode: "tu10"}).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	body := map[string]any{
		"name":                 "hidden-group-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 10)
	ctx.Set("group", "vip")
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected hidden group token creation to fail")
	}
	if !strings.Contains(response.Message, "无权创建或编辑 default 分组令牌") ||
		!strings.Contains(response.Message, "请退出账号后重新登录再试") {
		t.Fatalf("unexpected error message: %s", response.Message)
	}
}

func TestAddTokenRejectsEmptyGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	withTokenControllerGroupSettings(t, `{"default":"Default"}`, `{"default":"Default"}`)
	if err := db.Create(&model.User{Id: 13, Username: "group-required-user", Group: "default", Status: common.UserStatusEnabled, AffCode: "tu13"}).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	body := map[string]any{
		"name":                 "empty-group-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 13)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected empty group token creation to fail")
	}
	if !strings.Contains(response.Message, "请选择分组") {
		t.Fatalf("unexpected error message: %s", response.Message)
	}

	var count int64
	if err := db.Model(&model.Token{}).Where("user_id = ?", 13).Count(&count).Error; err != nil {
		t.Fatalf("failed to count tokens: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no token to be created, got %d", count)
	}
}

func TestAddTokenAllowsSelectedGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	withTokenControllerGroupSettings(t, `{"default":"Default"}`, `{"default":"Default"}`)
	if err := db.Create(&model.User{Id: 14, Username: "selected-group-user", Group: "default", Status: common.UserStatusEnabled, AffCode: "tu14"}).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	body := map[string]any{
		"name":                 "selected-group-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 14)
	ctx.Set("group", "default")
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected selected group token creation to succeed, got message: %s", response.Message)
	}

	var token model.Token
	if err := db.Where("user_id = ?", 14).First(&token).Error; err != nil {
		t.Fatalf("failed to get created token: %v", err)
	}
	if token.Group != "default" {
		t.Fatalf("expected created token group default, got %q", token.Group)
	}
}

func TestUpdateTokenAllowsKeepingEmptyGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 15, "empty-group-legacy-token", "empty1234token5678")
	if err := db.Model(token).Update("group", "").Error; err != nil {
		t.Fatalf("failed to make token group empty: %v", err)
	}

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "empty-group-legacy-token-updated",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 15)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected keeping empty group to succeed, got message: %s", response.Message)
	}

	var updated model.Token
	if err := db.First(&updated, token.Id).Error; err != nil {
		t.Fatalf("failed to get updated token: %v", err)
	}
	if updated.Group != "" {
		t.Fatalf("expected token group to remain empty, got %q", updated.Group)
	}
}

func TestUpdateTokenAllowsKeepingHiddenAccessibleGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	withTokenControllerGroupSettings(t, `{"default":"Default","vip":"VIP"}`, `{"vip":"VIP"}`)
	token := seedToken(t, db, 11, "legacy-token", "legacy1234token5678")
	if err := db.Model(&model.User{}).Where("id = ?", 11).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update user group: %v", err)
	}

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "legacy-token-updated",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 11)
	ctx.Set("group", "vip")
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected keeping hidden accessible group to succeed, got message: %s", response.Message)
	}
}

func TestUpdateTokenRejectsChangingToHiddenAccessibleGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	withTokenControllerGroupSettings(t, `{"default":"Default","vip":"VIP"}`, `{"vip":"VIP"}`)
	token := seedToken(t, db, 12, "vip-token", "vip1234token5678")
	if err := db.Model(&model.User{}).Where("id = ?", 12).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update user group: %v", err)
	}
	token.Group = "vip"
	if err := db.Save(token).Error; err != nil {
		t.Fatalf("failed to update token group: %v", err)
	}

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "vip-token-updated",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 12)
	ctx.Set("group", "vip")
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected changing to hidden group to fail")
	}
	if !strings.Contains(response.Message, "无权创建或编辑 default 分组令牌") ||
		!strings.Contains(response.Message, "请退出账号后重新登录再试") {
		t.Fatalf("unexpected error message: %s", response.Message)
	}
}

func TestGetTokenKeyRequiresOwnershipAndReturnsFullKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")

	authorizedCtx, authorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	authorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(authorizedCtx)

	authorizedResponse := decodeAPIResponse(t, authorizedRecorder)
	if !authorizedResponse.Success {
		t.Fatalf("expected authorized key fetch to succeed, got message: %s", authorizedResponse.Message)
	}

	var keyData tokenKeyResponse
	if err := common.Unmarshal(authorizedResponse.Data, &keyData); err != nil {
		t.Fatalf("failed to decode token key response: %v", err)
	}
	if keyData.Key != token.GetFullKey() {
		t.Fatalf("expected full key %q, got %q", token.GetFullKey(), keyData.Key)
	}

	unauthorizedCtx, unauthorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 2)
	unauthorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(unauthorizedCtx)

	unauthorizedResponse := decodeAPIResponse(t, unauthorizedRecorder)
	if unauthorizedResponse.Success {
		t.Fatalf("expected unauthorized key fetch to fail")
	}
	if strings.Contains(unauthorizedRecorder.Body.String(), token.Key) {
		t.Fatalf("unauthorized key response leaked raw token key: %s", unauthorizedRecorder.Body.String())
	}
}
