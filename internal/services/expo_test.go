package services

import (
	"expo-open-ota/config"
	"expo-open-ota/internal/types"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupApps(t *testing.T) {
	t.Helper()
	appsJSON := `[
      {"id":"app-1","publishTokens":["tok-a","tok-b"],
       "channels":{"production":"main"},
       "keys":{"mode":"local","publicPath":"/p","privatePath":"/q"}}
    ]`
	os.Setenv("EXPO_APPS_JSON", appsJSON)
	config.ResetAppsForTest()
	if err := config.LoadApps(); err != nil {
		t.Fatalf("LoadApps: %v", err)
	}
	t.Cleanup(func() {
		os.Unsetenv("EXPO_APPS_JSON")
		config.ResetAppsForTest()
	})
}

func strPtr(s string) *string { return &s }

func TestValidateExpoAuthAcceptsAnyConfiguredToken(t *testing.T) {
	setupApps(t)
	account, err := ValidateExpoAuth("app-1", types.ExpoAuth{Token: strPtr("tok-a")})
	assert.NoError(t, err)
	assert.Equal(t, "app-1", account.Username)
	_, err = ValidateExpoAuth("app-1", types.ExpoAuth{Token: strPtr("tok-b")})
	assert.NoError(t, err)
}

func TestValidateExpoAuthRejectsWrongPartialOrMissingToken(t *testing.T) {
	setupApps(t)
	_, err := ValidateExpoAuth("app-1", types.ExpoAuth{Token: strPtr("tok-c")})
	assert.Error(t, err)
	_, err = ValidateExpoAuth("app-1", types.ExpoAuth{})
	assert.Error(t, err)
	// 부분 일치(접두/접미)도 거부 — 비교는 전체 문자열 단위
	_, err = ValidateExpoAuth("app-1", types.ExpoAuth{Token: strPtr("tok-")})
	assert.Error(t, err)
	_, err = ValidateExpoAuth("app-1", types.ExpoAuth{Token: strPtr("tok-a ")})
	assert.Error(t, err)
}

func TestValidateExpoAuthRejectsSessionSecret(t *testing.T) {
	setupApps(t)
	_, err := ValidateExpoAuth("app-1", types.ExpoAuth{SessionSecret: strPtr("sess")})
	assert.Error(t, err)
}

func TestValidateExpoAuthRejectsUnknownApp(t *testing.T) {
	setupApps(t)
	_, err := ValidateExpoAuth("app-2", types.ExpoAuth{Token: strPtr("tok-a")})
	assert.Error(t, err)
}

func TestFetchSelfExpoUsernameReturnsAppId(t *testing.T) {
	setupApps(t)
	assert.Equal(t, "app-1", FetchSelfExpoUsername("app-1"))
}

func TestFetchExpoChannelMappingUsesConfigThenFallback(t *testing.T) {
	setupApps(t)
	m, err := FetchExpoChannelMapping("app-1", "production")
	assert.NoError(t, err)
	assert.Equal(t, "main", m.BranchName) // config 매핑
	m, err = FetchExpoChannelMapping("app-1", "staging")
	assert.NoError(t, err)
	assert.Equal(t, "staging", m.BranchName) // channel==branch 폴백
	_, err = FetchExpoChannelMapping("app-1", "")
	assert.Error(t, err)
}

func TestFetchExpoChannelsAndBranchesMappingFromConfig(t *testing.T) {
	setupApps(t)
	channels, err := FetchExpoChannels("app-1")
	assert.NoError(t, err)
	assert.Equal(t, []ExpoChannel{{Id: "production", Name: "production", BranchId: "main"}}, channels)
	mappings, err := FetchExpoBranchesMapping("app-1")
	assert.NoError(t, err)
	if assert.Len(t, mappings, 1) {
		assert.Equal(t, "main", mappings[0].BranchName)
		assert.Equal(t, "main", mappings[0].BranchId)
		assert.Equal(t, "production", *mappings[0].ChannelName)
	}
}

func TestBranchOpsAreLocalNoops(t *testing.T) {
	setupApps(t)
	branches, err := FetchExpoBranches("app-1")
	assert.NoError(t, err)
	assert.Empty(t, branches)                                                   // UpsertBranch가 CreateBranch로 진행
	assert.NoError(t, CreateBranch("app-1", "any"))                             // no-op
	assert.Error(t, UpdateChannelBranchMapping("app-1", "production", "other")) // config로만 관리
}

// An app with no publishTokens is read-only: every publish auth attempt
// fails, but the app still boots and serves manifests/assets.
func TestValidateExpoAuthRejectsAllWhenNoTokensConfigured(t *testing.T) {
	os.Setenv("EXPO_APPS_JSON", `[
      {"id":"ro-app","keys":{"mode":"local","publicPath":"/p","privatePath":"/q"}}
    ]`)
	config.ResetAppsForTest()
	if err := config.LoadApps(); err != nil {
		t.Fatalf("LoadApps: %v", err)
	}
	t.Cleanup(func() {
		os.Unsetenv("EXPO_APPS_JSON")
		config.ResetAppsForTest()
	})
	_, err := ValidateExpoAuth("ro-app", types.ExpoAuth{Token: strPtr("anything")})
	assert.Error(t, err)
}
