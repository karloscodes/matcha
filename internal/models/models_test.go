package models

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	err = db.AutoMigrate(&Product{}, &Customer{}, &LicenseKey{}, &AdminUser{}, &EmailSettings{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func TestEmailSettings_Save(t *testing.T) {
	db := setupTestDB(t)

	settings1 := &EmailSettings{
		Provider:       "smtp",
		SMTPHost:       "smtp.gmail.com",
		SMTPPort:       587,
		SMTPUsername:   "test@example.com",
		SMTPPassword:   "password",
		SMTPEncryption: "tls",
		FromEmail:      "noreply@example.com",
		FromName:       "Test Service",
		IsActive:       true,
	}

	err := settings1.Save(db)
	if err != nil {
		t.Fatalf("Failed to save email settings: %v", err)
	}

	settings2 := &EmailSettings{
		Provider:       "smtp",
		SMTPHost:       "smtp.mailgun.org",
		SMTPPort:       587,
		SMTPUsername:   "test2@example.com",
		SMTPPassword:   "password2",
		SMTPEncryption: "tls",
		FromEmail:      "noreply2@example.com",
		FromName:       "Test Service 2",
		IsActive:       true,
	}

	err = settings2.Save(db)
	if err != nil {
		t.Fatalf("Failed to save second email settings: %v", err)
	}

	db.First(settings1, settings1.ID)
	if settings1.IsActive {
		t.Error("First settings should be deactivated when second is saved as active")
	}
}

func TestGetActiveEmailSettings(t *testing.T) {
	db := setupTestDB(t)

	settings := &EmailSettings{
		Provider:       "smtp",
		SMTPHost:       "smtp.gmail.com",
		SMTPPort:       587,
		SMTPUsername:   "test@example.com",
		SMTPPassword:   "password",
		SMTPEncryption: "tls",
		FromEmail:      "noreply@example.com",
		FromName:       "Test Service",
		IsActive:       true,
	}

	err := db.Create(settings).Error
	if err != nil {
		t.Fatalf("Failed to create email settings: %v", err)
	}

	active, err := GetActiveEmailSettings(db)
	if err != nil {
		t.Fatalf("Failed to get active email settings: %v", err)
	}

	if active.ID != settings.ID {
		t.Error("Retrieved settings should match created settings")
	}

	if active.SMTPHost != "smtp.gmail.com" {
		t.Error("SMTP host should match")
	}
}

func TestEmailSettings_Activate(t *testing.T) {
	db := setupTestDB(t)

	settings1 := &EmailSettings{
		Provider:       "smtp",
		SMTPHost:       "smtp.gmail.com",
		SMTPPort:       587,
		SMTPUsername:   "test@example.com",
		SMTPPassword:   "password",
		SMTPEncryption: "tls",
		FromEmail:      "noreply@example.com",
		FromName:       "Test Service",
		IsActive:       true,
	}
	db.Create(settings1)

	settings2 := &EmailSettings{
		Provider:       "smtp",
		SMTPHost:       "smtp.mailgun.org",
		SMTPPort:       587,
		SMTPUsername:   "test2@example.com",
		SMTPPassword:   "password2",
		SMTPEncryption: "tls",
		FromEmail:      "noreply2@example.com",
		FromName:       "Test Service 2",
		IsActive:       false,
	}
	db.Create(settings2)

	err := settings2.Activate(db)
	if err != nil {
		t.Fatalf("Failed to activate settings: %v", err)
	}

	db.First(settings1, settings1.ID)
	if settings1.IsActive {
		t.Error("First settings should be deactivated when second is activated")
	}

	db.First(settings2, settings2.ID)
	if !settings2.IsActive {
		t.Error("Second settings should be active after activation")
	}
}