// backend/config/config.go

package config

import (
	"os"
	"strings"
)

type Config struct {
	Port            string
	PostgresDSN     string
	RedisAddr       string

	R2Endpoint  string
	R2Region    string
	R2Bucket    string
	R2Key       string
	R2Secret    string
	R2PathStyle bool

	CFImagesAccountID string
	CFImagesAPIToken  string

	AuthJWKS    string
	CORSOrigins []string
	FFmpegBin   string
	JWTSecret   string

	// Claude API key for reading floor plans (dollhouse builder); empty disables it
	AnthropicAPIKey string
}

func Load() Config {
	return Config{
		Port:        get("PORT", "5555"),
		PostgresDSN: get("POSTGRES_DSN", ""),
		RedisAddr:   get("REDIS_ADDR", "localhost:6379"),
		JWTSecret:   get("JWT_SECRET", ""),

		R2Endpoint:  get("R2_ENDPOINT", ""),
		R2Region:    get("R2_REGION", "auto"),
		R2Bucket:    get("R2_BUCKET", ""),
		R2Key:       get("R2_ACCESS_KEY", ""),
		R2Secret:    get("R2_SECRET_KEY", ""),
		R2PathStyle: get("R2_USE_PATH_STYLE", "false") == "true",

		CFImagesAccountID: get("CF_IMAGES_ACCOUNT_ID", ""),
		CFImagesAPIToken:  get("CF_IMAGES_API_TOKEN", ""),

		AuthJWKS:    get("AUTH_JWKS_URL", ""),
		CORSOrigins: split(get("CORS_ORIGINS", "*")),
		FFmpegBin:   get("FFMPEG_BIN", "/usr/bin/ffmpeg"),

		AnthropicAPIKey: get("ANTHROPIC_API_KEY", ""),
	}
}

func get(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
