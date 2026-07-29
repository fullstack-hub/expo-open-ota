package test

import (
	"encoding/json"
	"expo-open-ota/config"
	"expo-open-ota/internal/bucket"
	cache2 "expo-open-ota/internal/cache"
	"expo-open-ota/internal/handlers"
	"expo-open-ota/internal/types"
	"expo-open-ota/internal/update"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestNotMappedChannelForManifest: an unmapped channel falls back to
// channel==branch (instead of the old EAS 404), so a channel with no matching
// branch directory resolves to "no update" → NoUpdateAvailable directive on
// protocol v1.
func TestNotMappedChannelForManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	q := "http://localhost:3000/manifest"
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-channel-name", "bad_channel")
	r.Header.Add("expo-app-id", "test-app-id")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "Expected status code 200 with NoUpdateAvailable for an unmapped channel")
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Errorf("Error parsing response: %v", err)
	}
	assert.Equal(t, 1, len(parts), "Expected 1 part in the response")
	directivePart := parts[0]
	assert.Equal(t, true, IsMultipartPartWithName(directivePart, "directive"), "Expected a part with name 'directive'")
	var directive types.RollbackDirective
	if err := json.Unmarshal([]byte(directivePart.Body), &directive); err != nil {
		t.Errorf("Error parsing json body: %v", err)
	}
	assert.Equal(t, "noUpdateAvailable", directive.Type)
}

func TestNotValidProtocolVersionsForManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "test-app-id")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "invalid")
	r.Header.Add("expo-expect-signature", "true")
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 400, w.Code, "Expected status code 400 for an invalid protocole version")
	assert.Equal(t, "Invalid protocol version\n", w.Body.String(), "Expected 'Invalid protocol version' message")
}

func TestNotValidPlatformForManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "bad-platform")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "test-app-id")
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 400, w.Code, "Expected status code 400 for an invalid platform")
	assert.Equal(t, "Invalid platform\n", w.Body.String(), "Expected 'IInvalid platform' message")
}

// TestManifestMissingAppIdHeader covers the "no header at all" branch —
// a v1 client that never learned about expo-app-id must fail cleanly
// with a 400, not crash or resolve to some default app.
func TestManifestMissingAppIdHeader(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	q := "http://localhost:3000/manifest"
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	// No expo-app-id.

	handlers.ManifestHandler(w, r)
	assert.Equal(t, 400, w.Code, "Missing expo-app-id must fail with 400")
}

// TestManifestEmptyAppIdHeader — the header is present but empty. Must
// be treated the same as missing (400) rather than resolving to the
// empty-string app and falling through to an Expo call.
func TestManifestEmptyAppIdHeader(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	q := "http://localhost:3000/manifest"
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "")

	handlers.ManifestHandler(w, r)
	assert.Equal(t, 400, w.Code, "Empty expo-app-id must fail with 400")
}

// TestManifestMalformedAppIdHeader checks the handler rejects values
// that look like path traversal or whitespace-padded ids. Even though
// the registry lookup would 404 them, we want the response to be clean
// (not 500) and not trip any log-injection sensitivities.
func TestManifestMalformedAppIdHeader(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	for _, badId := range []string{"../etc", "a/b", "with\tctrl", "   "} {
		t.Run(badId, func(t *testing.T) {
			q := "http://localhost:3000/manifest"
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", q, nil)
			r.Header.Add("expo-platform", "ios")
			r.Header.Add("expo-runtime-version", "1")
			r.Header.Add("expo-protocol-version", "1")
			r.Header.Add("expo-expect-signature", "true")
			r.Header.Add("expo-channel-name", "staging")
			r.Header.Add("expo-app-id", badId)

			handlers.ManifestHandler(w, r)
			// 400 (malformed) or 404 (not in registry) are both acceptable
			// — the invariant is "no 5xx and no data returned".
			assert.Truef(t, w.Code == 400 || w.Code == 404, "want 400 or 404, got %d", w.Code)
		})
	}
}

// TestUnknownAppIdForManifest locks in the 404-on-unknown-app behaviour so
// we never regress into firing an outbound Expo API call with an empty
// Bearer token — which used to surface as an opaque 500 to the client.
func TestUnknownAppIdForManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	q := "http://localhost:3000/manifest"
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "this-id-is-not-in-apps-json")

	handlers.ManifestHandler(w, r)
	assert.Equal(t, 404, w.Code, "Unknown app id must fail early with 404")
	assert.Equal(t, "Unknown app id\n", w.Body.String())
}

func TestNotValidRuntimeVersionForManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()

	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "test-app-id")

	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 400, w.Code, "Expected status code 400 when runtime version is not provided")
	assert.Equal(t, "No runtime version provided\n", w.Body.String(), "Expected 'No runtime version provided' message")
}

func TestNotValidCertificatesForManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	projectRoot, _ := findProjectRoot()
	os.Setenv("LOCAL_BUCKET_BASE_PATH", filepath.Join(projectRoot, "/test/test-updates"))
	// Override the shared EXPO_APPS_JSON (set by SetValidConfiguration) with
	// paths that point to a missing key file so the signing step fails.
	brokenApps, _ := json.Marshal([]config.AppConfig{{
		Id:            "test-app-id",
		PublishTokens: []string{"expo_test_token"},
		Keys: config.KeysConfig{
			Mode:        config.KeysModeLocal,
			PublicPath:  filepath.Join(projectRoot, "/test/keys/not.pem"),
			PrivatePath: filepath.Join(projectRoot, "/test/keys/exists.pem"),
		},
	}})
	os.Setenv("EXPO_APPS_JSON", string(brokenApps))
	config.ResetAppsForTest()
	if err := config.LoadApps(); err != nil {
		t.Fatalf("LoadApps: %v", err)
	}
	defer func() {
		SetValidConfiguration()
	}()

	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "test-app-id")
	// No channel mapping needed: "staging" falls back to branch "staging",
	// and even the resulting NoUpdateAvailable directive must be signed —
	// which is exactly where the broken key blows up.
	handlers.ManifestHandler(w, r)

	assert.Equal(t, 500, w.Code, "Expected status code 500 when certificates are not valid")
	assert.Equal(t, "Error signing content\n", w.Body.String(), "Expected 'Error signing content' message")
}

func TestNoUpdatesForManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "nop")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "test-app-id")
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "Expected status code 200 when manifest is retrieved")
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Errorf("Error parsing response: %v", err)
	}
	assert.Equal(t, 1, len(parts), "Expected 1 parts in the response")

	manifestPart := parts[0]

	assert.Equal(t, true, IsMultipartPartWithName(manifestPart, "directive"), "Expected a part with name 'manifest'")
	body := manifestPart.Body

	signature := manifestPart.Headers["Expo-Signature"]
	assert.NotNil(t, signature, "Expected a signature in the response")
	assert.NotEqual(t, "", signature, "Expected a signature in the response")
	validSignature := ValidateSignatureHeader("test-app-id", signature, body)
	assert.Equal(t, true, validSignature, "Expected a valid signature")

	var directive types.RollbackDirective
	err = json.Unmarshal([]byte(body), &directive)
	if err != nil {
		t.Errorf("Error parsing json body: %v", err)
	}
	assert.Equal(t, "noUpdateAvailable", directive.Type, "noUpdateAvailable")
}

func TestSkippingNotValidUpdatesAndCache(t *testing.T) {
	teardown := setup(t)
	defer teardown()
		lastUpdate, err := update.GetLatestUpdateBundlePathForRuntimeVersion("test-app-id", "branch-4", "1", "android")
	if err != nil {
		t.Errorf("Error getting latest update: %v", err)
	}
	assert.Equal(t, "1674170951", lastUpdate.UpdateId, "Expected a specific update id")
	resolvedBucket := bucket.GetBucket()
	file, _ := resolvedBucket.GetFile(*lastUpdate, ".check")
	defer file.Reader.Close()
	cache := cache2.GetCache()
	cacheKey := update.ComputeLastUpdateCacheKey("test-app-id", "branch-4", "1", "android")
	value := cache.Get(cacheKey)
	assert.Equal(t, "{\"appId\":\"test-app-id\",\"branch\":\"branch-4\",\"runtimeVersion\":\"1\",\"updateId\":\"1674170951\",\"createdAt\":1674170951000000}", value, "Expected a specific value")
	assert.NotNil(t, file.Reader, "Expected a file")
}

func TestValidRequestForStagingManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})

	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "android")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "test-app-id")
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "Expected status code 200 when manifest is retrieved")
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Errorf("Error parsing response: %v", err)
	}
	assert.Equal(t, 1, len(parts), "Expected 1 parts in the response")

	manifestPart := parts[0]

	assert.Equal(t, true, IsMultipartPartWithName(manifestPart, "manifest"), "Expected a part with name 'manifest'")
	body := manifestPart.Body

	signature := manifestPart.Headers["Expo-Signature"]
	assert.NotNil(t, signature, "Expected a signature in the response")
	assert.NotEqual(t, "", signature, "Expected a signature in the response")
	validSignature := ValidateSignatureHeader("test-app-id", signature, body)
	assert.Equal(t, true, validSignature, "Expected a valid signature")
	var updateManifest types.UpdateManifest
	err = json.Unmarshal([]byte(body), &updateManifest)
	if err != nil {
		t.Errorf("Error parsing json body: %v", err)
	}
	assert.Equal(t, "1990-01-01T00:00:00.000Z", updateManifest.CreatedAt, "Expected a specific created at date")
	assert.Equal(t, "1", updateManifest.RunTimeVersion, "Expected a specific runtime version")
	assert.Equal(t, json.RawMessage("{\"branch\":\"branch-1\"}"), updateManifest.Metadata, "Expected branch in metadata")
	assert.Equal(t, "{\"id\":\"04b793a0-b6ab-fd4f-308c-b91d812adec2\",\"createdAt\":\"1990-01-01T00:00:00.000Z\",\"runtimeVersion\":\"1\",\"metadata\":{\"branch\":\"branch-1\"},\"assets\":[{\"hash\":\"JCcs2u_4LMX6zazNmCpvBbYMRQRwS7-UwZpjiGWYgLs\",\"key\":\"4f1cb2cac2370cd5050681232e8575a8\",\"fileExtension\":\".png\",\"contentType\":\"application/javascript\",\"url\":\"http://localhost:3000/assets?asset=assets%2F4f1cb2cac2370cd5050681232e8575a8\\u0026branch=branch-1\\u0026platform=android\\u0026runtimeVersion=1\"}],\"launchAsset\":{\"hash\":\"t3kWQ00Lhn5qCGGhNNMxiD_pcTO_4d7I_1zO3S5Me5k\",\"key\":\"82adadb1fb6e489d04ad95fd79670deb\",\"fileExtension\":\".bundle\",\"contentType\":\"\",\"url\":\"http://localhost:3000/assets?asset=bundles%2Fandroid-82adadb1fb6e489d04ad95fd79670deb.js\\u0026branch=branch-1\\u0026platform=android\\u0026runtimeVersion=1\"},\"extra\":{\"expoClient\":{\"name\":\"expo-updates-client\",\"slug\":\"expo-updates-client\",\"owner\":\"anonymous\",\"version\":\"1.0.0\",\"orientation\":\"portrait\",\"icon\":\"./assets/icon.png\",\"splash\":{\"image\":\"./assets/splash.png\",\"resizeMode\":\"contain\",\"backgroundColor\":\"#ffffff\"},\"runtimeVersion\":\"1\",\"updates\":{\"url\":\"http://localhost:3000/api/manifest\",\"enabled\":true,\"fallbackToCacheTimeout\":30000},\"assetBundlePatterns\":[\"**/*\"],\"ios\":{\"supportsTablet\":true,\"bundleIdentifier\":\"com.test.expo-updates-client\"},\"android\":{\"adaptiveIcon\":{\"foregroundImage\":\"./assets/adaptive-icon.png\",\"backgroundColor\":\"#FFFFFF\"},\"package\":\"com.test.expoupdatesclient\"},\"web\":{\"favicon\":\"./assets/favicon.png\"},\"sdkVersion\":\"47.0.0\",\"platforms\":[\"ios\",\"android\",\"web\"],\"currentFullName\":\"@anonymous/expo-updates-client\",\"originalFullName\":\"@anonymous/expo-updates-client\"},\"branch\":\"branch-1\"}}", body)
}

func TestNoUpdatesResponseForManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})

	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-current-update-id", "04b793a0-b6ab-fd4f-308c-b91d812adec2")
	r.Header.Add("expo-channel-name", "staging")
	r.Header.Add("expo-app-id", "test-app-id")
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "Expected status code 200 when manifest is retrieved")
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Errorf("Error parsing response: %v", err)
	}
	assert.Equal(t, 1, len(parts), "Expected 1 parts in the response")

	manifestPart := parts[0]

	assert.Equal(t, true, IsMultipartPartWithName(manifestPart, "directive"), "Expected a part with name 'manifest'")
	body := manifestPart.Body

	signature := manifestPart.Headers["Expo-Signature"]
	assert.NotNil(t, signature, "Expected a signature in the response")
	assert.NotEqual(t, "", signature, "Expected a signature in the response")
	validSignature := ValidateSignatureHeader("test-app-id", signature, body)
	assert.Equal(t, true, validSignature, "Expected a valid signature")

	var directive types.RollbackDirective
	err = json.Unmarshal([]byte(body), &directive)
	if err != nil {
		t.Errorf("Error parsing json body: %v", err)
	}
	assert.Equal(t, "noUpdateAvailable", directive.Type, "noUpdateAvailable")
}

func TestRollbackResponseforManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"rollbackenv": "branch-3"})
	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-current-update-id", "04b793a0-b6ab-fd4f-308c-b91d812adec2")
	r.Header.Add("expo-embedded-update-id", "embedded-update-id")
	r.Header.Add("expo-channel-name", "rollbackenv")
	r.Header.Add("expo-app-id", "test-app-id")
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "Expected status code 200 when manifest is retrieved")
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Errorf("Error parsing response: %v", err)
	}
	assert.Equal(t, 1, len(parts), "Expected 1 parts in the response")

	manifestPart := parts[0]

	assert.Equal(t, true, IsMultipartPartWithName(manifestPart, "directive"), "Expected a part with name 'manifest'")
	body := manifestPart.Body

	signature := manifestPart.Headers["Expo-Signature"]
	assert.NotNil(t, signature, "Expected a signature in the response")
	assert.NotEqual(t, "", signature, "Expected a signature in the response")
	validSignature := ValidateSignatureHeader("test-app-id", signature, body)
	assert.Equal(t, true, validSignature, "Expected a valid signature")

	var directive types.RollbackDirective
	err = json.Unmarshal([]byte(body), &directive)
	if err != nil {
		t.Errorf("Error parsing json body: %v", err)
	}
	assert.Equal(t, "rollBackToEmbedded", directive.Type, "rollBackToEmbedded")
}

func TestValidRequestForProductionManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"production": "branch-2"})

	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "ios")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "production")
	r.Header.Add("expo-app-id", "test-app-id")
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "Expected status code 200 when manifest is retrieved")
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Errorf("Error parsing response: %v", err)
	}
	assert.Equal(t, 1, len(parts), "Expected 1 parts in the response")

	manifestPart := parts[0]

	assert.Equal(t, true, IsMultipartPartWithName(manifestPart, "manifest"), "Expected a part with name 'manifest'")
	body := manifestPart.Body

	signature := manifestPart.Headers["Expo-Signature"]
	assert.NotNil(t, signature, "Expected a signature in the response")
	assert.NotEqual(t, "", signature, "Expected a signature in the response")
	validSignature := ValidateSignatureHeader("test-app-id", signature, body)
	assert.Equal(t, true, validSignature, "Expected a valid signature")
	var updateManifest types.UpdateManifest
	err = json.Unmarshal([]byte(body), &updateManifest)
	if err != nil {
		t.Errorf("Error parsing json body: %v", err)
	}
	assert.Equal(t, "1990-01-01T00:00:00.000Z", updateManifest.CreatedAt, "Expected a specific created at date")
	assert.Equal(t, "1", updateManifest.RunTimeVersion, "Expected a specific runtime version")
	assert.Equal(t, json.RawMessage("{\"branch\":\"branch-2\"}"), updateManifest.Metadata, "Expected branch in metadata")
	assert.Equal(t, "{\"id\":\"68e096e2-a619-9d56-7f7c-89f97bc27312\",\"createdAt\":\"1990-01-01T00:00:00.000Z\",\"runtimeVersion\":\"1\",\"metadata\":{\"branch\":\"branch-2\"},\"assets\":[{\"hash\":\"JCcs2u_4LMX6zazNmCpvBbYMRQRwS7-UwZpjiGWYgLs\",\"key\":\"4f1cb2cac2370cd5050681232e8575a8\",\"fileExtension\":\".png\",\"contentType\":\"application/javascript\",\"url\":\"http://localhost:3000/assets?asset=assets%2F4f1cb2cac2370cd5050681232e8575a8\\u0026branch=branch-2\\u0026platform=ios\\u0026runtimeVersion=1\"}],\"launchAsset\":{\"hash\":\"vH93RoNbdzk_2emr38L0ZVYJVBTPcspX5-5DXLUkiQ8\",\"key\":\"e44a25e2b1df198470a04adc1dd82e4e\",\"fileExtension\":\".bundle\",\"contentType\":\"\",\"url\":\"http://localhost:3000/assets?asset=_expo%2Fstatic%2Fjs%2Fios%2FAppEntry-546b83fc2035b34c5f2dbd9bb04a2478.hbc\\u0026branch=branch-2\\u0026platform=ios\\u0026runtimeVersion=1\"},\"extra\":{\"expoClient\":{\"name\":\"expo-updates-client\",\"slug\":\"expo-updates-client\",\"owner\":\"anonymous\",\"version\":\"1.0.0\",\"orientation\":\"portrait\",\"icon\":\"./assets/icon.png\",\"splash\":{\"image\":\"./assets/splash.png\",\"resizeMode\":\"contain\",\"backgroundColor\":\"#ffffff\"},\"runtimeVersion\":\"1\",\"updates\":{\"url\":\"http://localhost:3000/api/manifest\",\"enabled\":true,\"fallbackToCacheTimeout\":30000},\"assetBundlePatterns\":[\"**/*\"],\"ios\":{\"supportsTablet\":true,\"bundleIdentifier\":\"com.test.expo-updates-client\"},\"android\":{\"adaptiveIcon\":{\"foregroundImage\":\"./assets/adaptive-icon.png\",\"backgroundColor\":\"#FFFFFF\"},\"package\":\"com.test.expoupdatesclient\"},\"web\":{\"favicon\":\"./assets/favicon.png\"},\"plugins\":[[\"expo-build-properties\",{\"android\":{\"usesCleartextTraffic\":true},\"ios\":{}}]],\"sdkVersion\":\"52.0.0\",\"platforms\":[\"ios\",\"android\"],\"currentFullName\":\"@anonymous/expo-updates-client\",\"originalFullName\":\"@anonymous/expo-updates-client\"},\"branch\":\"branch-2\"}}", body)
}

func TestEmptyRequestForAndroid(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"production": "branch-3"})

	q := "http://localhost:3000/manifest"

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", q, nil)
	r.Header.Add("expo-platform", "android")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "production")
	r.Header.Add("expo-app-id", "test-app-id")
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "Expected status code 200 when manifest is retrieved")
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Errorf("Error parsing response: %v", err)
	}
	assert.Equal(t, 1, len(parts), "Expected 1 parts in the response")

	manifestPart := parts[0]

	assert.Equal(t, true, IsMultipartPartWithName(manifestPart, "directive"), "Expected a part with name 'directive'")
	body := manifestPart.Body

	signature := manifestPart.Headers["Expo-Signature"]
	assert.NotNil(t, signature, "Expected a signature in the response")
	assert.NotEqual(t, "", signature, "Expected a signature in the response")
	validSignature := ValidateSignatureHeader("test-app-id", signature, body)
	assert.Equal(t, true, validSignature, "Expected a valid signature")
	var updateManifest types.UpdateManifest
	err = json.Unmarshal([]byte(body), &updateManifest)
	if err != nil {
		t.Errorf("Error parsing json body: %v", err)
	}
	assert.Equal(t, "{\"type\":\"noUpdateAvailable\"}", body)
}


func TestPreWarmManifestCache(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})

	cache := cache2.GetCache()

	// Verify caches are empty before prewarm
	lastUpdateKey := update.ComputeLastUpdateCacheKey("test-app-id", "branch-1", "1", "android")
	assert.Equal(t, "", cache.Get(lastUpdateKey), "lastUpdate cache should be empty before prewarm")

	// Run PreWarm synchronously (not as goroutine) for testing
	update.PreWarmManifestCache("test-app-id", "branch-1", "1", "android")

	// Verify lastUpdate cache was populated
	lastUpdateCached := cache.Get(lastUpdateKey)
	assert.NotEqual(t, "", lastUpdateCached, "lastUpdate cache should be populated after prewarm")

	// Verify metadata cache was populated
	var cachedUpdate types.Update
	err := json.Unmarshal([]byte(lastUpdateCached), &cachedUpdate)
	assert.NoError(t, err)
	metadataKey := update.ComputeMetadataCacheKey("test-app-id", "branch-1", "1", cachedUpdate.UpdateId)
	assert.NotEqual(t, "", cache.Get(metadataKey), "metadata cache should be populated after prewarm")

	// Verify manifest cache was populated
	manifestKey := update.ComputeUpdateManifestCacheKey("test-app-id", "branch-1", "1", cachedUpdate.UpdateId, "android")
	assert.NotEqual(t, "", cache.Get(manifestKey), "manifest cache should be populated after prewarm")
}

// TestPathBasedManifestNoHeader: legacy apps put the app id in the URL path
// (/{APP_ID}/manifest) and never send the expo-app-id header. The handler must
// resolve the app id from the path var alone and serve a normal manifest.
func TestPathBasedManifestNoHeader(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://localhost:3000/test-app-id/manifest", nil)
	r.Header.Add("expo-platform", "android")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	// expo-app-id 헤더 없음 — 경로 var로만 식별.
	r = mux.SetURLVars(r, map[string]string{"APP_ID": "test-app-id"})
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "경로 var로 식별되어 200이어야 함")
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Errorf("Error parsing response: %v", err)
	}
	assert.Equal(t, 1, len(parts), "Expected 1 part")
	assert.Equal(t, true, IsMultipartPartWithName(parts[0], "manifest"), "Expected a 'manifest' part")
}

// TestPathBasedManifestPathWinsOverHeader: when both path var and header are
// present and disagree, the URL path wins (it's the route truth, and what
// AppResolverMiddleware validated). Header carries an unregistered id, so if
// the header won this would 404.
func TestPathBasedManifestPathWinsOverHeader(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://localhost:3000/test-app-id/manifest", nil)
	r.Header.Add("expo-platform", "android")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-expect-signature", "true")
	r.Header.Add("expo-channel-name", "staging")
	// 헤더는 미등록 app id — 경로가 우선되면 200, 헤더가 우선되면 404.
	r.Header.Add("expo-app-id", "this-id-is-not-in-apps-json")
	r = mux.SetURLVars(r, map[string]string{"APP_ID": "test-app-id"})
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code, "경로(URL)가 헤더보다 우선되어 200이어야 함")
}

// TestPathBasedManifestAssetUrlIsAppScoped: for path-based requests the asset
// and launchAsset URLs in the manifest body must be rewritten to the
// app-scoped path (/{APP_ID}/assets) so the legacy client can fetch them
// without the expo-app-id header. The query string is preserved.
func TestPathBasedManifestAssetUrlIsAppScoped(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "http://localhost:3000/test-app-id/manifest", nil)
	r.Header.Add("expo-platform", "android")
	r.Header.Add("expo-runtime-version", "1")
	r.Header.Add("expo-protocol-version", "1")
	r.Header.Add("expo-channel-name", "staging")
	r = mux.SetURLVars(r, map[string]string{"APP_ID": "test-app-id"})
	handlers.ManifestHandler(w, r)
	assert.Equal(t, 200, w.Code)
	parts, err := ParseMultipartMixedResponse(w.Header().Get("Content-Type"), w.Body.Bytes())
	if err != nil {
		t.Fatalf("Error parsing response: %v", err)
	}
	body := string(parts[0].Body)
	assert.Contains(t, body, "http://localhost:3000/test-app-id/assets?", "asset URL은 app-scoped 경로여야 함")
	assert.NotContains(t, body, "http://localhost:3000/assets?", "rewrite 후 비-scoped /assets URL이 남으면 안 됨")
}
