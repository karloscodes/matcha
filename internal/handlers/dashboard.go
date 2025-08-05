package handlers

import (
	"fmt"
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
	// Read current config (in a real app, you'd save this to database)
	cfg := config.New()

	return c.Render("admin/email-config", fiber.Map{
		"ShowNav":   true,
		"Config":    cfg,
		"CSRFToken": "",
	})
}

func (h *DashboardHandler) EmailConfigUpdate(c *fiber.Ctx) error {
	// In a real application, you would save these to database
	// For now, we'll show how the form would work

	emailService := c.FormValue("email_service")
	fromEmail := c.FormValue("from_email")

	message := fmt.Sprintf("Email configuration updated: Service=%s, From=%s", emailService, fromEmail)

	return c.Render("admin/email-config", fiber.Map{
		"ShowNav":   true,
		"Success":   message,
		"Config":    config.New(), // In reality, you'd load the updated config
		"CSRFToken": "",
	})
}

func (h *DashboardHandler) EmailTestSend(c *fiber.Ctx) error {
	testEmail := c.FormValue("test_email")
	if testEmail == "" {
		return c.Render("admin/email-config", fiber.Map{
			"ShowNav":   true,
			"Error":     "Please enter a test email address",
			"Config":    config.New(),
			"CSRFToken": "",
		})
	}

	// Actually send a test email
	emailService := services.NewEmailService(config.New())
	err := emailService.SendTestEmail(testEmail)
	if err != nil {
		return c.Render("admin/email-config", fiber.Map{
			"ShowNav":   true,
			"Error":     fmt.Sprintf("Failed to send test email: %v", err),
			"Config":    config.New(),
			"CSRFToken": "",
		})
	}

	return c.Render("admin/email-config", fiber.Map{
		"ShowNav":   true,
		"Success":   fmt.Sprintf("Test email sent successfully to %s", testEmail),
		"Config":    config.New(),
		"CSRFToken": "",
	})
}