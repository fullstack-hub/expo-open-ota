package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

type AppConfig struct {
	Slug            string `json:"slug"`
	ExpoAppId       string `json:"expoAppId"`
	ExpoAccessToken string `json:"expoAccessToken"`
	S3KeyPrefix     string `json:"s3KeyPrefix,omitempty"`
}

func (a *AppConfig) GetEffectiveS3KeyPrefix() string {
	if a.S3KeyPrefix != "" {
		prefix := a.S3KeyPrefix
		if prefix[len(prefix)-1] != '/' {
			prefix += "/"
		}
		return prefix
	}
	return a.Slug + "/"
}

var (
	appsConfig    []AppConfig
	multiAppMode  bool
	appsOnce      sync.Once
)

func LoadAppsConfig() {
	appsOnce.Do(func() {
		raw := os.Getenv("APPS_CONFIG")
		if raw == "" {
			multiAppMode = false
			return
		}
		if err := json.Unmarshal([]byte(raw), &appsConfig); err != nil {
			log.Fatalf("Invalid APPS_CONFIG JSON: %v", err)
		}
		if len(appsConfig) == 0 {
			log.Fatalf("APPS_CONFIG is empty")
		}
		slugs := make(map[string]bool)
		for _, app := range appsConfig {
			if app.Slug == "" || app.ExpoAppId == "" || app.ExpoAccessToken == "" {
				log.Fatalf("APPS_CONFIG: slug, expoAppId, and expoAccessToken are required for each app")
			}
			if slugs[app.Slug] {
				log.Fatalf("APPS_CONFIG: duplicate slug: %s", app.Slug)
			}
			slugs[app.Slug] = true
		}
		multiAppMode = true
	})
}

func IsMultiAppMode() bool {
	return multiAppMode
}

func GetAppConfig(slug string) *AppConfig {
	for i := range appsConfig {
		if appsConfig[i].Slug == slug {
			return &appsConfig[i]
		}
	}
	return nil
}

func GetAllApps() []AppConfig {
	return appsConfig
}

func GetDefaultAppConfig() *AppConfig {
	if multiAppMode {
		return nil
	}
	return &AppConfig{
		Slug:            "",
		ExpoAppId:       GetEnv("EXPO_APP_ID"),
		ExpoAccessToken: GetEnv("EXPO_ACCESS_TOKEN"),
		S3KeyPrefix:     GetEnv("S3_KEY_PREFIX"),
	}
}

func ResetAppsConfig() {
	appsConfig = nil
	multiAppMode = false
	appsOnce = sync.Once{}
}
