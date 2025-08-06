package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"matcha/internal/database"
	"matcha/internal/models"
)

type LicenseKeysHandler struct {
	db *gorm.DB
}

func NewLicenseKeysHandler(db *gorm.DB) *LicenseKeysHandler {
	return &LicenseKeysHandler{db: db}
}

func (h *LicenseKeysHandler) Index(c *fiber.Ctx) error {
	var licenseKeys []models.LicenseKey
	h.db.Preload("Product").Preload("Customer").
		Order("created_at DESC").
		Find(&licenseKeys)

	// Try to render template, fallback to JSON if no template engine
	if err := c.Render("admin/license-keys/index", fiber.Map{
		"ShowNav":     true,
		"PageType":    "license-keys-index",
		"LicenseKeys": licenseKeys,
		"CSRFToken":   "",
	}); err != nil {
		return c.Status(200).JSON(fiber.Map{
			"licenseKeys": licenseKeys,
		})
	}
	return nil
}

func (h *LicenseKeysHandler) New(c *fiber.Ctx) error {
	var products []models.Product
	var customers []models.Customer
	h.db.Find(&products)
	h.db.Find(&customers)

	// Try to render template, fallback to JSON if no template engine
	if err := c.Render("admin/license-keys/new", fiber.Map{
		"ShowNav":   true,
		"PageType":  "license-keys-new",
		"Products":  products,
		"Customers": customers,
		"CSRFToken": "",
	}); err != nil {
		return c.Status(200).JSON(fiber.Map{
			"products":  products,
			"customers": customers,
		})
	}
	return nil
}

func (h *LicenseKeysHandler) Create(c *fiber.Ctx) error {
	productID, _ := strconv.Atoi(c.FormValue("product_id"))
	customerID, _ := strconv.Atoi(c.FormValue("customer_id"))
	key := c.FormValue("key")
	maxActivations, _ := strconv.Atoi(c.FormValue("max_activations"))

	var product models.Product
	var customer models.Customer

	if err := h.db.First(&product, productID).Error; err != nil {
		return c.Status(400).SendString("Invalid product")
	}

	if err := h.db.First(&customer, customerID).Error; err != nil {
		return c.Status(400).SendString("Invalid customer")
	}

	// Create license key with provided details or generate defaults
	licenseKey := &models.LicenseKey{
		ProductID:          product.ID,
		CustomerID:         customer.ID,
		Key:                key,
		MaxActivations:     maxActivations,
		CurrentActivations: 0,
		Status:             "active",
		IsTrial:            false,
	}

	// If no key provided, generate one
	if licenseKey.Key == "" {
		generatedKey, err := product.GenerateLicenseKeyFor(h.db, &customer)
		if err != nil {
			return c.Status(500).SendString("Failed to create license key")
		}
		return c.Redirect("/admin/license-keys/" + strconv.Itoa(int(generatedKey.ID)))
	}

	// If max activations not provided, use product default
	if licenseKey.MaxActivations == 0 {
		licenseKey.MaxActivations = product.DefaultUsageLimit
	}

	// Set expiration if product has default
	if product.DefaultExpirationDays > 0 {
		expiresAt := time.Now().AddDate(0, 0, product.DefaultExpirationDays)
		licenseKey.ExpiresAt = &expiresAt
	}

	if err := h.db.Create(licenseKey).Error; err != nil {
		return c.Status(500).SendString("Failed to create license key")
	}

	return c.Redirect("/admin/license-keys/" + strconv.Itoa(int(licenseKey.ID)))
}

func (h *LicenseKeysHandler) Show(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var licenseKey models.LicenseKey
	if err := h.db.Preload("Product").Preload("Customer").First(&licenseKey, id).Error; err != nil {
		return c.Status(404).SendString("License key not found")
	}

	// Try to render template, fallback to JSON if no template engine
	if err := c.Render("admin/license-keys/show", fiber.Map{
		"ShowNav":    true,
		"PageType":   "license-keys-show",
		"LicenseKey": licenseKey,
	}); err != nil {
		return c.Status(200).JSON(fiber.Map{
			"licenseKey": licenseKey,
		})
	}
	return nil
}

func (h *LicenseKeysHandler) Edit(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var licenseKey models.LicenseKey
	if err := h.db.Preload("Product").Preload("Customer").First(&licenseKey, id).Error; err != nil {
		return c.Status(404).SendString("License key not found")
	}

	var products []models.Product
	var customers []models.Customer
	h.db.Find(&products)
	h.db.Find(&customers)

	// Try to render template, fallback to JSON if no template engine
	if err := c.Render("admin/license-keys/edit", fiber.Map{
		"ShowNav":    true,
		"PageType":   "license-keys-edit",
		"LicenseKey": licenseKey,
		"Products":   products,
		"Customers":  customers,
		"CSRFToken":  "",
	}); err != nil {
		return c.Status(200).JSON(fiber.Map{
			"licenseKey": licenseKey,
			"products":   products,
			"customers":  customers,
		})
	}
	return nil
}

func (h *LicenseKeysHandler) Update(c *fiber.Ctx) error {
	// Accept both PUT requests and POST requests with _method=PUT
	if c.Method() != "PUT" && !(c.Method() == "POST" && c.FormValue("_method") == "PUT") {
		return c.Status(405).SendString("Method not allowed")
	}

	id, _ := strconv.Atoi(c.Params("id"))
	var licenseKey models.LicenseKey
	if err := h.db.First(&licenseKey, id).Error; err != nil {
		return c.Status(404).SendString("License key not found")
	}

	// Update product ID
	if productID, err := strconv.Atoi(c.FormValue("product_id")); err == nil && productID > 0 {
		licenseKey.ProductID = uint(productID)
	}

	// Update customer ID
	if customerID, err := strconv.Atoi(c.FormValue("customer_id")); err == nil && customerID > 0 {
		licenseKey.CustomerID = uint(customerID)
	}

	// Update expiration date - handle both date and datetime-local formats
	if expiresAtStr := c.FormValue("expires_at"); expiresAtStr != "" {
		// Try datetime-local format first (YYYY-MM-DDTHH:MM)
		if expiresAt, err := time.Parse("2006-01-02T15:04", expiresAtStr); err == nil {
			licenseKey.ExpiresAt = &expiresAt
		} else if expiresAt, err := time.Parse("2006-01-02", expiresAtStr); err == nil {
			// Fallback to date format (YYYY-MM-DD)
			licenseKey.ExpiresAt = &expiresAt
		}
		// If neither format works, leave ExpiresAt unchanged
	}

	// Update max activations
	if maxActivations, err := strconv.Atoi(c.FormValue("max_activations")); err == nil {
		licenseKey.MaxActivations = maxActivations
	}

	// Update usage limit
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
			"CSRFToken":  "",
		})
	}

	return c.Redirect("/admin/license-keys/" + c.Params("id"))
}

func (h *LicenseKeysHandler) Delete(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	if err := h.db.Delete(&models.LicenseKey{}, id).Error; err != nil {
		return c.Status(500).SendString("Failed to delete license key")
	}

	return c.Redirect("/admin/license-keys")
}

func (h *LicenseKeysHandler) Revoke(c *fiber.Ctx) error {
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

func (h *LicenseKeysHandler) Reactivate(c *fiber.Ctx) error {
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

func (h *LicenseKeysHandler) SendEmail(c *fiber.Ctx) error {
	// This would require the email service to be injected
	// For now, just redirect back
	return c.Redirect("/admin/license-keys/" + c.Params("id"))
}
