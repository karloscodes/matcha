package handlers

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"matcha/internal/database"
	"matcha/internal/models"
)

type ProductsHandler struct {
	db *gorm.DB
}

func NewProductsHandler(db *gorm.DB) *ProductsHandler {
	return &ProductsHandler{db: db}
}

func (h *ProductsHandler) Index(c *fiber.Ctx) error {
	var products []models.Product
	h.db.Preload("LicenseKeys").Find(&products)

	return SafeRender(c, "admin/products/index", fiber.Map{
		"ShowNav":   true,
		"PageType":  "products-index",
		"Products":  products,
		"CSRFToken": "",
	})
}

func (h *ProductsHandler) New(c *fiber.Ctx) error {
	return SafeRender(c, "admin/products/new", fiber.Map{
		"ShowNav":   true,
		"PageType":  "products-new",
		"CSRFToken": "",
	})
}

func (h *ProductsHandler) Create(c *fiber.Ctx) error {
	log.Printf("ProductsCreate: Method=%s, Path=%s", c.Method(), c.Path())
	log.Printf("ProductsCreate: Form values - name=%s, description=%s, version=%s",
		c.FormValue("name"), c.FormValue("description"), c.FormValue("version"))

	// Validate required fields
	name := c.FormValue("name")
	if name == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Product name is required",
		})
	}

	product := models.Product{
		Name:        name,
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
		return SafeRenderWithStatus(c, 500, "admin/products/new", fiber.Map{
			"Error":   "Failed to create product: " + err.Error(),
			"Product": product,
			"ShowNav": true,
		}, "Failed to create product: "+err.Error())
	}

	return c.Redirect("/admin/products")
}

func (h *ProductsHandler) Show(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var product models.Product
	if err := h.db.Preload("LicenseKeys.Customer").First(&product, id).Error; err != nil {
		return c.Status(404).SendString("Product not found")
	}

	// Try to render template, fallback to JSON if no template engine
	if err := c.Render("admin/products/show", fiber.Map{
		"ShowNav":  true,
		"PageType": "products-show",
		"Product":  product,
	}); err != nil {
		return c.Status(200).JSON(fiber.Map{
			"product": product,
		})
	}
	return nil
}

func (h *ProductsHandler) Edit(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var product models.Product
	if err := h.db.First(&product, id).Error; err != nil {
		return c.Status(404).SendString("Product not found")
	}

	// Try to render template, fallback to JSON if no template engine
	if err := c.Render("admin/products/edit", fiber.Map{
		"ShowNav":   true,
		"PageType":  "products-edit",
		"Product":   product,
		"CSRFToken": "",
	}); err != nil {
		return c.Status(200).JSON(fiber.Map{
			"product": product,
		})
	}
	return nil
}

func (h *ProductsHandler) Update(c *fiber.Ctx) error {
	// Accept both PUT requests and POST requests with _method=PUT
	if c.Method() != "PUT" && !(c.Method() == "POST" && c.FormValue("_method") == "PUT") {
		return c.Status(405).SendString("Method not allowed")
	}

	id, _ := strconv.Atoi(c.Params("id"))
	var product models.Product
	if err := h.db.First(&product, id).Error; err != nil {
		return c.Status(404).SendString("Product not found")
	}

	// Only update non-empty fields
	if name := c.FormValue("name"); name != "" {
		product.Name = name
	}
	if description := c.FormValue("description"); description != "" {
		product.Description = description
	}
	if version := c.FormValue("version"); version != "" {
		product.Version = version
	}

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
		// Try to render template, fallback to JSON error
		if renderErr := c.Render("admin/products/edit", fiber.Map{
			"Error":     "Failed to update product: " + err.Error(),
			"Product":   product,
			"CSRFToken": "",
		}); renderErr != nil {
			return c.Status(400).JSON(fiber.Map{
				"error": "Failed to update product: " + err.Error(),
			})
		}
		return nil
	}

	return c.Redirect("/admin/products/" + c.Params("id"))
}

func (h *ProductsHandler) Delete(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	// Check if product has associated license keys
	var licenseKeyCount int64
	h.db.Model(&models.LicenseKey{}).Where("product_id = ?", id).Count(&licenseKeyCount)

	if licenseKeyCount > 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "Cannot delete product with associated license keys",
		})
	}

	if err := h.db.Delete(&models.Product{}, id).Error; err != nil {
		return c.Status(500).SendString("Failed to delete product")
	}

	return c.Redirect("/admin/products")
}
