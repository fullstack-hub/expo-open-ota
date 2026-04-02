package branch

import (
	"expo-open-ota/config"
	"expo-open-ota/internal/helpers"
	"expo-open-ota/internal/services"
)

func UpsertBranch(app *config.AppConfig, branch string) error {
	branches, err := services.FetchExpoBranches(app)
	if err != nil {
		return err
	}
	if !helpers.StringInSlice(branch, branches) {
		return services.CreateBranch(app, branch)
	}
	return nil
}
