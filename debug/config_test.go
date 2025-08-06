package main

import (
	"fmt"
	"os"

	"matcha/internal/config"
)

func main() {
	fmt.Println("=== Environment Variable Debug ===")

	// Check if .env is being loaded
	if secretFromEnv := os.Getenv("SECRET_KEY"); secretFromEnv != "" {
		fmt.Printf("SECRET_KEY from environment: %s\n", secretFromEnv)
	} else {
		fmt.Println("SECRET_KEY not found in environment variables")
	}

	// Check config loading
	cfg := config.New()
	fmt.Printf("Config SecretKey: %s\n", cfg.SecretKey)
	fmt.Printf("Config Environment: %s\n", cfg.Environment)
	fmt.Printf("Config Port: %s\n", cfg.Port)

	// Check if they match
	envSecret := os.Getenv("SECRET_KEY")
	if envSecret == cfg.SecretKey {
		fmt.Println("✓ Environment variable and config match")
	} else {
		fmt.Println("✗ Environment variable and config DO NOT match")
		fmt.Printf("  Env: '%s'\n", envSecret)
		fmt.Printf("  Config: '%s'\n", cfg.SecretKey)
	}
}
