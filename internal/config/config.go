package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Environment string
	Port        string
	DatabaseURL string
	SecretKey   string
	Debug       bool
}

func New() *Config {
	env := getEnv("GO_ENV", "development")

	cfg := &Config{
		Environment: env,
		Port:        getEnv("PORT", "8080"),
		SecretKey:   getEnv("SECRET_KEY", getDefaultSecretKey(env)),
		Debug:       getBoolEnv("DEBUG", env == "development"),
	}

	cfg.DatabaseURL = getEnv("DATABASE_URL", getDefaultDatabaseURL(env))

	return cfg
}

func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c *Config) IsTest() bool {
	return c.Environment == "test"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getDefaultDatabaseURL(env string) string {
	switch env {
	case "test":
		return "test_license_manager.db"
	case "production":
		return "prod_license_manager.db"
	default:
		return "license_manager.db"
	}
}

func getDefaultSecretKey(env string) string {
	switch env {
	case "production":
		return "CHANGE_ME_IN_PRODUCTION_" + fmt.Sprintf("%d", os.Getpid())
	default:
		return "dev-secret-key-not-for-production"
	}
}
