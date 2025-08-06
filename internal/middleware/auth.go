package middleware

import (
	"log"
	"strconv"
	"time"

	"license-key-manager/internal/config"
	"license-key-manager/internal/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

var secretKey []byte

func InitAuth(cfg *config.Config) {
	log.Printf("Initializing auth with SecretKey: %s", cfg.SecretKey)
	secretKey = []byte(cfg.SecretKey)
}

func RequireAuth(c *fiber.Ctx) error {
	log.Printf("RequireAuth: Checking authentication for path: %s, method: %s", c.Path(), c.Method())

	// Get admin ID from cookie
	adminIDStr := c.Cookies("admin_user_id")
	if adminIDStr == "" {
		log.Printf("RequireAuth: No admin_user_id cookie, redirecting to login")
		return c.Redirect("/admin/login")
	}

	adminID, err := strconv.ParseUint(adminIDStr, 10, 32)
	if err != nil {
		log.Printf("RequireAuth: Invalid admin_user_id cookie: %v", err)
		c.ClearCookie("admin_user_id")
		return c.Redirect("/admin/login")
	}

	// Get database from context
	db, ok := c.Locals("db").(*gorm.DB)
	if !ok {
		log.Printf("RequireAuth: Could not get database from context")
		return c.Redirect("/admin/login")
	}

	// Verify admin still exists
	var admin models.AdminUser
	if err := db.First(&admin, uint(adminID)).Error; err != nil {
		log.Printf("RequireAuth: Admin user not found in database: %v", err)
		c.ClearCookie("admin_user_id")
		return c.Redirect("/admin/login")
	}

	log.Printf("RequireAuth: Authentication successful for admin: %s", admin.Username)
	c.Locals("current_admin", &admin)
	return c.Next()
}

func GetCurrentAdmin(c *fiber.Ctx) *models.AdminUser {
	admin, ok := c.Locals("current_admin").(*models.AdminUser)
	if !ok {
		return nil
	}
	return admin
}

func Login(c *fiber.Ctx, adminID uint) error {
	// Set persistent cookie
	c.Cookie(&fiber.Cookie{
		Name:     "admin_user_id",
		Value:    strconv.FormatUint(uint64(adminID), 10),
		Expires:  time.Now().Add(30 * 24 * time.Hour), // 30 days
		HTTPOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: "Lax",
		Path:     "/",
	})

	log.Printf("Login: Successfully set cookie for admin ID: %d", adminID)
	return nil
}

func Logout(c *fiber.Ctx) error {
	// Clear the cookie
	c.ClearCookie("admin_user_id")
	return nil
}
