package handlers

import (
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"matcha/internal/models"
	"matcha/internal/testutils"
)

// Integration tests for Settings - tests full request flow with database
func TestSettingsHandler_Integration(t *testing.T) {
	t.Run("ShowEmailSettings - Empty List", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Get("/email-settings", handler.ShowEmailSettings)

		resp := testutils.TestRequest(t, app, "GET", "/email-settings", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("ShowEmailSettings - With Data", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Get("/email-settings", handler.ShowEmailSettings)

		// Create test email settings
		settings := models.EmailSettings{
			Provider:     "Gmail",
			SMTPHost:     "smtp.gmail.com",
			SMTPPort:     587,
			SMTPUsername: "test@gmail.com",
			SMTPPassword: "password",
			FromEmail:    "test@gmail.com",
			FromName:     "Test App",
			IsActive:     true,
		}
		require.NoError(t, db.Create(&settings).Error)

		resp := testutils.TestRequest(t, app, "GET", "/email-settings", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("CreateEmailSettings - Valid Settings", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Post("/email-settings", handler.CreateEmailSettings)

		form := url.Values{
			"provider":        {"Gmail"},
			"smtp_host":       {"smtp.gmail.com"},
			"smtp_port":       {"587"},
			"smtp_username":   {"test@gmail.com"},
			"smtp_password":   {"password"},
			"from_email":      {"test@gmail.com"},
			"from_name":       {"Test App"},
			"smtp_encryption": {"tls"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/email-settings", form.Encode())
		assert.Equal(t, 302, resp.StatusCode) // Should redirect

		// Verify email settings were created
		var emailSettings models.EmailSettings
		err := db.First(&emailSettings).Error
		require.NoError(t, err)
		assert.Equal(t, "Gmail", emailSettings.Provider)
		assert.Equal(t, "smtp.gmail.com", emailSettings.SMTPHost)
		assert.Equal(t, 587, emailSettings.SMTPPort)
		assert.Equal(t, "test@gmail.com", emailSettings.SMTPUsername)
		assert.Equal(t, "test@gmail.com", emailSettings.FromEmail)
		assert.Equal(t, "Test App", emailSettings.FromName)
	})

	t.Run("CreateEmailSettings - Invalid Port", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Post("/email-settings", handler.CreateEmailSettings)

		form := url.Values{
			"provider":      {"Gmail"},
			"smtp_host":     {"smtp.gmail.com"},
			"smtp_port":     {"invalid_port"},
			"smtp_username": {"test@gmail.com"},
			"smtp_password": {"password"},
			"from_email":    {"test@gmail.com"},
			"from_name":     {"Test App"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/email-settings", form.Encode())
		// Should handle error gracefully - could be 400 or redirect with error
		assert.True(t, resp.StatusCode == 400 || resp.StatusCode == 302)
	})

	t.Run("UpdateEmailSettings - Valid Update", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Put("/email-settings/:id", handler.UpdateEmailSettings)

		// Create existing settings
		settings := models.EmailSettings{
			Provider:     "Gmail",
			SMTPHost:     "smtp.gmail.com",
			SMTPPort:     587,
			SMTPUsername: "test@gmail.com",
			SMTPPassword: "password",
			FromEmail:    "test@gmail.com",
			FromName:     "Test App",
			IsActive:     true,
		}
		require.NoError(t, db.Create(&settings).Error)

		// Update settings
		form := url.Values{
			"provider":        {"Updated Gmail"},
			"smtp_host":       {"smtp.gmail.com"},
			"smtp_port":       {"465"},
			"smtp_username":   {"updated@gmail.com"},
			"smtp_password":   {"newpassword"},
			"from_email":      {"updated@gmail.com"},
			"from_name":       {"Updated App"},
			"smtp_encryption": {"ssl"},
		}

		url := "/email-settings/" + strconv.Itoa(int(settings.ID))
		resp := testutils.TestRequest(t, app, "PUT", url, form.Encode())
		assert.Equal(t, 302, resp.StatusCode)

		// Verify settings were updated
		var updatedSettings models.EmailSettings
		err := db.First(&updatedSettings, settings.ID).Error
		require.NoError(t, err)
		assert.Equal(t, "Updated Gmail", updatedSettings.Provider)
		assert.Equal(t, 465, updatedSettings.SMTPPort)
		assert.Equal(t, "updated@gmail.com", updatedSettings.SMTPUsername)
		assert.Equal(t, "updated@gmail.com", updatedSettings.FromEmail)
		assert.Equal(t, "Updated App", updatedSettings.FromName)
	})

	t.Run("UpdateEmailSettings - Non-existent Settings", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Put("/email-settings/:id", handler.UpdateEmailSettings)

		form := url.Values{
			"provider":      {"Gmail"},
			"smtp_host":     {"smtp.gmail.com"},
			"smtp_port":     {"587"},
			"smtp_username": {"test@gmail.com"},
		}

		resp := testutils.TestRequest(t, app, "PUT", "/email-settings/999", form.Encode())
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("ActivateEmailSettings - Valid Settings", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Post("/email-settings/:id/activate", handler.ActivateEmailSettings)

		// Create inactive settings
		settings := models.EmailSettings{
			Provider:     "Gmail",
			SMTPHost:     "smtp.gmail.com",
			SMTPPort:     587,
			SMTPUsername: "test@gmail.com",
			IsActive:     false,
		}
		require.NoError(t, db.Create(&settings).Error)

		url := "/email-settings/" + strconv.Itoa(int(settings.ID)) + "/activate"
		resp := testutils.TestRequest(t, app, "POST", url, "")
		assert.Equal(t, 302, resp.StatusCode)

		// Verify settings were activated
		var activatedSettings models.EmailSettings
		err := db.First(&activatedSettings, settings.ID).Error
		require.NoError(t, err)
		assert.True(t, activatedSettings.IsActive)
	})

	t.Run("DeleteEmailSettings - Existing Settings", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Delete("/email-settings/:id", handler.DeleteEmailSettings)

		// Create settings to delete
		settings := models.EmailSettings{
			Provider:     "Gmail",
			SMTPHost:     "smtp.gmail.com",
			SMTPPort:     587,
			SMTPUsername: "test@gmail.com",
		}
		require.NoError(t, db.Create(&settings).Error)

		url := "/email-settings/" + strconv.Itoa(int(settings.ID))
		resp := testutils.TestRequest(t, app, "DELETE", url, "")
		assert.Equal(t, 302, resp.StatusCode)

		// Verify settings were deleted
		var deletedSettings models.EmailSettings
		err := db.First(&deletedSettings, settings.ID).Error
		assert.Error(t, err) // Should not find the settings
	})

	t.Run("DeleteEmailSettings - Non-existent Settings", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Delete("/email-settings/:id", handler.DeleteEmailSettings)

		resp := testutils.TestRequest(t, app, "DELETE", "/email-settings/999", "")
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("TestEmailSettings - Valid Settings", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewSettingsHandler(db)

		app.Post("/email-settings/:id/test", handler.TestEmailSettings)

		// Create valid settings
		settings := models.EmailSettings{
			Provider:     "Gmail",
			SMTPHost:     "smtp.gmail.com",
			SMTPPort:     587,
			SMTPUsername: "test@gmail.com",
			SMTPPassword: "password",
			FromEmail:    "test@gmail.com",
			FromName:     "Test App",
			IsActive:     true,
		}
		require.NoError(t, db.Create(&settings).Error)

		url := "/email-settings/" + strconv.Itoa(int(settings.ID)) + "/test"
		resp := testutils.TestRequest(t, app, "POST", url, "")
		// Test email may fail due to invalid credentials, but should handle gracefully
		assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 302 || resp.StatusCode == 400)
	})
}
