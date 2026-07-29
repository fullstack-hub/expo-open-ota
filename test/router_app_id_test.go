package test

import (
	infrastructure "expo-open-ota/internal/router"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRouterPathBasedManifest verifies the full router path: /{APP_ID}/manifest
// serves with no expo-app-id header (path var carries the id), and an
// unregistered app id is short-circuited to 404 by AppResolverMiddleware.
func TestRouterPathBasedManifest(t *testing.T) {
	teardown := setup(t)
	defer teardown()
	SetChannelsConfig(t, map[string]string{"staging": "branch-1"})
	router := infrastructure.NewRouter()

	t.Run("registered app id via path, no header", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "http://localhost:3000/test-app-id/manifest", nil)
		r.Header.Add("expo-platform", "android")
		r.Header.Add("expo-runtime-version", "1")
		r.Header.Add("expo-protocol-version", "1")
		r.Header.Add("expo-expect-signature", "true")
		r.Header.Add("expo-channel-name", "staging")
		router.ServeHTTP(w, r)
		assert.Equal(t, 200, w.Code, "경로 기반 manifest는 헤더 없이 200이어야 함")
	})

	t.Run("unknown app id 404 via middleware", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "http://localhost:3000/this-id-is-not-in-apps-json/manifest", nil)
		r.Header.Add("expo-platform", "android")
		r.Header.Add("expo-runtime-version", "1")
		r.Header.Add("expo-protocol-version", "1")
		r.Header.Add("expo-channel-name", "staging")
		router.ServeHTTP(w, r)
		assert.Equal(t, 404, w.Code, "미등록 app id는 AppResolverMiddleware가 404로 차단해야 함")
	})
}
