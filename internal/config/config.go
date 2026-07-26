package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	BindAddr           string
	DatabaseURL        string
	JWTSecret          string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	UploadDir          string
	UploadBaseURL      string
	R2AccessKeyID      string
	R2SecretAccessKey  string
	R2Endpoint         string
	R2BucketName       string
	ScraperAPIKey      string
	RedisURL           string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	bind := getEnv("BIND_ADDR", ":8080")
	db := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/dashboard?sslmode=disable")

	redisUrl := getEnv("REDIS_URL", "redis://localhost:6379")

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	atMin := getEnv("ACCESS_TOKEN_MINUTES", "15")
	atM, _ := strconv.Atoi(atMin)

	rtDays := getEnv("REFRESH_TOKEN_DAYS", "7")
	rtD, _ := strconv.Atoi(rtDays)

	return &Config{
		BindAddr:           bind,
		DatabaseURL:        db,
		JWTSecret:          secret,
		AccessTokenTTL:     time.Duration(atM) * time.Minute,
		RefreshTokenTTL:    time.Duration(rtD) * 24 * time.Hour,
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		UploadDir:          getEnv("UPLOAD_DIR", "./uploads"),
		UploadBaseURL:      getEnv("UPLOAD_BASE_URL", "http://localhost:8080"),
		R2AccessKeyID:      os.Getenv("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey:  os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Endpoint:         os.Getenv("R2_ENDPOINT"),
		R2BucketName:       os.Getenv("R2_BUCKET_NAME"),
		ScraperAPIKey:      os.Getenv("SCRAPER_API_KEY"),
		RedisURL: redisUrl,
	}, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
