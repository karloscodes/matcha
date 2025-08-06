package handlers

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"license-key-manager/internal/config"
	"license-key-manager/internal/models"
	"license-key-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type SettingsHandler struct {
	db *gorm.DB
}

func NewSettingsHandler(db *gorm.DB) *SettingsHandler {
	return &SettingsHandler{db: db}
}

// ShowEmailSettings displays the email configuration settings
func (h *SettingsHandler) ShowEmailSettings(c *fiber.Ctx) error {
	var emailSettings []models.EmailSettings
	if err := h.db.Find(&emailSettings).Error; err != nil {
		log.Printf("Error fetching email settings: %v", err)
		return c.Status(500).Render("layouts/base", fiber.Map{
			"ShowNav":       true,
			"PageType":      "email-settings",
			"Title":         "Email Settings",
			"Error":         "Failed to load email settings",
			"EmailSettings": emailSettings,
		})
	}

	return c.Render("layouts/base", fiber.Map{
		"ShowNav":       true,
		"PageType":      "email-settings",
		"Title":         "Email Settings",
		"EmailSettings": emailSettings,
	})
}

// CreateEmailSettings creates a new email configuration
func (h *SettingsHandler) CreateEmailSettings(c *fiber.Ctx) error {
	provider := c.FormValue("provider")
	smtpHost := c.FormValue("smtp_host")
	smtpUsername := c.FormValue("smtp_username")
	smtpPassword := c.FormValue("smtp_password")
	fromEmail := c.FormValue("from_email")
	fromName := c.FormValue("from_name")
	smtpEncryption := c.FormValue("smtp_encryption")

	smtpPort, err := strconv.Atoi(c.FormValue("smtp_port"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid SMTP port",
		})
	}

	// Deactivate all existing settings
	if err := h.db.Model(&models.EmailSettings{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
		log.Printf("Error deactivating existing email settings: %v", err)
		return c.Status(500).Render("admin/settings/email", fiber.Map{
			"Error": "Failed to update email settings",
		})
	}

	// Create new settings
	emailSettings := models.EmailSettings{
		Provider:       provider,
		SMTPHost:       smtpHost,
		SMTPPort:       smtpPort,
		SMTPUsername:   smtpUsername,
		SMTPPassword:   smtpPassword,
		SMTPEncryption: smtpEncryption,
		FromEmail:      fromEmail,
		FromName:       fromName,
		IsActive:       true,
	}

	if err := h.db.Create(&emailSettings).Error; err != nil {
		log.Printf("Error creating email settings: %v", err)
		return c.Status(500).Render("admin/settings/email", fiber.Map{
			"Error": "Failed to save email settings",
		})
	}

	return c.Redirect("/admin/settings/email")
}

// UpdateEmailSettings updates an existing email configuration
func (h *SettingsHandler) UpdateEmailSettings(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).Render("admin/settings/email", fiber.Map{
			"Error": "Invalid settings ID",
		})
	}

	var emailSettings models.EmailSettings
	if err := h.db.First(&emailSettings, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(404).JSON(fiber.Map{
				"error": "Email settings not found",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to load email settings",
		})
	}

	// Update fields
	emailSettings.Provider = c.FormValue("provider")
	emailSettings.SMTPHost = c.FormValue("smtp_host")
	emailSettings.SMTPUsername = c.FormValue("smtp_username")
	emailSettings.SMTPPassword = c.FormValue("smtp_password")
	emailSettings.FromEmail = c.FormValue("from_email")
	emailSettings.FromName = c.FormValue("from_name")
	emailSettings.SMTPEncryption = c.FormValue("smtp_encryption")

	smtpPort, err := strconv.Atoi(c.FormValue("smtp_port"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid SMTP port",
		})
	}
	emailSettings.SMTPPort = smtpPort

	if err := h.db.Save(&emailSettings).Error; err != nil {
		log.Printf("Error updating email settings: %v", err)
		return c.Status(500).Render("admin/settings/email", fiber.Map{
			"Error": "Failed to update email settings",
		})
	}

	return c.Redirect("/admin/settings/email")
}

// ActivateEmailSettings activates a specific email configuration
func (h *SettingsHandler) ActivateEmailSettings(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid settings ID"})
	}

	// Deactivate all settings
	if err := h.db.Model(&models.EmailSettings{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
		log.Printf("Error deactivating email settings: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update settings"})
	}

	// Activate the selected settings
	if err := h.db.Model(&models.EmailSettings{}).Where("id = ?", uint(id)).Update("is_active", true).Error; err != nil {
		log.Printf("Error activating email settings: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to activate settings"})
	}

	return c.Redirect("/admin/settings/email")
}

// DeleteEmailSettings deletes an email configuration
func (h *SettingsHandler) DeleteEmailSettings(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid settings ID"})
	}

	if err := h.db.Delete(&models.EmailSettings{}, uint(id)).Error; err != nil {
		log.Printf("Error deleting email settings: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete settings"})
	}

	return c.Redirect("/admin/settings/email")
}

// TestEmailSettings sends a test email using active configuration
func (h *SettingsHandler) TestEmailSettings(c *fiber.Ctx) error {
	testEmail := c.FormValue("test_email")
	if testEmail == "" {
		var emailSettings []models.EmailSettings
		h.db.Find(&emailSettings)

		return c.Render("layouts/base", fiber.Map{
			"ShowNav":       true,
			"PageType":      "email-settings",
			"Title":         "Email Settings",
			"Error":         "Please enter a test email address",
			"EmailSettings": emailSettings,
		})
	}

	// Check if we have active settings
	_, err := models.GetActiveEmailSettings(h.db)
	if err != nil {
		var emailSettings []models.EmailSettings
		h.db.Find(&emailSettings)

		return c.Render("layouts/base", fiber.Map{
			"ShowNav":       true,
			"PageType":      "email-settings",
			"Title":         "Email Settings",
			"Error":         "No active email configuration found. Please activate a configuration first.",
			"EmailSettings": emailSettings,
		})
	}

	// Send a test email
	cfg := config.New()
	emailService := services.NewEmailService(cfg, h.db)
	err = emailService.SendTestEmail(testEmail)

	// Get all settings for display
	var emailSettings []models.EmailSettings
	h.db.Find(&emailSettings)

	if err != nil {
		return c.Render("layouts/base", fiber.Map{
			"ShowNav":       true,
			"PageType":      "email-settings",
			"Title":         "Email Settings",
			"Error":         fmt.Sprintf("Failed to send test email: %v", err),
			"EmailSettings": emailSettings,
		})
	}

	return c.Render("layouts/base", fiber.Map{
		"ShowNav":       true,
		"PageType":      "email-settings",
		"Title":         "Email Settings",
		"Success":       fmt.Sprintf("Test email sent successfully to %s", testEmail),
		"EmailSettings": emailSettings,
	})
}
