package main

import (
	"embed"
	"log"

	"matcha/internal/app"
	"matcha/internal/config"
	"matcha/internal/database"
	"matcha/internal/models"

	"github.com/joho/godotenv"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	} else {
		log.Println("Loaded .env file successfully")
	}

	// Initialize configuration
	cfg := config.New()
	log.Printf("Configuration loaded - Environment: %s, SecretKey: %s, Debug: %v", cfg.Environment, cfg.SecretKey, cfg.Debug)

	// Initialize database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto-migrate database
	if err := db.AutoMigrate(&models.Product{}, &models.Customer{}, &models.LicenseKey{}, &models.AdminUser{}, &models.EmailSettings{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Create default admin user
	if err := models.CreateDefaultAdmin(db, "admin", "admin123"); err != nil {
		log.Println("Warning: Could not create default admin user:", err)
	}

	// Create and configure the Fiber app
	fiberApp := app.NewApp(cfg, db, templateFS, staticFS)

	// Start server
	log.Printf("Server starting on port %s in %s environment", cfg.Port, cfg.Environment)
	log.Fatal(fiberApp.Listen(":" + cfg.Port))
}
