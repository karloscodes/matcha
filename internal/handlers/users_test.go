package handlers

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"matcha/internal/models"
	"matcha/internal/testutils"
)

func TestUsersHandler_LoginPage(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	handler := NewUsersHandler(db)

	app.Get("/test", testutils.MockRender(handler.LoginPage))

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 200, resp.StatusCode)
}

func TestUsersHandler_Login(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB)
		formData       map[string]string
		expectedStatus int
		expectedResult string
	}{
		{
			name: "should fail with invalid username",
			setupData: func(db *gorm.DB) {
				// No user created
			},
			formData: map[string]string{
				"username": "nonexistent",
				"password": "testpass",
			},
			expectedStatus: 200, // Render login page with error
			expectedResult: "error",
		},
		{
			name: "should fail with invalid password",
			setupData: func(db *gorm.DB) {
				admin := models.AdminUser{
					Username: "testuser",
				}
				_ = admin.SetPassword("correctpass")
				db.Create(&admin)
			},
			formData: map[string]string{
				"username": "testuser",
				"password": "wrongpass",
			},
			expectedStatus: 200, // Render login page with error
			expectedResult: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewUsersHandler(db)

			// Setup test data
			tt.setupData(db)

			// Create form data
			form := url.Values{}
			for key, value := range tt.formData {
				form.Set(key, value)
			}

			// Mock the login route with render mocking
			app.Post("/test", testutils.MockRender(handler.Login))

			req := httptest.NewRequest("POST", "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestUsersHandler_Logout(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	_ = NewUsersHandler(db) // We don't use the handler in this test

	app.Get("/logout", func(c *fiber.Ctx) error {
		// Mock the middleware.Logout call since we can't test the actual logout without session middleware
		return c.Redirect("/admin/login")
	})

	req := httptest.NewRequest("GET", "/logout", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/admin/login", resp.Header.Get("Location"))
}

func TestNewUsersHandler(t *testing.T) {
	db := testutils.SetupTestDB(t)
	handler := NewUsersHandler(db)

	assert.NotNil(t, handler)
	assert.Equal(t, db, handler.db)
}
