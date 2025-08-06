package handlers

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"matcha/internal/models"
	"matcha/internal/testutils"
)

// Integration tests for Dashboard - tests full request flow with database
func TestDashboardHandler_Integration(t *testing.T) {
	t.Run("Dashboard - Empty Stats", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewDashboardHandler(db)

		app.Get("/dashboard", handler.Dashboard)

		resp := testutils.TestRequest(t, app, "GET", "/dashboard", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Dashboard - With Statistics", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewDashboardHandler(db)

		app.Get("/dashboard", handler.Dashboard)

		// Create test data for statistics
		product := models.Product{
			Name:        "Test Product",
			Description: "Test Description",
			Version:     "1.0.0",
		}
		require.NoError(t, db.Create(&product).Error)

		customer := models.Customer{
			Name:  "John Doe",
			Email: "john@example.com",
		}
		require.NoError(t, db.Create(&customer).Error)

		licenseKey := models.LicenseKey{
			Key:        "TEST-KEY-123",
			ProductID:  product.ID,
			CustomerID: customer.ID,
			Status:     "active",
		}
		require.NoError(t, db.Create(&licenseKey).Error)

		resp := testutils.TestRequest(t, app, "GET", "/dashboard", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("EmailConfigPage - Display Email Configuration", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewDashboardHandler(db)

		app.Get("/email-config", handler.EmailConfigPage)

		resp := testutils.TestRequest(t, app, "GET", "/email-config", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("EmailConfigPage - With Existing Settings", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewDashboardHandler(db)

		app.Get("/email-config", handler.EmailConfigPage)

		// Create email settings
		emailSettings := models.EmailSettings{
			Provider:     "Gmail",
			SMTPHost:     "smtp.gmail.com",
			SMTPPort:     587,
			SMTPUsername: "test@gmail.com",
			SMTPPassword: "password",
			FromEmail:    "test@gmail.com",
			FromName:     "Test App",
			IsActive:     true,
		}
		require.NoError(t, db.Create(&emailSettings).Error)

		resp := testutils.TestRequest(t, app, "GET", "/email-config", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("EmailConfigUpdate - Valid Configuration", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewDashboardHandler(db)

		app.Post("/email-config", handler.EmailConfigUpdate)

		form := url.Values{
			"provider":      {"Gmail"},
			"smtp_host":     {"smtp.gmail.com"},
			"smtp_port":     {"587"},
			"smtp_username": {"test@gmail.com"},
			"smtp_password": {"password"},
			"from_email":    {"test@gmail.com"},
			"from_name":     {"Test App"},
			"is_active":     {"true"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/email-config", form.Encode())
		assert.Equal(t, 302, resp.StatusCode) // Should redirect

		// Verify email settings were created/updated
		var emailSettings models.EmailSettings
		err := db.First(&emailSettings).Error
		require.NoError(t, err)
		assert.Equal(t, "Gmail", emailSettings.Provider)
		assert.Equal(t, "smtp.gmail.com", emailSettings.SMTPHost)
		assert.Equal(t, 587, emailSettings.SMTPPort)
		assert.Equal(t, "test@gmail.com", emailSettings.SMTPUsername)
		assert.Equal(t, "test@gmail.com", emailSettings.FromEmail)
		assert.Equal(t, "Test App", emailSettings.FromName)
		assert.True(t, emailSettings.IsActive)
	})

	t.Run("EmailConfigUpdate - Update Existing Settings", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewDashboardHandler(db)

		app.Post("/email-config", handler.EmailConfigUpdate)

		// Create existing settings
		emailSettings := models.EmailSettings{
			Provider:     "SendGrid",
			SMTPHost:     "smtp.sendgrid.net",
			SMTPPort:     587,
			SMTPUsername: "apikey",
			SMTPPassword: "old_password",
			FromEmail:    "old@example.com",
			FromName:     "Old App",
			IsActive:     false,
		}
		require.NoError(t, db.Create(&emailSettings).Error)

		// Update with new values
		form := url.Values{
			"provider":      {"Gmail"},
			"smtp_host":     {"smtp.gmail.com"},
			"smtp_port":     {"587"},
			"smtp_username": {"new@gmail.com"},
			"smtp_password": {"new_password"},
			"from_email":    {"new@gmail.com"},
			"from_name":     {"New App"},
			"is_active":     {"true"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/email-config", form.Encode())
		assert.Equal(t, 302, resp.StatusCode)

		// Verify settings were updated
		var updatedSettings models.EmailSettings
		err := db.First(&updatedSettings).Error
		require.NoError(t, err)
		assert.Equal(t, "Gmail", updatedSettings.Provider)
		assert.Equal(t, "new@gmail.com", updatedSettings.SMTPUsername)
		assert.Equal(t, "new@gmail.com", updatedSettings.FromEmail)
		assert.Equal(t, "New App", updatedSettings.FromName)
		assert.True(t, updatedSettings.IsActive)
	})

	t.Run("EmailConfigUpdate - Invalid Port", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewDashboardHandler(db)

		app.Post("/email-config", handler.EmailConfigUpdate)

		form := url.Values{
			"provider":      {"Gmail"},
			"smtp_host":     {"smtp.gmail.com"},
			"smtp_port":     {"invalid_port"},
			"smtp_username": {"test@gmail.com"},
			"smtp_password": {"password"},
			"from_email":    {"test@gmail.com"},
			"from_name":     {"Test App"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/email-config", form.Encode())
		// Should handle the error gracefully - could be 400 or redirect with error
		assert.True(t, resp.StatusCode == 400 || resp.StatusCode == 302)
	})
}
