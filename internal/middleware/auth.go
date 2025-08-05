package middleware

import (
	"log"
	"time"

	"license-key-manager/internal/config"
	"license-key-manager/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"gorm.io/gorm"
)

var store *session.Store

func InitAuth(cfg *config.Config) {
	log.Printf("Initializing auth with SecretKey: %s", cfg.SecretKey)
	store = session.New(session.Config{
		KeyLookup:      "cookie:license_mgr_session",
		Expiration:     30 * 24 * time.Hour, // 30 days
		CookieHTTPOnly: true,                // Prevent XSS attacks
		CookieSecure:   cfg.IsProduction(),  // Use secure cookies in production
		CookieSameSite: "Lax",               // CSRF protection
		CookiePath:     "/",                 // Cookie available for entire site
		// Remove KeyGenerator to use default UUID generation
	})
}

func RequireAuth(c *fiber.Ctx) error {
	log.Printf("RequireAuth: Checking authentication for path: %s, method: %s", c.Path(), c.Method())

	sess, err := store.Get(c)
	if err != nil {
		log.Printf("RequireAuth: Session error: %v", err)
		return c.Redirect("/admin/login")
	}

	adminID := sess.Get("admin_user_id")
	log.Printf("RequireAuth: Session admin_user_id: %v", adminID)

	if adminID == nil {
		log.Printf("RequireAuth: No admin_user_id in session, redirecting to login")
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
	if err := db.First(&admin, adminID).Error; err != nil {
		log.Printf("RequireAuth: Admin user not found in database: %v", err)
		sess.Destroy()
		return c.Redirect("/admin/login")
	}

	log.Printf("RequireAuth: Authentication successful for admin: %s", admin.Username)
	log.Printf("RequireAuth: About to set c.Locals")
	c.Locals("current_admin", &admin)
	log.Printf("RequireAuth: c.Locals set successfully")
	log.Printf("RequireAuth: Proceeding to next middleware/handler")
	err = c.Next()
	log.Printf("RequireAuth: c.Next() returned with error: %v", err)
	return err
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
		log.Printf("Login: Error getting session: %v", err)
		return err
	}

	sess.Set("admin_user_id", adminID)
	err = sess.Save()
	if err != nil {
		log.Printf("Login: Error saving session: %v", err)
		return err
	}

	log.Printf("Login: Successfully saved session for admin ID: %d", adminID)
	return nil
}

func Logout(c *fiber.Ctx) error {
	sess, err := store.Get(c)
	if err != nil {
		return err
	}

	return sess.Destroy()
}
