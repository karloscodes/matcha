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

	"matcha/internal/models"
	"matcha/internal/testutils"
)

func TestProductsHandler_Index(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB)
		expectedStatus int
	}{
		{
			name: "should render products index with empty list",
			setupData: func(db *gorm.DB) {
				// No products
			},
			expectedStatus: 200,
		},
		{
			name: "should render products index with products",
			setupData: func(db *gorm.DB) {
				product := models.Product{
					Name:        "Test Product",
					Description: "Test Description",
					Version:     "1.0.0",
				}
				db.Create(&product)
			},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewProductsHandler(db)

			tt.setupData(db)

			app.Get("/test", testutils.MockRender(handler.Index))

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestProductsHandler_New(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	handler := NewProductsHandler(db)

	app.Get("/test", testutils.MockRender(handler.New))

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 200, resp.StatusCode)
}

func TestProductsHandler_Create(t *testing.T) {
	tests := []struct {
		name           string
		formData       map[string]string
		expectedStatus int
		expectedResult string
	}{
		{
			name: "should create product successfully",
			formData: map[string]string{
				"name":                    "Test Product",
				"description":             "Test Description",
				"version":                 "1.0.0",
				"default_expiration_days": "365",
				"default_usage_limit":     "1",
			},
			expectedStatus: 302,
			expectedResult: "/admin/products",
		},
		{
			name: "should create product with default values",
			formData: map[string]string{
				"name":        "Test Product",
				"description": "Test Description",
				"version":     "1.0.0",
			},
			expectedStatus: 302,
			expectedResult: "/admin/products",
		},
		{
			name: "should handle invalid expiration days",
			formData: map[string]string{
				"name":                    "Test Product",
				"description":             "Test Description",
				"version":                 "1.0.0",
				"default_expiration_days": "invalid",
				"default_usage_limit":     "1",
			},
			expectedStatus: 302,
			expectedResult: "/admin/products",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewProductsHandler(db)

			form := url.Values{}
			for key, value := range tt.formData {
				form.Set(key, value)
			}

			app.Post("/test", func(c *fiber.Ctx) error {
				return handler.Create(c)
			})

			req := httptest.NewRequest("POST", "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.expectedResult != "" {
				location := resp.Header.Get("Location")
				assert.Equal(t, tt.expectedResult, location)
			}

			// Verify product was created
			if tt.expectedStatus == 302 {
				var count int64
				db.Model(&models.Product{}).Count(&count)
				assert.Equal(t, int64(1), count)
			}
		})
	}
}

func TestProductsHandler_Show(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		productID      string
		expectedStatus int
	}{
		{
			name: "should show existing product",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{
					Name:        "Test Product",
					Description: "Test Description",
					Version:     "1.0.0",
				}
				db.Create(&product)
				return product.ID
			},
			expectedStatus: 200,
		},
		{
			name: "should return 404 for non-existent product",
			setupData: func(db *gorm.DB) uint {
				return 999
			},
			expectedStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewProductsHandler(db)

			productID := tt.setupData(db)

			app.Get("/test/:id", testutils.MockRender(handler.Show))

			req := httptest.NewRequest("GET", "/test/"+strconv.Itoa(int(productID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestProductsHandler_Edit(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should show edit form for existing product",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{
					Name:        "Test Product",
					Description: "Test Description",
					Version:     "1.0.0",
				}
				db.Create(&product)
				return product.ID
			},
			expectedStatus: 200,
		},
		{
			name: "should return 404 for non-existent product",
			setupData: func(db *gorm.DB) uint {
				return 999
			},
			expectedStatus: 404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewProductsHandler(db)

			productID := tt.setupData(db)

			app.Get("/test/:id", testutils.MockRender(handler.Edit))

			req := httptest.NewRequest("GET", "/test/"+strconv.Itoa(int(productID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestProductsHandler_Update(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		formData       map[string]string
		expectedStatus int
		expectedResult string
	}{
		{
			name: "should update product successfully",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{
					Name:        "Original Product",
					Description: "Original Description",
					Version:     "1.0.0",
				}
				db.Create(&product)
				return product.ID
			},
			formData: map[string]string{
				"_method":                 "PUT",
				"name":                    "Updated Product",
				"description":             "Updated Description",
				"version":                 "2.0.0",
				"default_expiration_days": "730",
				"default_usage_limit":     "5",
			},
			expectedStatus: 302,
		},
		{
			name: "should return 404 for non-existent product",
			setupData: func(db *gorm.DB) uint {
				return 999
			},
			formData: map[string]string{
				"_method":     "PUT",
				"name":        "Updated Product",
				"description": "Updated Description",
				"version":     "2.0.0",
			},
			expectedStatus: 404,
		},
		{
			name: "should return 405 for invalid method",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{
					Name:        "Test Product",
					Description: "Test Description",
					Version:     "1.0.0",
				}
				db.Create(&product)
				return product.ID
			},
			formData: map[string]string{
				"_method":     "INVALID",
				"name":        "Updated Product",
				"description": "Updated Description",
				"version":     "2.0.0",
			},
			expectedStatus: 405,
		},
		{
			name: "should return 405 for POST without _method=PUT",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{
					Name:        "Test Product",
					Description: "Test Description",
					Version:     "1.0.0",
				}
				db.Create(&product)
				return product.ID
			},
			formData: map[string]string{
				"name":        "Updated Product",
				"description": "Updated Description",
				"version":     "2.0.0",
			},
			expectedStatus: 405,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewProductsHandler(db)

			productID := tt.setupData(db)

			form := url.Values{}
			for key, value := range tt.formData {
				form.Set(key, value)
			}

			app.Post("/test/:id", func(c *fiber.Ctx) error {
				return handler.Update(c)
			})

			req := httptest.NewRequest("POST", "/test/"+strconv.Itoa(int(productID)), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify product was updated if successful
			if tt.expectedStatus == 302 {
				var product models.Product
				db.First(&product, productID)
				assert.Equal(t, "Updated Product", product.Name)
				assert.Equal(t, "Updated Description", product.Description)
				assert.Equal(t, "2.0.0", product.Version)
			}
		})
	}
}

func TestProductsHandler_Delete(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should delete existing product",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{
					Name:        "Test Product",
					Description: "Test Description",
					Version:     "1.0.0",
				}
				db.Create(&product)
				return product.ID
			},
			expectedStatus: 302,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewProductsHandler(db)

			productID := tt.setupData(db)

			app.Delete("/test/:id", func(c *fiber.Ctx) error {
				return handler.Delete(c)
			})

			req := httptest.NewRequest("DELETE", "/test/"+strconv.Itoa(int(productID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify product was deleted
			if tt.expectedStatus == 302 {
				var count int64
				db.Model(&models.Product{}).Where("id = ?", productID).Count(&count)
				assert.Equal(t, int64(0), count)
			}
		})
	}
}

func TestNewProductsHandler(t *testing.T) {
	db := testutils.SetupTestDB(t)
	handler := NewProductsHandler(db)

	assert.NotNil(t, handler)
	assert.Equal(t, db, handler.db)
}
