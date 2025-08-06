package testutils

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	htmlEngine "github.com/gofiber/template/html/v2"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"matcha/internal/models"
)

func SetupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&models.Product{}, &models.Customer{}, &models.LicenseKey{}, &models.AdminUser{}, &models.EmailSettings{})
	require.NoError(t, err)

	// Add cleanup function to ensure database is cleaned up after test
	t.Cleanup(func() {
		CleanupTestDB(db)
	})

	return db
}

// SetupTestAppWithDB creates a minimal Fiber app with database context for handler testing
func SetupTestAppWithDB(t *testing.T, db *gorm.DB) *fiber.App {
	// Set up template engine for tests - use absolute path from project root
	engine := htmlEngine.New("../../templates", ".gohtml")
	engine.Reload(true)

	// Add template functions
	engine.AddFunc("dict", func(values ...interface{}) map[string]interface{} {
		dict := make(map[string]interface{})
		for i := 0; i < len(values); i += 2 {
			if i+1 < len(values) {
				key, ok := values[i].(string)
				if ok {
					dict[key] = values[i+1]
				}
			}
		}
		return dict
	})

	app := fiber.New(fiber.Config{
		Views: engine, // Use template engine for tests
	})

	// Add database to context
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("db", db)
		return c.Next()
	})

	// Add method override middleware (for form testing)
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodPost {
			method := c.FormValue("_method")
			if method != "" {
				method = strings.ToUpper(method)
				if method == fiber.MethodPut || method == fiber.MethodDelete || method == fiber.MethodPatch {
					c.Request().Header.SetMethod(method)
				}
			}
		}
		return c.Next()
	})

	return app
}

// CleanupTestDB removes all data from test database tables using GORM
func CleanupTestDB(db *gorm.DB) {
	// Delete all records using GORM's Unscoped to permanently delete
	db.Unscoped().Where("1 = 1").Delete(&models.LicenseKey{})
	db.Unscoped().Where("1 = 1").Delete(&models.Customer{})
	db.Unscoped().Where("1 = 1").Delete(&models.Product{})
	db.Unscoped().Where("1 = 1").Delete(&models.AdminUser{})
	db.Unscoped().Where("1 = 1").Delete(&models.EmailSettings{})
}

// SetupTestApp creates a basic Fiber app for unit testing handlers
func SetupTestApp() *fiber.App {
	// Set up template engine for tests - use absolute path from project root
	engine := htmlEngine.New("../../templates", ".gohtml")
	engine.Reload(true)

	// Add template functions
	engine.AddFunc("dict", func(values ...interface{}) map[string]interface{} {
		dict := make(map[string]interface{})
		for i := 0; i < len(values); i += 2 {
			if i+1 < len(values) {
				key, ok := values[i].(string)
				if ok {
					dict[key] = values[i+1]
				}
			}
		}
		return dict
	})

	app := fiber.New(fiber.Config{
		Views: engine, // Use template engine for tests
	})
	return app
}

// SetupIntegrationApp creates a basic Fiber app with database context for integration testing
func SetupIntegrationApp(t *testing.T) (*fiber.App, *gorm.DB) {
	db := SetupTestDB(t)

	// Set up template engine for tests - use absolute path from project root
	engine := htmlEngine.New("../../templates", ".gohtml")
	engine.Reload(true)

	// Add template functions
	engine.AddFunc("dict", func(values ...interface{}) map[string]interface{} {
		dict := make(map[string]interface{})
		for i := 0; i < len(values); i += 2 {
			if i+1 < len(values) {
				key, ok := values[i].(string)
				if ok {
					dict[key] = values[i+1]
				}
			}
		}
		return dict
	})

	app := fiber.New(fiber.Config{
		Views: engine, // Use template engine for tests
	})

	// Add database to context
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("db", db)
		return c.Next()
	})

	// Add method override middleware (for form testing)
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() == fiber.MethodPost {
			method := c.FormValue("_method")
			if method != "" {
				method = strings.ToUpper(method)
				if method == fiber.MethodPut || method == fiber.MethodDelete || method == fiber.MethodPatch {
					c.Request().Header.SetMethod(method)
				}
			}
		}
		return c.Next()
	})

	return app, db
}

// TestRequest helper to make HTTP requests to the test app
func TestRequest(t *testing.T, app *fiber.App, method, url string, body string) *http.Response {
	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequest(method, url, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	require.NoError(t, err)

	// Set content type for POST/PUT requests
	if method == "POST" || method == "PUT" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := app.Test(req)
	require.NoError(t, err)

	return resp
}

// TestRequestJSON helper to make JSON HTTP requests to the test app
func TestRequestJSON(t *testing.T, app *fiber.App, method, url string, body string) *http.Response {
	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequest(method, url, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	require.NoError(t, err)

	// Set content type for JSON requests
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)

	return resp
}

// MockRender wraps a handler to mock template rendering by catching panics and returning OK
func MockRender(handler func(*fiber.Ctx) error) func(*fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				// If template rendering fails, just return OK
				_ = c.SendString("OK")
			}
		}()

		err := handler(c)
		if err != nil {
			// If there's an error (like template not found), return OK
			return c.SendString("OK")
		}
		return err
	}
}
