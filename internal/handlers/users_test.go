package handlers

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"matcha/internal/models"
	"matcha/internal/testutils"
)

// Integration tests for Users - tests full request flow with database
func TestUsersHandler_Integration(t *testing.T) {
	t.Run("LoginPage - Display Login Form", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewUsersHandler(db)

		app.Get("/login", handler.LoginPage)

		resp := testutils.TestRequest(t, app, "GET", "/login", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Login - Valid Credentials", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewUsersHandler(db)

		app.Post("/login", handler.Login)

		// Create test admin user
		admin := models.AdminUser{
			Username: "testuser",
		}
		require.NoError(t, admin.SetPassword("testpass"))
		require.NoError(t, db.Create(&admin).Error)

		form := url.Values{
			"username": {"testuser"},
			"password": {"testpass"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/login", form.Encode())
		// Should redirect on successful login
		assert.Equal(t, 302, resp.StatusCode)
	})

	t.Run("Login - Invalid Username", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewUsersHandler(db)

		app.Post("/login", handler.Login)

		form := url.Values{
			"username": {"nonexistent"},
			"password": {"testpass"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/login", form.Encode())
		// Should render login page with error (200) or redirect back (302)
		assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 302)
	})

	t.Run("Login - Invalid Password", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewUsersHandler(db)

		app.Post("/login", handler.Login)

		// Create test admin user
		admin := models.AdminUser{
			Username: "testuser",
		}
		require.NoError(t, admin.SetPassword("correctpass"))
		require.NoError(t, db.Create(&admin).Error)

		form := url.Values{
			"username": {"testuser"},
			"password": {"wrongpass"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/login", form.Encode())
		// Should render login page with error (200) or redirect back (302)
		assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 302)
	})

	t.Run("Login - Empty Credentials", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewUsersHandler(db)

		app.Post("/login", handler.Login)

		form := url.Values{
			"username": {""},
			"password": {""},
		}

		resp := testutils.TestRequest(t, app, "POST", "/login", form.Encode())
		// Should handle empty credentials gracefully
		assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 302 || resp.StatusCode == 400)
	})

	t.Run("Logout - Redirect to Login", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewUsersHandler(db)

		app.Get("/logout", handler.Logout)

		resp := testutils.TestRequest(t, app, "GET", "/logout", "")
		// Should redirect to login page
		assert.Equal(t, 302, resp.StatusCode)
	})

	t.Run("Database Verification - User Creation", func(t *testing.T) {
		db := testutils.SetupTestDB(t)

		// Test user creation and password verification
		admin := models.AdminUser{
			Username: "integration_test_user",
		}
		require.NoError(t, admin.SetPassword("integration_test_pass"))
		require.NoError(t, db.Create(&admin).Error)

		// Verify user was created
		var retrievedAdmin models.AdminUser
		err := db.Where("username = ?", "integration_test_user").First(&retrievedAdmin).Error
		require.NoError(t, err)
		assert.Equal(t, "integration_test_user", retrievedAdmin.Username)

		// Verify password verification works
		assert.True(t, retrievedAdmin.CheckPassword("integration_test_pass"))
		assert.False(t, retrievedAdmin.CheckPassword("wrong_password"))
	})
}
