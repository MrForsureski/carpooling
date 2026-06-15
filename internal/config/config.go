package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
	DatabaseURL string

	JWTSecret        string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration

	OSRMBaseURL string

	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	AppBaseURL string
	AppName    string

	DefaultMaxDetourMinutes  int
	DefaultMaxDistanceMeters int
	DefaultMinJoinMinutes    int

	RequireEmailVerification bool
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	accessExpiry, err := time.ParseDuration(getEnv("JWT_ACCESS_EXPIRY", "15m"))
	if err != nil {
		accessExpiry = 15 * time.Minute
	}

	refreshExpiry, err := time.ParseDuration(getEnv("JWT_REFRESH_EXPIRY", "168h"))
	if err != nil {
		refreshExpiry = 7 * 24 * time.Hour
	}

	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))

	env := getEnv("ENVIRONMENT", "development")
	requireEmailVerifyStr := getEnv("REQUIRE_EMAIL_VERIFICATION", "")
	requireEmailVerify := env == "production"
	if requireEmailVerifyStr != "" {
		if val, err := strconv.ParseBool(requireEmailVerifyStr); err == nil {
			requireEmailVerify = val
		}
	}

	return &Config{
		Port:        getEnv("PORT", "8080"),
		Environment: env,
		DatabaseURL: getEnv("DATABASE_URL", "postgres://carpooling:secret@localhost:5432/carpooling?sslmode=disable"),

		JWTSecret:        getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:  accessExpiry,
		JWTRefreshExpiry: refreshExpiry,

		OSRMBaseURL: getEnv("OSRM_BASE_URL", "http://router.project-osrm.org"),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     smtpPort,
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),

		AppBaseURL: getEnv("APP_BASE_URL", "http://localhost:8080"),
		AppName:    getEnv("APP_NAME", "Поездки в офис"),

		DefaultMaxDetourMinutes:  getEnvInt("DEFAULT_MAX_DETOUR_MINUTES", 15),
		DefaultMaxDistanceMeters: getEnvInt("DEFAULT_MAX_DISTANCE_METERS", 2000),
		DefaultMinJoinMinutes:    getEnvInt("DEFAULT_MIN_JOIN_MINUTES", 30),

		RequireEmailVerification: requireEmailVerify,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}
