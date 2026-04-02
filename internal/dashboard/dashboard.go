package dashboard

import (
	"expo-open-ota/config"
	"expo-open-ota/internal/version"
	"fmt"
)

func IsDashboardEnabled() bool {
	return config.GetEnv("USE_DASHBOARD") == "true"
}

func ComputeGetRuntimeVersionsCacheKey(slug string, branch string) string {
	return fmt.Sprintf("dashboard:%s:%s:request:getRuntimeVersions:%s", version.Version, slug, branch)
}

func ComputeGetBranchesCacheKey(slug string) string {
	return fmt.Sprintf("dashboard:%s:%s:request:getBranches", version.Version, slug)
}

func ComputeGetChannelsCacheKey(slug string) string {
	return fmt.Sprintf("dashboard:%s:%s:request:getChannels", version.Version, slug)
}

func ComputeGetUpdatesCacheKey(slug string, branch string, runtimeVersion string) string {
	return fmt.Sprintf("dashboard:%s:%s:request:getUpdates:%s:%s", version.Version, slug, branch, runtimeVersion)
}

func ComputeGetUpdateDetailsCacheKey(slug string, branch string, runtimeVersion string, updateID string) string {
	return fmt.Sprintf("dashboard:%s:%s:request:getUpdateDetails:%s:%s:%s", version.Version, slug, branch, runtimeVersion, updateID)
}
