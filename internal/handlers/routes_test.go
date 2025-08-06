package handlers

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"license-key-manager/internal/models"
	"license-key-manager/internal/testutils"
)

func setupTestRoutes() (*fiber.App, *gorm.DB) {
	app := testutils.SetupTestApp()
	db := testutils.SetupTestDB(&testing.T{})

	// Initialize handlers
	dashboardHandler := NewDashboardHandler(db)
	usersHandler := NewUsersHandler(db)
	productsHandler := NewProductsHandler(db)
	customersHandler := NewCustomersHandler(db)
	licenseKeysHandler := NewLicenseKeysHandler(db)

	// Setup routes without middleware to avoid auth issues in tests
	admin := app.Group("/admin")

	// Login routes
	admin.Get("/login", testutils.MockRender(usersHandler.LoginPage))
	admin.Post("/login", testutils.MockRender(usersHandler.Login))
	admin.Get("/logout", usersHandler.Logout)

	// Dashboard
	admin.Get("/", testutils.MockRender(dashboardHandler.Dashboard))

	// Products
	admin.Get("/products", testutils.MockRender(productsHandler.Index))
	admin.Get("/products/new", testutils.MockRender(productsHandler.New))
	admin.Post("/products", productsHandler.Create)
	admin.Get("/products/:id", testutils.MockRender(productsHandler.Show))
	admin.Get("/products/:id/edit", testutils.MockRender(productsHandler.Edit))
	admin.Put("/products/:id", productsHandler.Update)
	admin.Post("/products/:id", productsHandler.Update) // For form method override
	admin.Delete("/products/:id", productsHandler.Delete)

	// Customers
	admin.Get("/customers", testutils.MockRender(customersHandler.Index))
	admin.Get("/customers/new", testutils.MockRender(customersHandler.New))
	admin.Post("/customers", customersHandler.Create)
	admin.Get("/customers/:id", testutils.MockRender(customersHandler.Show))
	admin.Get("/customers/:id/edit", testutils.MockRender(customersHandler.Edit))
	admin.Put("/customers/:id", customersHandler.Update)
	admin.Post("/customers/:id", customersHandler.Update) // For form method override
	admin.Delete("/customers/:id", customersHandler.Delete)

	// License Keys
	admin.Get("/license-keys", testutils.MockRender(licenseKeysHandler.Index))
	admin.Get("/license-keys/new", testutils.MockRender(licenseKeysHandler.New))
	admin.Post("/license-keys", licenseKeysHandler.Create)
	admin.Get("/license-keys/:id", testutils.MockRender(licenseKeysHandler.Show))
	admin.Get("/license-keys/:id/edit", testutils.MockRender(licenseKeysHandler.Edit))
	admin.Put("/license-keys/:id", licenseKeysHandler.Update)
	admin.Post("/license-keys/:id", licenseKeysHandler.Update) // For form method override
	admin.Delete("/license-keys/:id", licenseKeysHandler.Delete)
	admin.Post("/license-keys/:id/revoke", licenseKeysHandler.Revoke)
	admin.Post("/license-keys/:id/reactivate", licenseKeysHandler.Reactivate)
	admin.Post("/license-keys/:id/send-email", licenseKeysHandler.SendEmail)

	// Email Configuration
	admin.Get("/email-config", testutils.MockRender(dashboardHandler.EmailConfigPage))
	admin.Post("/email-config", testutils.MockRender(dashboardHandler.EmailConfigUpdate))
	admin.Post("/email-config/test", testutils.MockRender(dashboardHandler.EmailTestSend))

	return app, db
}

func TestRoutes_Dashboard(t *testing.T) {
	app, _ := setupTestRoutes()

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/admin/", 200},
		{"GET", "/admin/login", 200},
		{"GET", "/admin/logout", 302}, // Redirects to login
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tt.status, resp.StatusCode)
		})
	}
}

func TestRoutes_Products(t *testing.T) {
	app, db := setupTestRoutes()

	// Create a test product for Show and Edit tests
	product := models.Product{
		Name:                  "Test Product",
		Description:           "Test Description",
		Version:               "1.0.0",
		DefaultExpirationDays: 365,
		DefaultUsageLimit:     1,
	}
	db.Create(&product)

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/admin/products", 200},
		{"GET", "/admin/products/new", 200},
		{"GET", "/admin/products/" + strconv.Itoa(int(product.ID)), 200},           // Existing product should return 200
		{"GET", "/admin/products/" + strconv.Itoa(int(product.ID)) + "/edit", 200}, // Existing product should return 200
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tt.status, resp.StatusCode)
		})
	}
}

func TestRoutes_Customers(t *testing.T) {
	app, db := setupTestRoutes()

	// Create a test customer for Show and Edit tests
	customer := models.Customer{
		Name:      "John Doe",
		Email:     "john@example.com",
		FirstName: "John",
		LastName:  "Doe",
	}
	db.Create(&customer)

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/admin/customers", 200},
		{"GET", "/admin/customers/new", 200},
		{"GET", "/admin/customers/" + strconv.Itoa(int(customer.ID)), 200},           // Existing customer should return 200
		{"GET", "/admin/customers/" + strconv.Itoa(int(customer.ID)) + "/edit", 200}, // Existing customer should return 200
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tt.status, resp.StatusCode)
		})
	}
}

func TestRoutes_LicenseKeys(t *testing.T) {
	app, db := setupTestRoutes()

	// Create test data for license key tests (product and customer first)
	product := models.Product{
		Name:                  "Test Product",
		Description:           "Test Description",
		Version:               "1.0.0",
		DefaultExpirationDays: 365,
		DefaultUsageLimit:     1,
	}
	db.Create(&product)

	customer := models.Customer{
		Name:      "Jane Doe",
		Email:     "jane@example.com",
		FirstName: "Jane",
		LastName:  "Doe",
	}
	db.Create(&customer)

	// Create a test license key for Show and Edit tests
	licenseKey, _ := product.GenerateLicenseKeyFor(db, &customer)

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/admin/license-keys", 200},
		{"GET", "/admin/license-keys/new", 200},
		{"GET", "/admin/license-keys/" + strconv.Itoa(int(licenseKey.ID)), 200},           // Existing license key should return 200
		{"GET", "/admin/license-keys/" + strconv.Itoa(int(licenseKey.ID)) + "/edit", 200}, // Existing license key should return 200
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tt.status, resp.StatusCode)
		})
	}
}

func TestRoutes_EmailConfig(t *testing.T) {
	app, _ := setupTestRoutes()

	tests := []struct {
		method string
		path   string
		status int
	}{
		{"GET", "/admin/email-config", 200},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tt.status, resp.StatusCode)
		})
	}
}
