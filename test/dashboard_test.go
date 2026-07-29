package test

import (
	"encoding/json"
	"expo-open-ota/config"
	"expo-open-ota/internal/auth"
	"expo-open-ota/internal/bucket"
	"expo-open-ota/internal/handlers"
	infrastructure "expo-open-ota/internal/router"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestLoginDashboardNotEnabled(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	os.Setenv("USE_DASHBOARD", "false")
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", nil)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusNotFound, respRec.Code)

}

func TestLoginInvalidPassword(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	formData := url.Values{}
	formData.Set("password", "wrongpassword")
	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

func TestShouldRejectLoginIfAdminPasswordNotSet(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	os.Setenv("ADMIN_PASSWORD", "")
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	formData := url.Values{}
	formData.Set("password", "admin")
	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

func TestLoginValidPassword(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	formData := url.Values{}
	formData.Set("password", "admin")
	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusOK, respRec.Code)
	// Retrieve token & refreshToken from response
	body := respRec.Body.String()

	var response auth.AuthResponse
	err := json.Unmarshal([]byte(body), &response)
	assert.Nil(t, err)
	assert.NotEmpty(t, response.Token)
	assert.NotEmpty(t, response.RefreshToken)
}

func login() auth.AuthResponse {
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	formData := url.Values{}
	formData.Set("password", "admin")
	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(respRec, req)
	body := respRec.Body.String()
	var response auth.AuthResponse
	_ = json.Unmarshal([]byte(body), &response)
	return response
}

func TestRefreshToken(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	formData := url.Values{}
	formData.Set("refreshToken", login().RefreshToken)
	req, _ := http.NewRequest("POST", "/auth/refreshToken", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusOK, respRec.Code)
	body := respRec.Body.String()
	var response auth.AuthResponse
	err := json.Unmarshal([]byte(body), &response)
	assert.Nil(t, err)
	assert.NotEmpty(t, response.Token)
	assert.NotEmpty(t, response.RefreshToken)
}

func TestRefreshTokenInvalidToken(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	formData := url.Values{}
	formData.Set("refreshToken", "invalid-or-expired-token")
	req, _ := http.NewRequest("POST", "/auth/refreshToken", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

func TestSettings(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+login().Token)
	router.ServeHTTP(respRec, req)

	assert.Equal(t, http.StatusOK, respRec.Code)

	projectRoot, err := os.Getwd()
	assert.Nil(t, err)

	responseBody := strings.TrimSpace(string(respRec.Body.Bytes()))

	responseBody = strings.ReplaceAll(responseBody, projectRoot+"/test-updates", "{PROJECT_ROOT}/test/test-updates")
	responseBody = strings.ReplaceAll(responseBody, projectRoot+"/keys/public-key-test.pem", "{PROJECT_ROOT}/test/keys/public-key-test.pem")
	responseBody = strings.ReplaceAll(responseBody, projectRoot+"/keys/private-key-test.pem", "{PROJECT_ROOT}/test/keys/private-key-test.pem")

	expectedSnapshot := `{"BASE_URL":"http://localhost:3000","CACHE_MODE":"","REDIS_HOST":"","REDIS_PORT":"","REDIS_SENTINEL_ADDRS":"","REDIS_SENTINEL_MASTER_NAME":"","STORAGE_MODE":"local","S3_BUCKET_NAME":"","LOCAL_BUCKET_BASE_PATH":"{PROJECT_ROOT}/test/test-updates","AWS_REGION":"eu-west-3","AWS_BASE_ENDPOINT":"","AWS_ACCESS_KEY_ID":"***","CLOUDFRONT_DOMAIN":"","CLOUDFRONT_KEY_PAIR_ID":"***","CLOUDFRONT_PRIVATE_KEY_B64":"***","AWSSM_CLOUDFRONT_PRIVATE_KEY_SECRET_ID":"","PRIVATE_LOCAL_CLOUDFRONT_KEY_PATH":"","PROMETHEUS_ENABLED":"","APPS":[{"id":"test-app-id"}]}`

	assert.Equal(t, expectedSnapshot, responseBody)
}

func TestSettingsWithoutAuth(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/settings", nil)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

func TestBranches(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branches", nil)
	req.Header.Set("Authorization", "Bearer "+login().Token)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusOK, respRec.Code)

	var response []handlers.BranchMapping
	err := json.Unmarshal(respRec.Body.Bytes(), &response)
	assert.Nil(t, err)
	// Branch list comes from the bucket; only branches pointed at by a
	// configured channel get a branchId/releaseChannel (config-based mapping).
	assert.Equal(t, `[{"branchName":"branch-1","branchId":"branch-1","releaseChannel":"staging"},{"branchName":"branch-2","branchId":null,"releaseChannel":null},{"branchName":"branch-3","branchId":null,"releaseChannel":null},{"branchName":"branch-4","branchId":null,"releaseChannel":null}]`, strings.TrimSpace(string(respRec.Body.Bytes())))
}

func TestBranchesWithoutAuth(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branches", nil)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

// The dashboard /api/apps/{APP_ID}/... routes must run AppResolverMiddleware
// so unknown app ids return 404 — without it handlers fall through to
// bucket lookups and can answer 200 with [] for a nonexistent app.
func TestDashboardUnknownAppIdReturns404(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/does-not-exist/branches", nil)
	req.Header.Set("Authorization", "Bearer "+login().Token)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusNotFound, respRec.Code)
}

// AuthMiddleware must reject a syntactically valid but cryptographically bogus
// JWT. The "no header" path is covered by TestSettingsWithoutAuth et al; this
// closes the gap where a malicious caller sends garbage in the Bearer slot.
func TestDashboardRejectsInvalidBearerToken(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer not.a.real.jwt")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

// Use-Expo-Auth relays the caller's Expo credentials on app-scoped routes;
// on app-agnostic routes (here /api/settings) there is no APP_ID to validate
// against, so the middleware must short-circuit with 401 instead of falling
// through to an Expo call it cannot make.
func TestDashboardUseExpoAuthRejectedOnAppAgnosticRoute(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/settings", nil)
	req.Header.Set("Use-Expo-Auth", "true")
	req.Header.Set("Authorization", "Bearer expo_test_token")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

// Use-Expo-Auth with a bearer the Expo API rejects must surface as a 401
// from the middleware. Exercises the ValidateExpoAuth failure branch for
// the dashboard-relayed Expo session path.
func TestDashboardUseExpoAuthRejectsInvalidExpoToken(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branches", nil)
	req.Header.Set("Use-Expo-Auth", "true")
	req.Header.Set("Authorization", "Bearer bogus_expo_token")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

// TestDashboardUseExpoAuthCrossAppAttackRejected — the promise of
// Use-Expo-Auth is that a caller can only reach an app whose stored
// EXPO_ACCESS_TOKEN resolves to the same Expo user as their session.
// If two tenants coexist on the server, a caller with a valid Expo
// token for tenant A must NOT be able to read tenant B via
// /api/apps/{B}/... The middleware enforces this by calling Expo with
// BOTH tokens and comparing the returned usernames — a mismatch is
// a 401, no 500, no fall-through to the handler.
func TestDashboardUseExpoAuthCrossAppAttackRejected(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	// Override the default single-app fixture with two apps holding distinct
	// publish tokens. The per-app publishTokens ARE the tenant boundary:
	// app-1's token must never validate against app-2.
	appsJSON := `[
      {"id":"app-1","publishTokens":["token-app-1"],"keys":{"mode":"local","publicPath":"/a","privatePath":"/b"}},
      {"id":"app-2","publishTokens":["token-app-2"],"keys":{"mode":"local","publicPath":"/a","privatePath":"/b"}}
    ]`
	os.Setenv("EXPO_APPS_JSON", appsJSON)
	config.ResetAppsForTest()
	if err := config.LoadApps(); err != nil {
		t.Fatalf("LoadApps: %v", err)
	}

	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	// Caller holds app-1's publish token, but hits /api/apps/app-2/…
	req, _ := http.NewRequest("GET", "/api/apps/app-2/branches", nil)
	req.Header.Set("Use-Expo-Auth", "true")
	req.Header.Set("Authorization", "Bearer token-app-1")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

// Use-Expo-Auth happy path: caller's Expo token resolves to the same username
// as the app's EXPO_ACCESS_TOKEN (ValidateExpoAuth's match check) so the
// middleware lets the request through to the handler.
func TestDashboardUseExpoAuthHappyPath(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branches", nil)
	req.Header.Set("Use-Expo-Auth", "true")
	req.Header.Set("Authorization", "Bearer expo_test_token")
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusOK, respRec.Code)
}

func TestRuntimeVersionsWithoutAuth(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branch/branch-1/runtimeVersions", nil)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

func TestRuntimeVersions(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branch/branch-1/runtimeVersions", nil)
	req.Header.Set("Authorization", "Bearer "+login().Token)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusOK, respRec.Code)
	var response []bucket.RuntimeVersionWithStats
	err := json.Unmarshal(respRec.Body.Bytes(), &response)
	assert.Nil(t, err)
	assert.Equal(t, "[{\"runtimeVersion\":\"1\",\"lastUpdatedAt\":\"1970-01-20T09:02:50Z\",\"createdAt\":\"1970-01-20T09:02:50Z\",\"numberOfUpdates\":1}]", strings.TrimSpace(string(respRec.Body.Bytes())))
}

func TestUpdatesWithoutAuth(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branch/branch-1/runtimeVersion/1/updates", nil)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusUnauthorized, respRec.Code)
}

func TestUpdatesRegularBranch1(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branch/branch-1/runtimeVersion/1/updates", nil)
	req.Header.Set("Authorization", "Bearer "+login().Token)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusOK, respRec.Code)
	assert.Equal(t, "[{\"updateUUID\":\"04b793a0-b6ab-fd4f-308c-b91d812adec2\",\"updateId\":\"1674170951\",\"createdAt\":\"1970-01-20T09:02:50Z\",\"commitHash\":\"1674170951\",\"platform\":\"android\"}]", strings.TrimSpace(string(respRec.Body.Bytes())))
}

func TestUpdatesMultiBranch2(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branch/branch-2/runtimeVersion/1/updates", nil)
	req.Header.Set("Authorization", "Bearer "+login().Token)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusOK, respRec.Code)
	assert.Equal(t, "[{\"updateUUID\":\"68e096e2-a619-9d56-7f7c-89f97bc27312\",\"updateId\":\"1737455526\",\"createdAt\":\"1970-01-21T02:37:35Z\",\"commitHash\":\"\",\"platform\":\"ios\"},{\"updateUUID\":\"fdc14544-9e15-732f-cd9c-e3e26c55cbea\",\"updateId\":\"1674170951\",\"createdAt\":\"1970-01-20T09:02:50Z\",\"commitHash\":\"\",\"platform\":\"android\"},{\"updateUUID\":\"d100f19f-e0be-45c4-212a-27d1f067552b\",\"updateId\":\"1666629107\",\"createdAt\":\"1970-01-20T06:57:09Z\",\"commitHash\":\"1674170951\",\"platform\":\"android\"},{\"updateUUID\":\"Rollback to embedded\",\"updateId\":\"1666629141\",\"createdAt\":\"1970-01-20T06:57:09Z\",\"commitHash\":\"1674170951\",\"platform\":\"ios\"},{\"updateUUID\":\"Rollback to embedded\",\"updateId\":\"1666304169\",\"createdAt\":\"1970-01-20T06:51:44Z\",\"commitHash\":\"1674170951\",\"platform\":\"ios\"}]", strings.TrimSpace(string(respRec.Body.Bytes())))
}

func TestUpdatesSomeNotValidBranch4(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/apps/test-app-id/branch/branch-4/runtimeVersion/1/updates", nil)
	req.Header.Set("Authorization", "Bearer "+login().Token)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusOK, respRec.Code)
	assert.Equal(t, "[{\"updateUUID\":\"3f23a8c4-cd0e-a5a4-63f2-bb2841e95a01\",\"updateId\":\"1674170951\",\"createdAt\":\"1970-01-20T09:02:50Z\",\"commitHash\":\"1674170951\",\"platform\":\"android\"}]", strings.TrimSpace(string(respRec.Body.Bytes())))
}

// Channel mapping lives in EXPO_APPS_JSON now — the dashboard edit endpoint
// must fail loudly (500) instead of pretending to save.
func TestUpdateChannelBranchMappingIsRejected(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	router := infrastructure.NewRouter()
	respRec := httptest.NewRecorder()
	body := `{"releaseChannel":"staging"}`
	req, _ := http.NewRequest("POST", "/api/apps/test-app-id/branch/branch-1/updateChannelBranchMapping", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+login().Token)
	router.ServeHTTP(respRec, req)
	assert.Equal(t, http.StatusInternalServerError, respRec.Code)
}
