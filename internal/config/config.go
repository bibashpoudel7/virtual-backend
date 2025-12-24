// backend/config/config.go

package config

import (
	"os"
	"strings"
)

type Config struct {
	Port            string
	PostgresDSN     string
	MainPostgresDSN string // Main TheNimto database for users, companies, properties
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
}

func Load() Config {
	return Config{
		// postgresql://company_user:pacecode@123@209.38.154.134/company"
		Port:            get("PORT", "5555"),
		PostgresDSN:     get("POSTGRES_DSN", "postgres://pacecode:root@localhost:5432/virtual?sslmode=disable"),
		MainPostgresDSN: get("MAIN_POSTGRES_DSN", "postgres://nimto:nimto7f4556f43-61da-4bd7-9dca-99b046370f1a%40@209.38.154.134:5432/devevent?sslmode=disable"),
		RedisAddr:       get("REDIS_ADDR", "localhost:6379"),
		JWTSecret:       get("JWT_SECRET", "8db1b8d5158b97eb7b54bf002765323c04a5c1be86d34b44548637f6e7a358e160a078390957e5481b069761e8b8>"),

		R2Endpoint:  get("R2_ENDPOINT", "https://6d42dedb027b4e7b3b60bf73190ee171.r2.cloudflarestorage.com"),
		R2Region:    get("R2_REGION", "APAC"),
		R2Bucket:    get("R2_BUCKET", "test"),
		R2Key:       get("R2_ACCESS_KEY", "ff9b99e63ecca7293eb5d5b165f2886f"),
		R2Secret:    get("R2_SECRET_KEY", "610986a838e44426f93c66be7931777f5cc25dbb744439dc40d1d802c7a628b1"),
		R2PathStyle: get("R2_USE_PATH_STYLE", "false") == "true",

		CFImagesAccountID: get("CF_IMAGES_ACCOUNT_ID", "6d42dedb027b4e7b3b60bf73190ee171"),
		CFImagesAPIToken:  get("CF_IMAGES_API_TOKEN", "610986a838e44426f93c66be7931777f5cc25dbb744439dc40d1d802c7a628b1"),

		AuthJWKS:    get("AUTH_JWKS_URL", ""),
		CORSOrigins: split(get("CORS_ORIGINS", "*")),
		FFmpegBin:   get("FFMPEG_BIN", "/usr/bin/ffmpeg"),
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
