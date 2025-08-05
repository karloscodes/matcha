package middleware

import (
	"time"
	"license-key-manager/internal/config"
	"license-key-manager/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"gorm.io/gorm"
)

var store *session.Store

func InitAuth(cfg *config.Config) {
	store = session.New(session.Config{
		KeyLookup:      "cookie:license_mgr_session",
		Expiration:     30 * 24 * time.Hour, // 30 days
		CookieHTTPOnly: true,                // Prevent XSS attacks
		CookieSecure:   cfg.IsProduction(),  // Use secure cookies in production
		CookieSameSite: "Lax",               // CSRF protection
		CookiePath:     "/",                 // Cookie available for entire site
	})
}

func RequireAuth(c *fiber.Ctx) error {
	sess, err := store.Get(c)
	if err != nil {
		return c.Redirect("/admin/login")
	}

	adminID := sess.Get("admin_user_id")
	if adminID == nil {
		return c.Redirect("/admin/login")
	}

	// Get database from context
	db, ok := c.Locals("db").(*gorm.DB)
	if !ok {
		return c.Redirect("/admin/login")
	}

	// Verify admin still exists
	var admin models.AdminUser
	if err := db.First(&admin, adminID).Error; err != nil {
		sess.Destroy()
		return c.Redirect("/admin/login")
	}

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
	sess, err := store.Get(c)
	if err != nil {
		return err
	}

	sess.Set("admin_user_id", adminID)
	return sess.Save()
}

func Logout(c *fiber.Ctx) error {
	sess, err := store.Get(c)
	if err != nil {
		return err
	}

	return sess.Destroy()
}