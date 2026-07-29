package test

import (
	"encoding/json"
	"expo-open-ota/config"
	"expo-open-ota/internal/bucket"
	cache2 "expo-open-ota/internal/cache"
	"expo-open-ota/internal/cdn"
	"expo-open-ota/internal/handlers"
	"expo-open-ota/internal/metrics"
	"expo-open-ota/internal/types"
	"github.com/jarcoal/httpmock"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func setup(t *testing.T) func() {
	GlobalBeforeEach()
	// No responders are registered anywhere in the suite: httpmock stays
	// active purely as a tripwire so any unexpected outbound HTTP call
	// (e.g. a regression reintroducing api.expo.dev) fails the test.
	httpmock.Activate()
	SetValidConfiguration()
	metrics.InitMetrics()
	return func() {
		GlobalAfterEach(t)
		defer httpmock.DeactivateAndReset()
	}
}

func GlobalBeforeEach() {
	metrics.CleanupMetrics()
	cache := cache2.GetCache()
	_ = cache.Clear()
	newTime := time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC)

	ChangeModTimeRecursively(os.Getenv("LOCAL_BUCKET_BASE_PATH"), newTime)
}

func GlobalAfterEach(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		bucket.ResetBucketInstance()
		cdn.ResetCDNInstance()
		projectRoot, err := findProjectRoot()
		if err != nil {
			t.Errorf("Error finding project root: %v", err)
		}
		// Clean both legacy path (./updates/DO_NOT_USE) and v2 multi-app path
		// (./updates/test-app-id/DO_NOT_USE) — tests mix both depending on how
		// they set LOCAL_BUCKET_BASE_PATH.
		for _, updatesPath := range []string{
			filepath.Join(projectRoot, "./updates/DO_NOT_USE"),
			filepath.Join(projectRoot, "./updates/test-app-id/DO_NOT_USE"),
		} {
			updates, err := os.ReadDir(updatesPath)
			if err != nil {
				continue
			}
			for _, update := range updates {
				if update.IsDir() {
					err = os.RemoveAll(filepath.Join(updatesPath, update.Name()))
					if err != nil {
						t.Errorf("Error removing update directory: %v", err)
					}
				}
			}
		}
		// Also remove all folders > 1674170951 in ./test/test-updates/test-app-id/branch-1/1
		fixturePath := filepath.Join(projectRoot, "./test/test-updates/test-app-id/branch-1/1")
		fixtureUpdates, err := os.ReadDir(fixturePath)
		if err != nil {
			t.Errorf("Error reading updates directory: %v", err)
		}
		for _, update := range fixtureUpdates {
			if update.IsDir() {
				updateTime, err := strconv.Atoi(update.Name())
				if err != nil {
					continue
				}
				if updateTime > 1674170951 {
					err = os.RemoveAll(filepath.Join(fixturePath, update.Name()))
					if err != nil {
						t.Errorf("Error removing update directory: %v", err)
					}
				}
			}
		}
	})

}

func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}

	return "", os.ErrNotExist
}

func ComputeUploadRequestsInput(dirPath string) handlers.FileNamesRequest {
	metadataFilePath := filepath.Join(dirPath, "metadata.json")
	metadataFile, err := os.Open(metadataFilePath)
	if err != nil {
		panic(err)
	}
	defer metadataFile.Close()
	var metadataObject types.MetadataObject
	err = json.NewDecoder(metadataFile).Decode(&metadataObject)
	if err != nil {
		panic(err)
	}
	fileNames := make([]string, 0)
	for _, asset := range metadataObject.FileMetadata.IOS.Assets {
		fileNames = append(fileNames, asset.Path)
	}
	for _, asset := range metadataObject.FileMetadata.Android.Assets {
		fileNames = append(fileNames, asset.Path)
	}
	if metadataObject.FileMetadata.Android.Bundle != "" {
		fileNames = append(fileNames, metadataObject.FileMetadata.Android.Bundle)
	}
	if metadataObject.FileMetadata.IOS.Bundle != "" {
		fileNames = append(fileNames, metadataObject.FileMetadata.IOS.Bundle)
	}
	// Add metadata.json & expoConfig.json
	fileNames = append(fileNames, "metadata.json")
	fileNames = append(fileNames, "expoConfig.json")
	return handlers.FileNamesRequest{FileNames: fileNames}
}

func ChangeModTime(filePath string, newTime time.Time) error {
	// Ouvre le fichier
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = os.Chtimes(filePath, newTime, newTime)
	if err != nil {
		return err
	}

	return nil
}

func ChangeModTimeRecursively(dir string, newTime time.Time) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			err := ChangeModTime(path, newTime)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func SetValidConfiguration() {
	projectRoot, err := findProjectRoot()
	if err != nil {
		panic(err)
	}
	os.Setenv("BASE_URL", "http://localhost:3000")
	os.Setenv("LOCAL_BUCKET_BASE_PATH", filepath.Join(projectRoot, "/test/test-updates"))
	os.Setenv("JWT_SECRET", "test_jwt_secret")
	os.Setenv("PRIVATE_CLOUDFRONT_KEY_PATH", "")
	os.Setenv("CLOUDFRONT_DOMAIN", "")
	os.Setenv("CLOUDFRONT_KEY_PAIR_ID", "")
	os.Setenv("USE_DASHBOARD", "true")
	os.Setenv("ADMIN_PASSWORD", "admin")

	// v2 multi-app config: a single test-app-id entry pointing at the
	// existing test keys. publishTokens registers the literal
	// "expo_test_token" string the tests send as Bearer, so request code
	// stays unchanged.
	appsJSON, err := json.Marshal([]config.AppConfig{{
		Id:            "test-app-id",
		PublishTokens: []string{"expo_test_token"},
		Keys: config.KeysConfig{
			Mode:        config.KeysModeLocal,
			PublicPath:  filepath.Join(projectRoot, "/test/keys/public-key-test.pem"),
			PrivatePath: filepath.Join(projectRoot, "/test/keys/private-key-test.pem"),
		},
	}})
	if err != nil {
		panic(err)
	}
	os.Setenv("EXPO_APPS_JSON", string(appsJSON))
	config.ResetAppsForTest()
	if err := config.LoadApps(); err != nil {
		panic(err)
	}
}

// SetChannelsConfig rebuilds the test app config with the given channel→branch
// map and reloads apps. Cleanup restores the default (no channels) config.
func SetChannelsConfig(t *testing.T, channels map[string]string) {
	t.Helper()
	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	appsJSON, err := json.Marshal([]config.AppConfig{{
		Id:            "test-app-id",
		PublishTokens: []string{"expo_test_token"},
		Channels:      channels,
		Keys: config.KeysConfig{
			Mode:        config.KeysModeLocal,
			PublicPath:  filepath.Join(projectRoot, "/test/keys/public-key-test.pem"),
			PrivatePath: filepath.Join(projectRoot, "/test/keys/private-key-test.pem"),
		},
	}})
	if err != nil {
		t.Fatalf("marshal apps: %v", err)
	}
	os.Setenv("EXPO_APPS_JSON", string(appsJSON))
	config.ResetAppsForTest()
	if err := config.LoadApps(); err != nil {
		t.Fatalf("LoadApps: %v", err)
	}
	t.Cleanup(SetValidConfiguration)
}
