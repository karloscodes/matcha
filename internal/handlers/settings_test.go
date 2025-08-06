package handlers

import (
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"license-key-manager/internal/models"
	"license-key-manager/internal/testutils"
)

func TestSettingsHandler_ShowEmailSettings(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB)
		expectedStatus int
	}{
		{
			name: "should render email settings with empty list",
			setupData: func(db *gorm.DB) {
				// No email settings
			},
			expectedStatus: 200,
		},
		{
			name: "should render email settings with data",
			setupData: func(db *gorm.DB) {
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
				db.Create(&settings)
			},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewSettingsHandler(db)

			tt.setupData(db)

			app.Get("/test", testutils.MockRender(handler.ShowEmailSettings))

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestSettingsHandler_CreateEmailSettings(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB)
		formData       map[string]string
		expectedStatus int
	}{
		{
			name: "should create email settings successfully",
			setupData: func(db *gorm.DB) {
				// No existing settings
			},
			formData: map[string]string{
				"provider":        "Gmail",
				"smtp_host":       "smtp.gmail.com",
				"smtp_port":       "587",
				"smtp_username":   "test@gmail.com",
				"smtp_password":   "password",
				"from_email":      "test@gmail.com",
				"from_name":       "Test App",
				"smtp_encryption": "tls",
			},
			expectedStatus: 302,
		},
		{
			name: "should return 400 for invalid port",
			setupData: func(db *gorm.DB) {
				// No existing settings
			},
			formData: map[string]string{
				"provider":      "Gmail",
				"smtp_host":     "smtp.gmail.com",
				"smtp_port":     "invalid",
				"smtp_username": "test@gmail.com",
				"smtp_password": "password",
				"from_email":    "test@gmail.com",
				"from_name":     "Test App",
			},
			expectedStatus: 400,
		},
		{
			name: "should deactivate existing settings when creating new one",
			setupData: func(db *gorm.DB) {
				settings := models.EmailSettings{
					Provider: "Existing",
					IsActive: true,
				}
				db.Create(&settings)
			},
			formData: map[string]string{
				"provider":        "Gmail",
				"smtp_host":       "smtp.gmail.com",
				"smtp_port":       "587",
				"smtp_username":   "test@gmail.com",
				"smtp_password":   "password",
				"from_email":      "test@gmail.com",
				"from_name":       "Test App",
				"smtp_encryption": "tls",
			},
			expectedStatus: 302,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewSettingsHandler(db)

			tt.setupData(db)

			form := url.Values{}
			for key, value := range tt.formData {
				form.Set(key, value)
			}

			app.Post("/test", func(c *fiber.Ctx) error {
				return handler.CreateEmailSettings(c)
			})

			req := httptest.NewRequest("POST", "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify creation if successful
			if tt.expectedStatus == 302 {
				var count int64
				db.Model(&models.EmailSettings{}).Where("is_active = ?", true).Count(&count)
				assert.Equal(t, int64(1), count)
			}
		})
	}
}

func TestSettingsHandler_UpdateEmailSettings(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		formData       map[string]string
		expectedStatus int
	}{
		{
			name: "should update email settings successfully",
			setupData: func(db *gorm.DB) uint {
				settings := models.EmailSettings{
					Provider:     "Gmail",
					SMTPHost:     "smtp.gmail.com",
					SMTPPort:     587,
					SMTPUsername: "test@gmail.com",
					IsActive:     true,
				}
				db.Create(&settings)
				return settings.ID
			},
			formData: map[string]string{
				"provider":        "Updated Gmail",
				"smtp_host":       "smtp.gmail.com",
				"smtp_port":       "465",
				"smtp_username":   "updated@gmail.com",
				"smtp_password":   "newpassword",
				"from_email":      "updated@gmail.com",
				"from_name":       "Updated App",
				"smtp_encryption": "ssl",
			},
			expectedStatus: 302,
		},
		{
			name: "should return 404 for non-existent settings",
			setupData: func(db *gorm.DB) uint {
				return 999 // Non-existent ID
			},
			formData: map[string]string{
				"provider":   "Gmail",
				"smtp_host":  "smtp.gmail.com",
				"smtp_port":  "587",
				"from_email": "test@gmail.com",
			},
			expectedStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewSettingsHandler(db)

			settingsID := tt.setupData(db)

			form := url.Values{}
			for key, value := range tt.formData {
				form.Set(key, value)
			}

			app.Post("/test/:id", func(c *fiber.Ctx) error {
				return handler.UpdateEmailSettings(c)
			})

			req := httptest.NewRequest("POST", "/test/"+strconv.Itoa(int(settingsID)), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify update if successful
			if tt.expectedStatus == 302 {
				var settings models.EmailSettings
				db.First(&settings, settingsID)
				assert.Equal(t, "Updated Gmail", settings.Provider)
				assert.Equal(t, 465, settings.SMTPPort)
			}
		})
	}
}

func TestSettingsHandler_ActivateEmailSettings(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	handler := NewSettingsHandler(db)

	// Create two email settings
	settings1 := models.EmailSettings{Provider: "Gmail", IsActive: true}
	db.Create(&settings1)

	settings2 := models.EmailSettings{Provider: "SendGrid", IsActive: false}
	db.Create(&settings2)

	app.Post("/test/:id", func(c *fiber.Ctx) error {
		return handler.ActivateEmailSettings(c)
	})

	req := httptest.NewRequest("POST", "/test/"+strconv.Itoa(int(settings2.ID)), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 302, resp.StatusCode)

	// Verify activation
	var updatedSettings1, updatedSettings2 models.EmailSettings
	db.First(&updatedSettings1, settings1.ID)
	db.First(&updatedSettings2, settings2.ID)

	assert.False(t, updatedSettings1.IsActive)
	assert.True(t, updatedSettings2.IsActive)
}

func TestSettingsHandler_DeleteEmailSettings(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should delete email settings successfully",
			setupData: func(db *gorm.DB) uint {
				settings := models.EmailSettings{Provider: "Gmail"}
				db.Create(&settings)
				return settings.ID
			},
			expectedStatus: 302,
		},
		{
			name: "should handle non-existent settings gracefully",
			setupData: func(db *gorm.DB) uint {
				return 999 // Non-existent ID
			},
			expectedStatus: 302, // GORM doesn't error on delete of non-existent record
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewSettingsHandler(db)

			settingsID := tt.setupData(db)

			app.Delete("/test/:id", func(c *fiber.Ctx) error {
				return handler.DeleteEmailSettings(c)
			})

			req := httptest.NewRequest("DELETE", "/test/"+strconv.Itoa(int(settingsID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify deletion if it was a real record
			if settingsID != 999 {
				var count int64
				db.Model(&models.EmailSettings{}).Where("id = ?", settingsID).Count(&count)
				assert.Equal(t, int64(0), count)
			}
		})
	}
}

func TestNewSettingsHandler(t *testing.T) {
	db := testutils.SetupTestDB(t)
	handler := NewSettingsHandler(db)

	assert.NotNil(t, handler)
	assert.Equal(t, db, handler.db)
}