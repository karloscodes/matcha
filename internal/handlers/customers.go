package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"matcha/internal/database"
	"matcha/internal/models"
)

type CustomersHandler struct {
	db *gorm.DB
}

func NewCustomersHandler(db *gorm.DB) *CustomersHandler {
	return &CustomersHandler{db: db}
}

func (h *CustomersHandler) Index(c *fiber.Ctx) error {
	var customers []models.Customer
	h.db.Preload("LicenseKeys").Find(&customers)

	return c.Render("admin/customers/index", fiber.Map{
		"ShowNav":   true,
		"PageType":  "customers-index",
		"Customers": customers,
		"CSRFToken": "",
	})
}

func (h *CustomersHandler) New(c *fiber.Ctx) error {
	return c.Render("admin/customers/new", fiber.Map{
		"ShowNav":   true,
		"PageType":  "customers-new",
		"CSRFToken": "",
	})
}

func (h *CustomersHandler) Create(c *fiber.Ctx) error {
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

func (h *CustomersHandler) Show(c *fiber.Ctx) error {
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

func (h *CustomersHandler) Edit(c *fiber.Ctx) error {
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

func (h *CustomersHandler) Update(c *fiber.Ctx) error {
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

func (h *CustomersHandler) Delete(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	err := database.PerformWrite(h.db, func(db *gorm.DB) error {
		return db.Delete(&models.Customer{}, id).Error
	})
	if err != nil {
		return c.Status(500).SendString("Failed to delete customer")
	}

	return c.Redirect("/admin/customers")
}
