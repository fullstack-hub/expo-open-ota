package test

import (
	"expo-open-ota/internal/types"
	"expo-open-ota/internal/update"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRewriteManifestAssetsToAppScoped checks that the app id is inserted
// immediately before the trailing /assets segment so a BASE_URL carrying a
// path prefix (e.g. behind /api) still produces a route-matchable URL.
// The query string must be preserved untouched.
func TestRewriteManifestAssetsToAppScoped(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		expected string
	}{
		{
			name:     "bare host (no path prefix)",
			in:       "https://ota.juvis.co.kr/assets?asset=bundles%2Fandroid-x.js&branch=branch-1&platform=android&runtimeVersion=1",
			expected: "https://ota.juvis.co.kr/test-app-id/assets?asset=bundles%2Fandroid-x.js&branch=branch-1&platform=android&runtimeVersion=1",
		},
		{
			name:     "path prefix preserved before app id",
			in:       "https://host/api/assets?asset=a",
			expected: "https://host/api/test-app-id/assets?asset=a",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &types.UpdateManifest{
				Assets:      []types.ManifestAsset{{Url: c.in}},
				LaunchAsset: types.ManifestAsset{Url: c.in},
			}
			update.RewriteManifestAssetsToAppScoped(m, "test-app-id")
			assert.Equal(t, c.expected, m.Assets[0].Url, "asset URL")
			assert.Equal(t, c.expected, m.LaunchAsset.Url, "launchAsset URL")
		})
	}
}
