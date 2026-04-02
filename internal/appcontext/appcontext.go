package appcontext

import (
	"context"
	"expo-open-ota/config"
)

type contextKey string

const appConfigKey contextKey = "appConfig"

func WithAppConfig(ctx context.Context, cfg *config.AppConfig) context.Context {
	return context.WithValue(ctx, appConfigKey, cfg)
}

func GetAppConfig(ctx context.Context) *config.AppConfig {
	cfg, _ := ctx.Value(appConfigKey).(*config.AppConfig)
	return cfg
}
