package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"license-key-manager/internal/database"
	"license-key-manager/internal/middleware"
	"license-key-manager/internal/models"
)

type AdminHandler struct {
	db *gorm.DB
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

// Authentication
func (h *AdminHandler) LoginPage(c *fiber.Ctx) error {
	return c.Render("admin/users/login", fiber.Map{
		"ShowNav": false,
		"Title":   "Login",
	})
}

func (h *AdminHandler) Login(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	var admin models.AdminUser
	if err := h.db.Where("username = ?", username).First(&admin).Error; err != nil {
		return c.Render("admin/users/login", fiber.Map{
			"Error":   "Invalid username or password",
			"ShowNav": false,
			"Title":   "Login",
		})
	}

	if !admin.CheckPassword(password) {
		return c.Render("admin/users/login", fiber.Map{
			"Error":   "Invalid username or password",
			"ShowNav": false,
			"Title":   "Login",
		})
	}

	if err := middleware.Login(c, admin.ID); err != nil {
		return c.Status(500).SendString("Login failed")
	}

	return c.Redirect("/admin/")
}

func (h *AdminHandler) Logout(c *fiber.Ctx) error {
	_ = middleware.Logout(c)
	return c.Redirect("/admin/login")
}

// Dashboard
func (h *AdminHandler) Dashboard(c *fiber.Ctx) error {
	log.Printf("Dashboard: Rendering dashboard template for path: %s", c.Path())
	// Strong cache-busting headers to prevent browser caching issues
	c.Set("Cache-Control", "no-cache, no-store, must-revalidate, private")
	c.Set("Pragma", "no-cache")
	c.Set("Expires", "0")
	c.Set("Last-Modified", time.Now().UTC().Format(time.RFC1123))
	c.Set("ETag", fmt.Sprintf("\"%d\"", time.Now().Unix()))

	// Add timestamp to URL parameters to ensure fresh request
	timestamp := time.Now().Unix()

	var stats struct {
		TotalProducts   int64
		TotalCustomers  int64
		TotalLicenses   int64
		ActiveLicenses  int64
		ExpiredLicenses int64
	}

	h.db.Model(&models.Product{}).Count(&stats.TotalProducts)
	h.db.Model(&models.Customer{}).Count(&stats.TotalCustomers)
	h.db.Model(&models.LicenseKey{}).Count(&stats.TotalLicenses)
	h.db.Model(&models.LicenseKey{}).Where("status = ?", "active").Count(&stats.ActiveLicenses)
	h.db.Model(&models.LicenseKey{}).Where("expires_at < ?", time.Now()).Count(&stats.ExpiredLicenses)

	var recentLicenses []models.LicenseKey
	h.db.Preload("Product").Preload("Customer").
		Order("created_at DESC").
		Limit(10).
		Find(&recentLicenses)

	return c.Render("admin/dashboard/index", fiber.Map{
		"ShowNav":            true,
		"PageType":           "dashboard",
		"Title":              "Dashboard - Live " + time.Now().Format("15:04:05"),
		"ProductCount":       stats.TotalProducts,
		"CustomerCount":      stats.TotalCustomers,
		"TotalLicenseCount":  stats.TotalLicenses,
		"ActiveLicenseCount": stats.ActiveLicenses,
		"RecentLicenses":     recentLicenses,
		"CacheBuster":        timestamp,
		"CurrentTime":        time.Now().Format("2006-01-02 15:04:05"),
	})
}

// Products
func (h *AdminHandler) ProductsIndex(c *fiber.Ctx) error {
	var products []models.Product
	h.db.Preload("LicenseKeys").Find(&products)

	return c.Render("admin/products/index", fiber.Map{
		"ShowNav":   true,
		"PageType":  "products-index",
		"Products":  products,
		"CSRFToken": "",
	})
}

func (h *AdminHandler) ProductsNew(c *fiber.Ctx) error {
	return c.Render("admin/products/new", fiber.Map{
		"ShowNav":   true,
		"PageType":  "products-new",
		"CSRFToken": "",
	})
}

func (h *AdminHandler) ProductsCreate(c *fiber.Ctx) error {
	log.Printf("ProductsCreate: Method=%s, Path=%s", c.Method(), c.Path())
	log.Printf("ProductsCreate: Form values - name=%s, description=%s, version=%s",
		c.FormValue("name"), c.FormValue("description"), c.FormValue("version"))

	product := models.Product{
		Name:        c.FormValue("name"),
		Description: c.FormValue("description"),
		Version:     c.FormValue("version"),
	}

	// Handle expiration days
	if days, err := strconv.Atoi(c.FormValue("default_expiration_days")); err == nil {
		product.DefaultExpirationDays = days
	} else {
		product.DefaultExpirationDays = 365
	}

	// Handle usage limit
	if limit, err := strconv.Atoi(c.FormValue("default_usage_limit")); err == nil {
		product.DefaultUsageLimit = limit
	} else {
		product.DefaultUsageLimit = 1
	}

	// Use PerformWrite for database operation with retry logic
	err := database.PerformWrite(h.db, func(db *gorm.DB) error {
		return db.Create(&product).Error
	})
	if err != nil {
		return c.Render("admin/products/new", fiber.Map{
			"Error":   "Failed to create product: " + err.Error(),
			"Product": product,
			"ShowNav": true,
		})
	}

	return c.Redirect("/admin/products")
}

func (h *AdminHandler) ProductsShow(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var product models.Product
	if err := h.db.Preload("LicenseKeys.Customer").First(&product, id).Error; err != nil {
		return c.Status(404).SendString("Product not found")
	}

	return c.Render("admin/products/show", fiber.Map{
		"ShowNav":  true,
		"PageType": "products-show",
		"Product":  product,
	})
}

func (h *AdminHandler) ProductsEdit(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var product models.Product
	if err := h.db.First(&product, id).Error; err != nil {
		return c.Status(404).SendString("Product not found")
	}

	return c.Render("admin/products/edit", fiber.Map{
		"ShowNav":   true,
		"PageType":  "products-edit",
		"Product":   product,
		"CSRFToken": "",
	})
}

func (h *AdminHandler) ProductsUpdate(c *fiber.Ctx) error {
	// Handle method override for HTML forms
	if c.Method() == "POST" && c.FormValue("_method") != "PUT" {
		return c.Status(405).SendString("Method not allowed")
	}

	id, _ := strconv.Atoi(c.Params("id"))
	var product models.Product
	if err := h.db.First(&product, id).Error; err != nil {
		return c.Status(404).SendString("Product not found")
	}

	product.Name = c.FormValue("name")
	product.Description = c.FormValue("description")
	product.Version = c.FormValue("version")

	if days, err := strconv.Atoi(c.FormValue("default_expiration_days")); err == nil {
		product.DefaultExpirationDays = days
	}

	if limit, err := strconv.Atoi(c.FormValue("default_usage_limit")); err == nil {
		product.DefaultUsageLimit = limit
	}

	err := database.PerformWrite(h.db, func(db *gorm.DB) error {
		return db.Save(&product).Error
	})
	if err != nil {
		return c.Render("admin/products/edit", fiber.Map{
			"Error":     "Failed to update product: " + err.Error(),
			"Product":   product,
			"CSRFToken": "",
		})
	}

	return c.Redirect("/admin/products/" + c.Params("id"))
}

func (h *AdminHandler) ProductsDelete(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	if err := h.db.Delete(&models.Product{}, id).Error; err != nil {
		return c.Status(500).SendString("Failed to delete product")
	}

	return c.Redirect("/admin/products")
}

// Customers
func (h *AdminHandler) CustomersIndex(c *fiber.Ctx) error {
	var customers []models.Customer
	h.db.Preload("LicenseKeys").Find(&customers)

	return c.Render("admin/customers/index", fiber.Map{
		"ShowNav":   true,
		"PageType":  "customers-index",
		"Customers": customers,
		"CSRFToken": "",
	})
}

func (h *AdminHandler) CustomersNew(c *fiber.Ctx) error {
	return c.Render("admin/customers/new", fiber.Map{
		"ShowNav":   true,
		"PageType":  "customers-new",
		"CSRFToken": "",
	})
}

func (h *AdminHandler) CustomersCreate(c *fiber.Ctx) error {
	customer := models.Customer{
		Email:     c.FormValue("email"),
		FirstName: c.FormValue("first_name"),
		LastName:  c.FormValue("last_name"),
		Company:   c.FormValue("company"),
	}

	// Set Name field as combination of first and last name
	if customer.FirstName != "" || customer.LastName != "" {
		customer.Name = strings.TrimSpace(customer.FirstName + " " + customer.LastName)
	} else if customer.Email != "" {
		// Extract name from email if no name provided (get part before @)
		atIndex := strings.Index(customer.Email, "@")
		if atIndex > 0 {
			customer.Name = customer.Email[:atIndex]
		} else {
			customer.Name = customer.Email
		}
	}

	// Use PerformWrite for database operation with retry logic
	err := database.PerformWrite(h.db, func(db *gorm.DB) error {
		return db.Create(&customer).Error
	})
	if err != nil {
		return c.Render("admin/customers/new", fiber.Map{
			"Error":    "Failed to create customer: " + err.Error(),
			"Customer": customer,
			"ShowNav":  true,
		})
	}

	return c.Redirect("/admin/customers")
}

func (h *AdminHandler) CustomersShow(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var customer models.Customer
	if err := h.db.Preload("LicenseKeys.Product").First(&customer, id).Error; err != nil {
		return c.Status(404).SendString("Customer not found")
	}

	return c.Render("admin/customers/show", fiber.Map{
		"ShowNav":  true,
		"PageType": "customers-show",
		"Customer": customer,
	})
}

func (h *AdminHandler) CustomersEdit(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var customer models.Customer
	if err := h.db.First(&customer, id).Error; err != nil {
		return c.Status(404).SendString("Customer not found")
	}

	return c.Render("admin/customers/edit", fiber.Map{
		"ShowNav":   true,
		"PageType":  "customers-edit",
		"Customer":  customer,
		"CSRFToken": "",
	})
}

func (h *AdminHandler) CustomersUpdate(c *fiber.Ctx) error {
	// Handle method override for HTML forms
	if c.Method() == "POST" && c.FormValue("_method") != "PUT" {
		return c.Status(405).SendString("Method not allowed")
	}

	id, _ := strconv.Atoi(c.Params("id"))
	var customer models.Customer
	if err := h.db.First(&customer, id).Error; err != nil {
		return c.Status(404).SendString("Customer not found")
	}

	customer.Email = c.FormValue("email")
	customer.FirstName = c.FormValue("first_name")
	customer.LastName = c.FormValue("last_name")
	customer.Company = c.FormValue("company")

	// Update Name field
	if customer.FirstName != "" || customer.LastName != "" {
		customer.Name = strings.TrimSpace(customer.FirstName + " " + customer.LastName)
	} else if customer.Email != "" {
		// Extract name from email if no name provided (get part before @)
		atIndex := strings.Index(customer.Email, "@")
		if atIndex > 0 {
			customer.Name = customer.Email[:atIndex]
		} else {
			customer.Name = customer.Email
		}
	}

	err := database.PerformWrite(h.db, func(db *gorm.DB) error {
		return db.Save(&customer).Error
	})
	if err != nil {
		return c.Render("admin/customers/edit", fiber.Map{
			"Error":     "Failed to update customer: " + err.Error(),
			"Customer":  customer,
			"ShowNav":   true,
			"CSRFToken": "",
		})
	}

	return c.Redirect("/admin/customers/" + c.Params("id"))
}

func (h *AdminHandler) CustomersDelete(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	err := database.PerformWrite(h.db, func(db *gorm.DB) error {
		return db.Delete(&models.Customer{}, id).Error
	})
	if err != nil {
		return c.Status(500).SendString("Failed to delete customer")
	}

	return c.Redirect("/admin/customers")
}

// License Keys
func (h *AdminHandler) LicenseKeysIndex(c *fiber.Ctx) error {
	var licenseKeys []models.LicenseKey
	h.db.Preload("Product").Preload("Customer").
		Order("created_at DESC").
		Find(&licenseKeys)

	return c.Render("admin/license-keys/index", fiber.Map{
		"ShowNav":     true,
		"PageType":    "license-keys-index",
		"LicenseKeys": licenseKeys,
		"CSRFToken":   "",
	})
}

func (h *AdminHandler) LicenseKeysNew(c *fiber.Ctx) error {
	var products []models.Product
	var customers []models.Customer
	h.db.Find(&products)
	h.db.Find(&customers)

	return c.Render("admin/license-keys/new", fiber.Map{
		"ShowNav":   true,
		"PageType":  "license-keys-new",
		"Products":  products,
		"Customers": customers,
		"CSRFToken": "",
	})
}

func (h *AdminHandler) LicenseKeysCreate(c *fiber.Ctx) error {
	productID, _ := strconv.Atoi(c.FormValue("product_id"))
	customerID, _ := strconv.Atoi(c.FormValue("customer_id"))

	var product models.Product
	var customer models.Customer

	if err := h.db.First(&product, productID).Error; err != nil {
		return c.Status(400).SendString("Invalid product")
	}

	if err := h.db.First(&customer, customerID).Error; err != nil {
		return c.Status(400).SendString("Invalid customer")
	}

	licenseKey, err := product.GenerateLicenseKeyFor(h.db, &customer)
	if err != nil {
		return c.Status(500).SendString("Failed to create license key")
	}

	return c.Redirect("/admin/license-keys/" + strconv.Itoa(int(licenseKey.ID)))
}

func (h *AdminHandler) LicenseKeysShow(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var licenseKey models.LicenseKey
	if err := h.db.Preload("Product").Preload("Customer").First(&licenseKey, id).Error; err != nil {
		return c.Status(404).SendString("License key not found")
	}

	return c.Render("admin/license-keys/show", fiber.Map{
		"ShowNav":    true,
		"PageType":   "license-keys-show",
		"LicenseKey": licenseKey,
	})
}

func (h *AdminHandler) LicenseKeysEdit(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var licenseKey models.LicenseKey
	if err := h.db.Preload("Product").Preload("Customer").First(&licenseKey, id).Error; err != nil {
		return c.Status(404).SendString("License key not found")
	}

	var products []models.Product
	var customers []models.Customer
	h.db.Find(&products)
	h.db.Find(&customers)

	return c.Render("admin/license-keys/edit", fiber.Map{
		"ShowNav":    true,
		"PageType":   "license-keys-edit",
		"LicenseKey": licenseKey,
		"Products":   products,
		"Customers":  customers,
		"CSRFToken":  "",
	})
}

func (h *AdminHandler) LicenseKeysUpdate(c *fiber.Ctx) error {
	// Handle method override for HTML forms
	if c.Method() == "POST" && c.FormValue("_method") != "PUT" {
		return c.Status(405).SendString("Method not allowed")
	}

	id, _ := strconv.Atoi(c.Params("id"))
	var licenseKey models.LicenseKey
	if err := h.db.First(&licenseKey, id).Error; err != nil {
		return c.Status(404).SendString("License key not found")
	}

	if expiresAt, err := time.Parse("2006-01-02", c.FormValue("expires_at")); err == nil {
		licenseKey.ExpiresAt = &expiresAt
	}

	if usageLimit, err := strconv.Atoi(c.FormValue("usage_limit")); err == nil {
		licenseKey.UsageLimit = usageLimit
	}

	licenseKey.Metadata = c.FormValue("metadata")

	err := database.PerformWrite(h.db, func(db *gorm.DB) error {
		return db.Save(&licenseKey).Error
	})
	if err != nil {
		var products []models.Product
		var customers []models.Customer
		h.db.Find(&products)
		h.db.Find(&customers)

		return c.Render("admin/license-keys/edit", fiber.Map{
			"Error":      "Failed to update license key: " + err.Error(),
			"LicenseKey": licenseKey,
			"Products":   products,
			"Customers":  customers,
			"CSRFToken":  c.Locals("csrf"),
		})
	}

	return c.Redirect("/admin/license-keys/" + c.Params("id"))
}

func (h *AdminHandler) LicenseKeysDelete(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	if err := h.db.Delete(&models.LicenseKey{}, id).Error; err != nil {
		return c.Status(500).SendString("Failed to delete license key")
	}

	return c.Redirect("/admin/license-keys")
}

func (h *AdminHandler) LicenseKeysRevoke(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var licenseKey models.LicenseKey
	if err := h.db.First(&licenseKey, id).Error; err != nil {
		return c.Status(404).SendString("License key not found")
	}

	if err := licenseKey.Revoke(h.db); err != nil {
		return c.Status(500).SendString("Failed to revoke license key")
	}

	return c.Redirect("/admin/license-keys/" + c.Params("id"))
}

func (h *AdminHandler) LicenseKeysReactivate(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var licenseKey models.LicenseKey
	if err := h.db.First(&licenseKey, id).Error; err != nil {
		return c.Status(404).SendString("License key not found")
	}

	if err := licenseKey.Reactivate(h.db); err != nil {
		return c.Status(500).SendString("Failed to reactivate license key")
	}

	return c.Redirect("/admin/license-keys/" + c.Params("id"))
}

func (h *AdminHandler) LicenseKeysSendEmail(c *fiber.Ctx) error {
	// This would require the email service to be injected
	// For now, just redirect back
	return c.Redirect("/admin/license-keys/" + c.Params("id"))
}
