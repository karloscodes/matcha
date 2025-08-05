package handlers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"license-key-manager/internal/middleware"
	"license-key-manager/internal/models"
)

type UsersHandler struct {
	db *gorm.DB
}

func NewUsersHandler(db *gorm.DB) *UsersHandler {
	return &UsersHandler{db: db}
}

func (h *UsersHandler) LoginPage(c *fiber.Ctx) error {
	return c.Render("admin/users/login", fiber.Map{
		"ShowNav": false,
		"Title":   "Login",
	})
}

func (h *UsersHandler) Login(c *fiber.Ctx) error {
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

func (h *UsersHandler) Logout(c *fiber.Ctx) error {
	middleware.Logout(c)
	return c.Redirect("/admin/login")
}