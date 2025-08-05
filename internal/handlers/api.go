package handlers

import (
	"license-key-manager/internal/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type APIHandler struct {
	db *gorm.DB
}

func NewAPIHandler(db *gorm.DB) *APIHandler {
	return &APIHandler{db: db}
}

func (h *APIHandler) VerifyLicense(c *fiber.Ctx) error {
	productIDStr := c.FormValue("product_id")
	licenseKey := c.FormValue("license_key")
	incrementUsesStr := c.FormValue("increment_uses_count")

	if productIDStr == "" || licenseKey == "" {
		return c.Status(404).JSON(fiber.Map{"success": false})
	}

	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false})
	}

	var product models.Product
	if err := h.db.First(&product, productID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false})
	}

	var license models.LicenseKey
	if err := h.db.Preload("Product").Preload("Customer").
		Where("product_id = ? AND key = ?", productID, licenseKey).
		First(&license).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false})
	}

	if !license.IsValidForUse() {
		return c.Status(404).JSON(fiber.Map{"success": false})
	}

	// Check if we should increment usage count (default is true)
	incrementUses := incrementUsesStr != "false"
	if incrementUses {
		if err := license.IncrementUsage(h.db); err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false})
		}
	}

	return c.JSON(license.ToAPIResponse())
}