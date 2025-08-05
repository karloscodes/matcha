package main

import (
	"log"

	"license-key-manager/internal/config"
	"license-key-manager/internal/database"
	"license-key-manager/internal/handlers"
	"license-key-manager/internal/middleware"
	"license-key-manager/internal/models"
	"license-key-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize configuration
	cfg := config.New()

	// Initialize authentication middleware
	middleware.InitAuth(cfg)

	// Initialize database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto-migrate database
	if err := db.AutoMigrate(&models.Product{}, &models.Customer{}, &models.LicenseKey{}, &models.AdminUser{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Create default admin user
	if err := models.CreateDefaultAdmin(db, "admin", "admin123"); err != nil {
		log.Println("Warning: Could not create default admin user:", err)
	}

	// Initialize services
	emailService := services.NewEmailService(cfg)

	// Initialize handlers
	adminHandler := handlers.NewAdminHandler(db)
	apiHandler := handlers.NewAPIHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db, emailService)

	// Initialize template engine
	engine := html.New("./templates", ".html")
	engine.Reload(cfg.IsDevelopment()) // Only reload in development
	engine.Debug(cfg.Debug)

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Add database to context
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("db", db)
		return c.Next()
	})

	// Method override middleware for HTML forms
	app.Use(func(c *fiber.Ctx) error {
		if c.Method() == "POST" {
			method := c.FormValue("_method")
			if method == "PUT" || method == "DELETE" || method == "PATCH" {
				c.Request().Header.SetMethod(method)
			}
		}
		return c.Next()
	})

	// Rate limiting - stricter for API endpoints
	app.Use("/api/v1/licenses/verify", limiter.New(limiter.Config{
		Max:        60,  // 60 requests per window
		Expiration: 60,  // 1 minute window
		KeyGenerator: func(c *fiber.Ctx) string {
			// Rate limit by IP address
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"error":   "Rate limit exceeded",
				"message": "Too many license verification requests. Please try again later.",
			})
		},
	}))

	// General API rate limiting (more lenient)
	app.Use("/api", limiter.New(limiter.Config{
		Max:        300, // 300 requests per window
		Expiration: 60,  // 1 minute window
	}))

	// Static files
	app.Static("/static", "./static")

	// Routes
	setupRoutes(app, adminHandler, apiHandler, webhookHandler)

	// Start server
	log.Printf("Server starting on port %s in %s environment", cfg.Port, cfg.Environment)
	log.Fatal(app.Listen(":" + cfg.Port))
}

func setupRoutes(app *fiber.App, adminHandler *handlers.AdminHandler, apiHandler *handlers.APIHandler, webhookHandler *handlers.WebhookHandler) {
	// Redirect root to admin
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/admin")
	})

	// Admin routes
	admin := app.Group("/admin")
	
	// Login routes (no CSRF protection) - MUST BE FIRST
	admin.Get("/login", adminHandler.LoginPage)
	admin.Post("/login", adminHandler.Login)
	admin.Get("/logout", adminHandler.Logout)
	
	// Authentication middleware with CSRF for protected routes
	adminProtected := admin.Group("/", middleware.RequireAuth, csrf.New(csrf.Config{
		KeyLookup:      "form:_token",
		CookieName:     "csrf_",
		CookieSameSite: "Lax",
		Expiration:     1 * 60 * 60, // 1 hour
		ContextKey:     "csrf",
	}))
	
	adminProtected.Get("/", adminHandler.Dashboard)

	// Products
	adminProtected.Get("/products", adminHandler.ProductsIndex)
	adminProtected.Get("/products/new", adminHandler.ProductsNew)
	adminProtected.Post("/products", adminHandler.ProductsCreate)
	adminProtected.Get("/products/:id", adminHandler.ProductsShow)
	adminProtected.Get("/products/:id/edit", adminHandler.ProductsEdit)
	adminProtected.Put("/products/:id", adminHandler.ProductsUpdate)
	adminProtected.Delete("/products/:id", adminHandler.ProductsDelete)

	// Customers
	adminProtected.Get("/customers", adminHandler.CustomersIndex)
	adminProtected.Get("/customers/new", adminHandler.CustomersNew)
	adminProtected.Post("/customers", adminHandler.CustomersCreate)
	adminProtected.Get("/customers/:id", adminHandler.CustomersShow)
	adminProtected.Get("/customers/:id/edit", adminHandler.CustomersEdit)
	adminProtected.Put("/customers/:id", adminHandler.CustomersUpdate)
	adminProtected.Delete("/customers/:id", adminHandler.CustomersDelete)

	// License Keys
	adminProtected.Get("/license-keys", adminHandler.LicenseKeysIndex)
	adminProtected.Get("/license-keys/new", adminHandler.LicenseKeysNew)
	adminProtected.Post("/license-keys", adminHandler.LicenseKeysCreate)
	adminProtected.Get("/license-keys/:id", adminHandler.LicenseKeysShow)
	adminProtected.Get("/license-keys/:id/edit", adminHandler.LicenseKeysEdit)
	adminProtected.Put("/license-keys/:id", adminHandler.LicenseKeysUpdate)
	adminProtected.Delete("/license-keys/:id", adminHandler.LicenseKeysDelete)
	adminProtected.Post("/license-keys/:id/revoke", adminHandler.LicenseKeysRevoke)
	adminProtected.Post("/license-keys/:id/reactivate", adminHandler.LicenseKeysReactivate)
	adminProtected.Post("/license-keys/:id/send-email", adminHandler.LicenseKeysSendEmail)

	// Email Configuration
	adminProtected.Get("/email-config", adminHandler.EmailConfigPage)
	adminProtected.Post("/email-config", adminHandler.EmailConfigUpdate)
	adminProtected.Post("/email-config/test", adminHandler.EmailTestSend)

	// API routes
	api := app.Group("/api/v1")
	api.Post("/licenses/verify", apiHandler.VerifyLicense)

	// Webhook routes
	api.Post("/webhooks/stripe", webhookHandler.StripeWebhook)
	api.Post("/webhooks/gumroad", webhookHandler.GumroadWebhook)
	api.Post("/webhooks/paypal", webhookHandler.PayPalWebhook)
}
