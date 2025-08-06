package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"license-key-manager/internal/config"
	"license-key-manager/internal/models"
	"license-key-manager/internal/services"
)

type DashboardHandler struct {
	db *gorm.DB
}

func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

func (h *DashboardHandler) Dashboard(c *fiber.Ctx) error {
	// Strong cache-busting headers to prevent browser caching issues
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	c.Set("Last-Modified", time.Now().UTC().Format(time.RFC1123))
	c.Set("ETag", fmt.Sprintf("\"%d\"", time.Now().Unix()))

	// Add timestamp to URL parameters to ensure fresh request
	timestamp := time.Now().Unix()

	var stats struct {
		TotalProducts   int64
		TotalCustomers  int64
		TotalLicenses   int64
		ActiveLicenses  int64
		ExpiredLicenses int64
	}

	h.db.Model(&models.Product{}).Count(&stats.TotalProducts)
	h.db.Model(&models.Customer{}).Count(&stats.TotalCustomers)
	h.db.Model(&models.LicenseKey{}).Count(&stats.TotalLicenses)
	h.db.Model(&models.LicenseKey{}).Where("status = ?", "active").Count(&stats.ActiveLicenses)
	h.db.Model(&models.LicenseKey{}).Where("expires_at < ?", time.Now()).Count(&stats.ExpiredLicenses)

	var recentLicenses []models.LicenseKey
	h.db.Preload("Product").Preload("Customer").
		Order("created_at DESC").
		Limit(10).
		Find(&recentLicenses)

	return c.Render("admin/dashboard/index", fiber.Map{
		"ShowNav":            true,
		"PageType":           "dashboard",
		"Title":              "Dashboard - Live " + time.Now().Format("15:04:05"),
		"ProductCount":       stats.TotalProducts,
		"CustomerCount":      stats.TotalCustomers,
		"TotalLicenseCount":  stats.TotalLicenses,
		"ActiveLicenseCount": stats.ActiveLicenses,
		"RecentLicenses":     recentLicenses,
		"CacheBuster":        timestamp,
		"CurrentTime":        time.Now().Format("2006-01-02 15:04:05"),
	})
}

// Email Configuration
func (h *DashboardHandler) EmailConfigPage(c *fiber.Ctx) error {
	var settings models.EmailSettings
	
	// Try to get active email settings
	activeSettings, err := models.GetActiveEmailSettings(h.db)
	if err != nil {
		// No active settings found, show empty form
		settings = models.EmailSettings{
			SMTPPort: 587,
			SMTPEncryption: "tls",
		}
	} else {
		settings = *activeSettings
	}

	return c.Render("admin/email-config", fiber.Map{
		"ShowNav":   true,
		"Config":    settings,
		"CSRFToken": "",
	})
}

func (h *DashboardHandler) EmailConfigUpdate(c *fiber.Ctx) error {
	// Extract form values
	smtpHost := c.FormValue("smtp_host")
	smtpPort, _ := strconv.Atoi(c.FormValue("smtp_port"))
	smtpUsername := c.FormValue("smtp_username")
	smtpPassword := c.FormValue("smtp_password")
	smtpEncryption := c.FormValue("smtp_encryption")
	fromEmail := c.FormValue("from_email")
	fromName := c.FormValue("from_name")

	// Create or update email settings
	var settings models.EmailSettings
	activeSettings, err := models.GetActiveEmailSettings(h.db)
	if err != nil {
		// Create new settings
		settings = models.EmailSettings{
			Provider:       "smtp",
			SMTPHost:       smtpHost,
			SMTPPort:       smtpPort,
			SMTPUsername:   smtpUsername,
			SMTPPassword:   smtpPassword,
			SMTPEncryption: smtpEncryption,
			FromEmail:      fromEmail,
			FromName:       fromName,
			IsActive:       true,
		}
	} else {
		// Update existing settings
		settings = *activeSettings
		settings.SMTPHost = smtpHost
		settings.SMTPPort = smtpPort
		settings.SMTPUsername = smtpUsername
		settings.SMTPPassword = smtpPassword
		settings.SMTPEncryption = smtpEncryption
		settings.FromEmail = fromEmail
		settings.FromName = fromName
	}

	// Save to database
	if err := settings.Save(h.db); err != nil {
		return c.Render("admin/email-config", fiber.Map{
			"ShowNav":   true,
			"Error":     fmt.Sprintf("Failed to save email configuration: %v", err),
			"Config":    settings,
			"CSRFToken": "",
		})
	}

	return c.Render("admin/email-config", fiber.Map{
		"ShowNav":   true,
		"Success":   "Email configuration saved successfully",
		"Config":    settings,
		"CSRFToken": "",
	})
}

func (h *DashboardHandler) EmailTestSend(c *fiber.Ctx) error {
	testEmail := c.FormValue("test_email")
	if testEmail == "" {
		settings, _ := models.GetActiveEmailSettings(h.db)
		if settings == nil {
			settings = &models.EmailSettings{}
		}
		return c.Render("admin/email-config", fiber.Map{
			"ShowNav":   true,
			"Error":     "Please enter a test email address",
			"Config":    *settings,
			"CSRFToken": "",
		})
	}

	// Get current settings for display
	settings, err := models.GetActiveEmailSettings(h.db)
	if err != nil {
		return c.Render("admin/email-config", fiber.Map{
			"ShowNav":   true,
			"Error":     "No email configuration found. Please configure email settings first.",
			"Config":    models.EmailSettings{},
			"CSRFToken": "",
		})
	}

	// Send a test email
	cfg := config.New()
	emailService := services.NewEmailService(cfg, h.db)
	err = emailService.SendTestEmail(testEmail)
	if err != nil {
		return c.Render("admin/email-config", fiber.Map{
			"ShowNav":   true,
			"Error":     fmt.Sprintf("Failed to send test email: %v", err),
			"Config":    *settings,
			"CSRFToken": "",
		})
	}

	return c.Render("admin/email-config", fiber.Map{
		"ShowNav":   true,
		"Success":   fmt.Sprintf("Test email sent successfully to %s", testEmail),
		"Config":    *settings,
		"CSRFToken": "",
	})
}