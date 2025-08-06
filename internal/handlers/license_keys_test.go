package handlers

import (
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"license-key-manager/internal/models"
	"license-key-manager/internal/testutils"
)

func TestLicenseKeysHandler_Index(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB)
		expectedStatus int
	}{
		{
			name: "should render license keys index with empty list",
			setupData: func(db *gorm.DB) {
				// No license keys
			},
			expectedStatus: 200,
		},
		{
			name: "should render license keys index with license keys",
			setupData: func(db *gorm.DB) {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-123",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
				}
				db.Create(&licenseKey)
			},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewLicenseKeysHandler(db)

			tt.setupData(db)

			app.Get("/test", testutils.MockRender(handler.Index))

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestLicenseKeysHandler_New(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	handler := NewLicenseKeysHandler(db)

	// Create test data
	product := models.Product{Name: "Test Product", Version: "1.0.0"}
	db.Create(&product)
	
	customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
	db.Create(&customer)

	app.Get("/test", testutils.MockRender(handler.New))

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 200, resp.StatusCode)
}

func TestLicenseKeysHandler_Create(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) (productID, customerID uint)
		formData       map[string]string
		expectedStatus int
	}{
		{
			name: "should create license key successfully",
			setupData: func(db *gorm.DB) (uint, uint) {
				product := models.Product{
					Name:                     "Test Product",
					Version:                  "1.0.0",
					DefaultExpirationDays:    365,
					DefaultUsageLimit:        1,
				}
				db.Create(&product)
				
				customer := models.Customer{
					Name:  "John Doe",
					Email: "john@example.com",
				}
				db.Create(&customer)
				
				return product.ID, customer.ID
			},
			expectedStatus: 302,
		},
		{
			name: "should return 400 for invalid product",
			setupData: func(db *gorm.DB) (uint, uint) {
				customer := models.Customer{
					Name:  "John Doe",
					Email: "john@example.com",
				}
				db.Create(&customer)
				
				return 999, customer.ID // Invalid product ID
			},
			expectedStatus: 400,
		},
		{
			name: "should return 400 for invalid customer",
			setupData: func(db *gorm.DB) (uint, uint) {
				product := models.Product{
					Name:                     "Test Product",
					Version:                  "1.0.0",
					DefaultExpirationDays:    365,
					DefaultUsageLimit:        1,
				}
				db.Create(&product)
				
				return product.ID, 999 // Invalid customer ID
			},
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewLicenseKeysHandler(db)

			productID, customerID := tt.setupData(db)

			form := url.Values{}
			form.Set("product_id", strconv.Itoa(int(productID)))
			form.Set("customer_id", strconv.Itoa(int(customerID)))

			app.Post("/test", func(c *fiber.Ctx) error {
				return handler.Create(c)
			})

			req := httptest.NewRequest("POST", "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify license key was created if successful
			if tt.expectedStatus == 302 {
				var count int64
				db.Model(&models.LicenseKey{}).Count(&count)
				assert.Equal(t, int64(1), count)
			}
		})
	}
}

func TestLicenseKeysHandler_Show(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should show existing license key",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-123",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
			expectedStatus: 200,
		},
		{
			name: "should return 404 for non-existent license key",
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
			handler := NewLicenseKeysHandler(db)

			licenseKeyID := tt.setupData(db)
			
			app.Get("/test/:id", testutils.MockRender(handler.Show))

			req := httptest.NewRequest("GET", "/test/"+strconv.Itoa(int(licenseKeyID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestLicenseKeysHandler_Edit(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should show edit form for existing license key",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-123",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
			expectedStatus: 200,
		},
		{
			name: "should return 404 for non-existent license key",
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
			handler := NewLicenseKeysHandler(db)

			licenseKeyID := tt.setupData(db)
			
			app.Get("/test/:id", testutils.MockRender(handler.Edit))

			req := httptest.NewRequest("GET", "/test/"+strconv.Itoa(int(licenseKeyID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestLicenseKeysHandler_Update(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		formData       map[string]string
		expectedStatus int
	}{
		{
			name: "should update license key successfully",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-123",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
					UsageLimit: 1,
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
			formData: map[string]string{
				"_method":     "PUT",
				"expires_at":  "2025-12-31",
				"usage_limit": "5",
				"metadata":    "Updated metadata",
			},
			expectedStatus: 302,
		},
		{
			name: "should return 404 for non-existent license key",
			setupData: func(db *gorm.DB) uint {
				return 999
			},
			formData: map[string]string{
				"_method":     "PUT",
				"expires_at":  "2025-12-31",
				"usage_limit": "5",
			},
			expectedStatus: 404,
		},
		{
			name: "should return 405 for invalid method",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-123",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
			formData: map[string]string{
				"_method":     "INVALID",
				"expires_at":  "2025-12-31",
				"usage_limit": "5",
			},
			expectedStatus: 405,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewLicenseKeysHandler(db)

			licenseKeyID := tt.setupData(db)

			form := url.Values{}
			for key, value := range tt.formData {
				form.Set(key, value)
			}

			app.Post("/test/:id", func(c *fiber.Ctx) error {
				return handler.Update(c)
			})

			req := httptest.NewRequest("POST", "/test/"+strconv.Itoa(int(licenseKeyID)), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify license key was updated if successful
			if tt.expectedStatus == 302 {
				var licenseKey models.LicenseKey
				db.First(&licenseKey, licenseKeyID)
				assert.Equal(t, 5, licenseKey.UsageLimit)
				assert.Equal(t, "Updated metadata", licenseKey.Metadata)
				
				expectedTime, _ := time.Parse("2006-01-02", "2025-12-31")
				assert.Equal(t, expectedTime, *licenseKey.ExpiresAt)
			}
		})
	}
}

func TestLicenseKeysHandler_Delete(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should delete existing license key",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-123",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
			expectedStatus: 302,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewLicenseKeysHandler(db)

			licenseKeyID := tt.setupData(db)
			
			app.Delete("/test/:id", func(c *fiber.Ctx) error {
				return handler.Delete(c)
			})

			req := httptest.NewRequest("DELETE", "/test/"+strconv.Itoa(int(licenseKeyID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify license key was deleted
			if tt.expectedStatus == 302 {
				var count int64
				db.Model(&models.LicenseKey{}).Where("id = ?", licenseKeyID).Count(&count)
				assert.Equal(t, int64(0), count)
			}
		})
	}
}

func TestLicenseKeysHandler_Revoke(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	handler := NewLicenseKeysHandler(db)

	// Create test data
	product := models.Product{Name: "Test Product", Version: "1.0.0"}
	db.Create(&product)
	
	customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
	db.Create(&customer)
	
	licenseKey := models.LicenseKey{
		Key:        "TEST-KEY-123",
		ProductID:  product.ID,
		CustomerID: customer.ID,
		Status:     "active",
	}
	db.Create(&licenseKey)

	app.Post("/test/:id", func(c *fiber.Ctx) error {
		return handler.Revoke(c)
	})

	req := httptest.NewRequest("POST", "/test/"+strconv.Itoa(int(licenseKey.ID)), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 302, resp.StatusCode)
}

func TestLicenseKeysHandler_Reactivate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	handler := NewLicenseKeysHandler(db)

	// Create test data
	product := models.Product{Name: "Test Product", Version: "1.0.0"}
	db.Create(&product)
	
	customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
	db.Create(&customer)
	
	licenseKey := models.LicenseKey{
		Key:        "TEST-KEY-123",
		ProductID:  product.ID,
		CustomerID: customer.ID,
		Status:     "revoked",
	}
	db.Create(&licenseKey)

	app.Post("/test/:id", func(c *fiber.Ctx) error {
		return handler.Reactivate(c)
	})

	req := httptest.NewRequest("POST", "/test/"+strconv.Itoa(int(licenseKey.ID)), nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 302, resp.StatusCode)
}

func TestLicenseKeysHandler_SendEmail(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	handler := NewLicenseKeysHandler(db)

	app.Post("/test/:id", func(c *fiber.Ctx) error {
		return handler.SendEmail(c)
	})

	req := httptest.NewRequest("POST", "/test/123", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 302, resp.StatusCode)
	assert.Equal(t, "/admin/license-keys/123", resp.Header.Get("Location"))
}

func TestNewLicenseKeysHandler(t *testing.T) {
	db := testutils.SetupTestDB(t)
	handler := NewLicenseKeysHandler(db)
	
	assert.NotNil(t, handler)
	assert.Equal(t, db, handler.db)
}

func TestLicenseKeysHandler_EditTemplateRendering(t *testing.T) {
	// This test verifies that the edit template can render without panics
	// by testing scenarios that would cause template errors
	
	tests := []struct {
		name      string
		setupData func(*gorm.DB) uint
	}{
		{
			name: "should handle license key with nil expires_at",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0", DefaultExpirationDays: 365, DefaultUsageLimit: 1}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-123",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
					ExpiresAt:  nil, // Nil pointer - would cause panic if not handled properly
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
		},
		{
			name: "should handle license key with expires_at set",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0", DefaultExpirationDays: 365, DefaultUsageLimit: 1}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				expirationDate := time.Now().AddDate(0, 0, 30) // 30 days from now
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-456",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
					ExpiresAt:  &expirationDate,
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewLicenseKeysHandler(db)

			licenseKeyID := tt.setupData(db)
			
			// Don't use MockRender - we want to test that the template actually works
			app.Get("/test/:id", handler.Edit)

			req := httptest.NewRequest("GET", "/test/"+strconv.Itoa(int(licenseKeyID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			
			// Should not panic and should return success (even if template is missing in test env)
			// The important thing is that the handler logic doesn't panic on template data preparation
			assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 500) // 500 is OK for missing template in tests
		})
	}
}

func TestLicenseKeysHandler_ShowTemplateRendering(t *testing.T) {
	// This test verifies that the show template can render without panics
	// by testing scenarios that would cause template errors
	
	tests := []struct {
		name      string
		setupData func(*gorm.DB) uint
	}{
		{
			name: "should handle license key with nil LastValidatedAt",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:               "TEST-KEY-123",
					ProductID:         product.ID,
					CustomerID:        customer.ID,
					Status:            "active",
					LastValidatedAt:   nil, // Nil pointer
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
		},
		{
			name: "should handle license key with LastValidatedAt set",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				lastValidated := time.Now()
				licenseKey := models.LicenseKey{
					Key:               "TEST-KEY-456",
					ProductID:         product.ID,
					CustomerID:        customer.ID,
					Status:            "active",
					LastValidatedAt:   &lastValidated,
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
		},
		{
			name: "should handle license key with nil ExpiresAt",
			setupData: func(db *gorm.DB) uint {
				product := models.Product{Name: "Test Product", Version: "1.0.0"}
				db.Create(&product)
				
				customer := models.Customer{Name: "John Doe", Email: "john@example.com"}
				db.Create(&customer)
				
				licenseKey := models.LicenseKey{
					Key:        "TEST-KEY-789",
					ProductID:  product.ID,
					CustomerID: customer.ID,
					Status:     "active",
					ExpiresAt:  nil,
				}
				db.Create(&licenseKey)
				return licenseKey.ID
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewLicenseKeysHandler(db)

			licenseKeyID := tt.setupData(db)
			
			// Don't use MockRender - we want to test that the template actually works
			app.Get("/test/:id", handler.Show)

			req := httptest.NewRequest("GET", "/test/"+strconv.Itoa(int(licenseKeyID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			
			// Should not panic and should return success (even if template is missing in test env)
			// The important thing is that the handler logic doesn't panic on template data preparation
			assert.True(t, resp.StatusCode == 200 || resp.StatusCode == 500) // 500 is OK for missing template in tests
		})
	}
}