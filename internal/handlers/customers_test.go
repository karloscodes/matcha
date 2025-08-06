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

func TestCustomersHandler_Index(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB)
		expectedStatus int
	}{
		{
			name: "should render customers index with empty list",
			setupData: func(db *gorm.DB) {
				// No customers
			},
			expectedStatus: 200,
		},
		{
			name: "should render customers index with customers",
			setupData: func(db *gorm.DB) {
				customer := models.Customer{
					Name:      "John Doe",
					Email:     "john@example.com",
					FirstName: "John",
					LastName:  "Doe",
				}
				db.Create(&customer)
			},
			expectedStatus: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewCustomersHandler(db)

			tt.setupData(db)

			app.Get("/test", testutils.MockRender(handler.Index))

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestCustomersHandler_New(t *testing.T) {
	db := testutils.SetupTestDB(t)
	app := testutils.SetupTestApp()
	handler := NewCustomersHandler(db)

	app.Get("/test", testutils.MockRender(handler.New))

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, 200, resp.StatusCode)
}

func TestCustomersHandler_Create(t *testing.T) {
	tests := []struct {
		name           string
		formData       map[string]string
		expectedStatus int
		expectedResult string
		expectedName   string
	}{
		{
			name: "should create customer with full name",
			formData: map[string]string{
				"email":      "john@example.com",
				"first_name": "John",
				"last_name":  "Doe",
				"company":    "Test Corp",
			},
			expectedStatus: 302,
			expectedResult: "/admin/customers",
			expectedName:   "John Doe",
		},
		{
			name: "should create customer with email-derived name",
			formData: map[string]string{
				"email": "jane@example.com",
			},
			expectedStatus: 302,
			expectedResult: "/admin/customers",
			expectedName:   "jane",
		},
		{
			name: "should create customer with first name only",
			formData: map[string]string{
				"email":      "bob@example.com",
				"first_name": "Bob",
			},
			expectedStatus: 302,
			expectedResult: "/admin/customers",
			expectedName:   "Bob",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewCustomersHandler(db)

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

			// Verify customer was created with correct name
			if tt.expectedStatus == 302 {
				var customer models.Customer
				db.First(&customer)
				assert.Equal(t, tt.expectedName, customer.Name)
				assert.Equal(t, tt.formData["email"], customer.Email)
			}
		})
	}
}

func TestCustomersHandler_Show(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should show existing customer",
			setupData: func(db *gorm.DB) uint {
				customer := models.Customer{
					Name:      "John Doe",
					Email:     "john@example.com",
					FirstName: "John",
					LastName:  "Doe",
				}
				db.Create(&customer)
				return customer.ID
			},
			expectedStatus: 200,
		},
		{
			name: "should return 404 for non-existent customer",
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
			handler := NewCustomersHandler(db)

			customerID := tt.setupData(db)

			app.Get("/test/:id", testutils.MockRender(handler.Show))

			req := httptest.NewRequest("GET", "/test/"+strconv.Itoa(int(customerID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestCustomersHandler_Edit(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should show edit form for existing customer",
			setupData: func(db *gorm.DB) uint {
				customer := models.Customer{
					Name:      "John Doe",
					Email:     "john@example.com",
					FirstName: "John",
					LastName:  "Doe",
				}
				db.Create(&customer)
				return customer.ID
			},
			expectedStatus: 200,
		},
		{
			name: "should return 404 for non-existent customer",
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
			handler := NewCustomersHandler(db)

			customerID := tt.setupData(db)

			app.Get("/test/:id", testutils.MockRender(handler.Edit))

			req := httptest.NewRequest("GET", "/test/"+strconv.Itoa(int(customerID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)
		})
	}
}

func TestCustomersHandler_Update(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		formData       map[string]string
		expectedStatus int
		expectedName   string
	}{
		{
			name: "should update customer successfully",
			setupData: func(db *gorm.DB) uint {
				customer := models.Customer{
					Name:      "Original Name",
					Email:     "original@example.com",
					FirstName: "Original",
					LastName:  "Name",
				}
				db.Create(&customer)
				return customer.ID
			},
			formData: map[string]string{
				"_method":    "PUT",
				"email":      "updated@example.com",
				"first_name": "Updated",
				"last_name":  "Name",
				"company":    "Updated Corp",
			},
			expectedStatus: 302,
			expectedName:   "Updated Name",
		},
		{
			name: "should return 404 for non-existent customer",
			setupData: func(db *gorm.DB) uint {
				return 999
			},
			formData: map[string]string{
				"_method":    "PUT",
				"email":      "updated@example.com",
				"first_name": "Updated",
				"last_name":  "Name",
			},
			expectedStatus: 404,
		},
		{
			name: "should return 405 for invalid method",
			setupData: func(db *gorm.DB) uint {
				customer := models.Customer{
					Name:  "Test Customer",
					Email: "test@example.com",
				}
				db.Create(&customer)
				return customer.ID
			},
			formData: map[string]string{
				"_method": "INVALID",
				"email":   "updated@example.com",
			},
			expectedStatus: 405,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewCustomersHandler(db)

			customerID := tt.setupData(db)

			form := url.Values{}
			for key, value := range tt.formData {
				form.Set(key, value)
			}

			app.Post("/test/:id", func(c *fiber.Ctx) error {
				return handler.Update(c)
			})

			req := httptest.NewRequest("POST", "/test/"+strconv.Itoa(int(customerID)), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify customer was updated if successful
			if tt.expectedStatus == 302 && tt.expectedName != "" {
				var customer models.Customer
				db.First(&customer, customerID)
				assert.Equal(t, tt.expectedName, customer.Name)
				assert.Equal(t, "updated@example.com", customer.Email)
			}
		})
	}
}

func TestCustomersHandler_Delete(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB) uint
		expectedStatus int
	}{
		{
			name: "should delete existing customer",
			setupData: func(db *gorm.DB) uint {
				customer := models.Customer{
					Name:  "Test Customer",
					Email: "test@example.com",
				}
				db.Create(&customer)
				return customer.ID
			},
			expectedStatus: 302,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewCustomersHandler(db)

			customerID := tt.setupData(db)

			app.Delete("/test/:id", func(c *fiber.Ctx) error {
				return handler.Delete(c)
			})

			req := httptest.NewRequest("DELETE", "/test/"+strconv.Itoa(int(customerID)), nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify customer was deleted
			if tt.expectedStatus == 302 {
				var count int64
				db.Model(&models.Customer{}).Where("id = ?", customerID).Count(&count)
				assert.Equal(t, int64(0), count)
			}
		})
	}
}

func TestNewCustomersHandler(t *testing.T) {
	db := testutils.SetupTestDB(t)
	handler := NewCustomersHandler(db)

	assert.NotNil(t, handler)
	assert.Equal(t, db, handler.db)
}
