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

	return c.Render("admin/products/index", fiber.Map{
		"ShowNav":   true,
		"PageType":  "products-index",
		"Products":  products,
		"CSRFToken": "",
	})
}

func (h *ProductsHandler) New(c *fiber.Ctx) error {
	return c.Render("admin/products/new", fiber.Map{
		"ShowNav":   true,
		"PageType":  "products-new",
		"CSRFToken": "",
	})
}

func (h *ProductsHandler) Create(c *fiber.Ctx) error {
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

func (h *ProductsHandler) Show(c *fiber.Ctx) error {
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

func (h *ProductsHandler) Edit(c *fiber.Ctx) error {
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

func (h *ProductsHandler) Delete(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	if err := h.db.Delete(&models.Product{}, id).Error; err != nil {
		return c.Status(500).SendString("Failed to delete product")
	}

	return c.Redirect("/admin/products")
}
