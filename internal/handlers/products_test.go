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

// Integration tests for Products - tests full request flow with database
func TestProductsHandler_Integration(t *testing.T) {
	t.Run("Index - Display Products", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Get("/products", handler.Index)

		// Test empty list
		resp := testutils.TestRequest(t, app, "GET", "/products", "")
		assert.Equal(t, 200, resp.StatusCode)

		// Create test data and test with data
		product := models.Product{
			Name:        "Test Product",
			Description: "Test Description",
			Version:     "1.0.0",
		}
		require.NoError(t, db.Create(&product).Error)

		resp = testutils.TestRequest(t, app, "GET", "/products", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("New - Display Create Form", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Get("/products/new", handler.New)

		resp := testutils.TestRequest(t, app, "GET", "/products/new", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Create - Valid Product", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Post("/products", handler.Create)

		// Test form submission
		form := url.Values{
			"name":                    {"Integration Test Product"},
			"description":             {"Test product description"},
			"version":                 {"1.0.0"},
			"default_expiration_days": {"365"},
			"default_usage_limit":     {"1"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/products", form.Encode())
		assert.Equal(t, 302, resp.StatusCode) // Should redirect

		// Verify database state
		var product models.Product
		err := db.Where("name = ?", "Integration Test Product").First(&product).Error
		require.NoError(t, err)
		assert.Equal(t, "Integration Test Product", product.Name)
		assert.Equal(t, "Test product description", product.Description)
		assert.Equal(t, "1.0.0", product.Version)
		assert.Equal(t, 365, product.DefaultExpirationDays)
		assert.Equal(t, 1, product.DefaultUsageLimit)
	})

	t.Run("Create - Invalid Product (Missing Name)", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Post("/products", handler.Create)

		form := url.Values{
			"description": {"Test product description"},
			"version":     {"1.0.0"},
		}

		resp := testutils.TestRequest(t, app, "POST", "/products", form.Encode())
		assert.Equal(t, 400, resp.StatusCode)
	})

	t.Run("Show - Existing Product", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Get("/products/:id", handler.Show)

		// Setup test data
		product := models.Product{
			Name:        "Test Product",
			Description: "Test Description",
			Version:     "1.0.0",
		}
		require.NoError(t, db.Create(&product).Error)

		resp := testutils.TestRequest(t, app, "GET", "/products/"+strconv.Itoa(int(product.ID)), "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Show - Non-existent Product", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Get("/products/:id", handler.Show)

		resp := testutils.TestRequest(t, app, "GET", "/products/999", "")
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("Edit - Existing Product", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Get("/products/:id/edit", handler.Edit)

		// Setup test data
		product := models.Product{
			Name:        "Test Product",
			Description: "Test Description",
			Version:     "1.0.0",
		}
		require.NoError(t, db.Create(&product).Error)

		resp := testutils.TestRequest(t, app, "GET", "/products/"+strconv.Itoa(int(product.ID))+"/edit", "")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Edit - Non-existent Product", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Get("/products/:id/edit", handler.Edit)

		resp := testutils.TestRequest(t, app, "GET", "/products/999/edit", "")
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("Update - Complete Update", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Put("/products/:id", handler.Update)

		// Setup test data
		product := models.Product{
			Name:                  "Original Product",
			Description:           "Original description",
			Version:               "1.0.0",
			DefaultExpirationDays: 30,
			DefaultUsageLimit:     1,
		}
		require.NoError(t, db.Create(&product).Error)

		// Test update via form submission
		form := url.Values{
			"name":                    {"Updated Product"},
			"description":             {"Updated description"},
			"version":                 {"2.0.0"},
			"default_expiration_days": {"60"},
			"default_usage_limit":     {"5"},
		}

		url := "/products/" + strconv.Itoa(int(product.ID))
		resp := testutils.TestRequest(t, app, "PUT", url, form.Encode())
		assert.Equal(t, 302, resp.StatusCode)

		// Verify database was updated
		var updatedProduct models.Product
		err := db.First(&updatedProduct, product.ID).Error
		require.NoError(t, err)
		assert.Equal(t, "Updated Product", updatedProduct.Name)
		assert.Equal(t, "Updated description", updatedProduct.Description)
		assert.Equal(t, "2.0.0", updatedProduct.Version)
		assert.Equal(t, 60, updatedProduct.DefaultExpirationDays)
		assert.Equal(t, 5, updatedProduct.DefaultUsageLimit)
	})

	t.Run("Update - Partial Update", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Put("/products/:id", handler.Update)

		// Setup test data
		product := models.Product{
			Name:        "Test Product",
			Description: "Original description",
			Version:     "1.0.0",
		}
		require.NoError(t, db.Create(&product).Error)

		// Update only description
		form := url.Values{
			"description": {"Updated description only"},
		}

		url := "/products/" + strconv.Itoa(int(product.ID))
		resp := testutils.TestRequest(t, app, "PUT", url, form.Encode())
		assert.Equal(t, 302, resp.StatusCode)

		// Verify only description was updated
		var updatedProduct models.Product
		err := db.First(&updatedProduct, product.ID).Error
		require.NoError(t, err)
		assert.Equal(t, "Updated description only", updatedProduct.Description)
		// Other fields should remain unchanged
		assert.Equal(t, "Test Product", updatedProduct.Name)
		assert.Equal(t, "1.0.0", updatedProduct.Version)
	})

	t.Run("Update - Non-existent Product", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Put("/products/:id", handler.Update)

		form := url.Values{
			"name":        {"Updated Product"},
			"description": {"Updated description"},
		}

		resp := testutils.TestRequest(t, app, "PUT", "/products/999", form.Encode())
		assert.Equal(t, 404, resp.StatusCode)
	})

	t.Run("Delete - Existing Product", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Delete("/products/:id", handler.Delete)

		// Setup test data
		product := models.Product{
			Name:        "Delete Product",
			Description: "For deletion",
			Version:     "1.0.0",
		}
		require.NoError(t, db.Create(&product).Error)

		// Test deletion
		url := "/products/" + strconv.Itoa(int(product.ID))
		resp := testutils.TestRequest(t, app, "DELETE", url, "")
		assert.Equal(t, 302, resp.StatusCode)

		// Verify product was deleted from database
		var deletedProduct models.Product
		err := db.First(&deletedProduct, product.ID).Error
		assert.Error(t, err) // Should not find the product
	})

	t.Run("Delete - Product with License Keys", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		app := testutils.SetupTestAppWithDB(t, db)
		handler := NewProductsHandler(db)

		app.Delete("/products/:id", handler.Delete)

		// Setup test data with related license keys
		product := models.Product{
			Name:        "Product with Keys",
			Description: "Has license keys",
			Version:     "1.0.0",
		}
		require.NoError(t, db.Create(&product).Error)

		customer := models.Customer{Name: "Test Customer", Email: "test@example.com"}
		require.NoError(t, db.Create(&customer).Error)

		licenseKey := models.LicenseKey{
			Key:        "TEST-KEY",
			ProductID:  product.ID,
			CustomerID: customer.ID,
		}
		require.NoError(t, db.Create(&licenseKey).Error)

		// Test deletion should fail due to foreign key constraint
		url := "/products/" + strconv.Itoa(int(product.ID))
		resp := testutils.TestRequest(t, app, "DELETE", url, "")
		// Expect either 400 (validation error) or 500 (database constraint error)
		assert.True(t, resp.StatusCode == 400 || resp.StatusCode == 500)

		// Verify product was NOT deleted
		var existingProduct models.Product
		err := db.First(&existingProduct, product.ID).Error
		assert.NoError(t, err) // Should still find the product
	})
}
