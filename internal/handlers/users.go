package handlers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"matcha/internal/middleware"
	"matcha/internal/models"
)

type UsersHandler struct {
	db *gorm.DB
}

func NewUsersHandler(db *gorm.DB) *UsersHandler {
	return &UsersHandler{db: db}
}

func (h *UsersHandler) LoginPage(c *fiber.Ctx) error {
	return SafeRender(c, "admin/users/login", fiber.Map{
		"ShowNav": false,
		"Title":   "Login",
	})
}

func (h *UsersHandler) Login(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	// Validate input
	if username == "" || password == "" {
		return SafeRenderWithStatus(c, 200, "admin/users/login", fiber.Map{
			"Error":   "Username and password are required",
			"ShowNav": false,
			"Title":   "Login",
		}, "Username and password are required")
	}

	var admin models.AdminUser
	if err := h.db.Where("username = ?", username).First(&admin).Error; err != nil {
		return SafeRenderWithStatus(c, 200, "admin/users/login", fiber.Map{
			"Error":   "Invalid username or password",
			"ShowNav": false,
			"Title":   "Login",
		}, "Invalid username or password")
	}

	if !admin.CheckPassword(password) {
		return SafeRenderWithStatus(c, 200, "admin/users/login", fiber.Map{
			"Error":   "Invalid username or password",
			"ShowNav": false,
			"Title":   "Login",
		}, "Invalid username or password")
	}

	if err := middleware.Login(c, admin.ID); err != nil {
		return c.Status(500).SendString("Login failed")
	}

	return c.Redirect("/admin/")
}

func (h *UsersHandler) Logout(c *fiber.Ctx) error {
	_ = middleware.Logout(c)
	return c.Redirect("/admin/login")
}
