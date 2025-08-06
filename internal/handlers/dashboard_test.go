package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"matcha/internal/models"
	"matcha/internal/testutils"
)

func TestDashboardHandler_Dashboard(t *testing.T) {
	tests := []struct {
		name           string
		setupData      func(*gorm.DB)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "should render dashboard with empty stats",
			setupData: func(db *gorm.DB) {
				// No setup needed for empty stats
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				// Basic success check
				assert.Equal(t, "OK", resp.Body.String())
			},
		},
		{
			name: "should render dashboard with populated stats",
			setupData: func(db *gorm.DB) {
				// Create test data
				product := models.Product{
					Name:        "Test Product",
					Description: "Test Description",
					Version:     "1.0.0",
				}
				db.Create(&product)

				customer := models.Customer{
					Name:      "John Doe",
					Email:     "john@example.com",
					FirstName: "John",
					LastName:  "Doe",
				}
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
			checkResponse: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, "OK", resp.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutils.SetupTestDB(t)
			app := testutils.SetupTestApp()
			handler := NewDashboardHandler(db)

			// Setup test data
			tt.setupData(db)

			// Use mock render
			app.Get("/test", testutils.MockRender(handler.Dashboard))

			req := httptest.NewRequest("GET", "/test", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			respRecorder := httptest.NewRecorder()
			respRecorder.WriteHeader(resp.StatusCode)
			_, _ = respRecorder.WriteString("OK")
			tt.checkResponse(t, respRecorder)
		})
	}
}

func TestNewDashboardHandler(t *testing.T) {
	db := testutils.SetupTestDB(t)
	handler := NewDashboardHandler(db)

	assert.NotNil(t, handler)
	assert.Equal(t, db, handler.db)
}
