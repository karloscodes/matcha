package main

import (
	"log"
	"strings"

	"license-key-manager/internal/config"
	"license-key-manager/internal/database"
	"license-key-manager/internal/handlers"
	"license-key-manager/internal/middleware"
	"license-key-manager/internal/models"
	"license-key-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
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
	} else {
		log.Println("Loaded .env file successfully")
	}

	// Initialize configuration
	cfg := config.New()
	log.Printf("Configuration loaded - SecretKey: %s", cfg.SecretKey)

	// Initialize authentication middleware
	middleware.InitAuth(cfg)

	// Initialize database
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto-migrate database
	if err := db.AutoMigrate(&models.Product{}, &models.Customer{}, &models.LicenseKey{}, &models.AdminUser{}, &models.EmailSettings{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Create default admin user
	if err := models.CreateDefaultAdmin(db, "admin", "admin123"); err != nil {
		log.Println("Warning: Could not create default admin user:", err)
	}

	// Initialize services
	emailService := services.NewEmailService(cfg, db)

	// Initialize handlers
	dashboardHandler := handlers.NewDashboardHandler(db)
	usersHandler := handlers.NewUsersHandler(db)
	productsHandler := handlers.NewProductsHandler(db)
	customersHandler := handlers.NewCustomersHandler(db)
	licenseKeysHandler := handlers.NewLicenseKeysHandler(db)
	settingsHandler := handlers.NewSettingsHandler(db)
	apiHandler := handlers.NewAPIHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db, emailService)

	// Initialize template engine
	engine := html.New("./templates", ".gohtml")

	// Add template functions
	engine.AddFunc("dict", func(values ...interface{}) map[string]interface{} {
		dict := make(map[string]interface{})
		for i := 0; i < len(values); i += 2 {
			if i+1 < len(values) {
				key, ok := values[i].(string)
				if ok {
					dict[key] = values[i+1]
				}
			}
		}
		return dict
	})

	// Don't use global layout - templates will extend layouts manually
	// engine.Layout("layouts/base")
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
		if c.Method() == fiber.MethodPost {
			method := c.FormValue("_method")
			if method != "" {
				method = strings.ToUpper(method)
				if method == fiber.MethodPut || method == fiber.MethodDelete || method == fiber.MethodPatch {
					c.Request().Header.SetMethod(method)
				}
			}
		}
		return c.Next()
	})

	// Rate limiting - stricter for API endpoints
	app.Use("/api/v1/licenses/verify", limiter.New(limiter.Config{
		Max:        60, // 60 requests per window
		Expiration: 60, // 1 minute window
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
	setupRoutes(app, dashboardHandler, usersHandler, productsHandler, customersHandler, licenseKeysHandler, settingsHandler, apiHandler, webhookHandler)

	// Start server
	log.Printf("Server starting on port %s in %s environment", cfg.Port, cfg.Environment)
	log.Fatal(app.Listen(":" + cfg.Port))
}

func setupRoutes(app *fiber.App, dashboardHandler *handlers.DashboardHandler, usersHandler *handlers.UsersHandler, productsHandler *handlers.ProductsHandler, customersHandler *handlers.CustomersHandler, licenseKeysHandler *handlers.LicenseKeysHandler, settingsHandler *handlers.SettingsHandler, apiHandler *handlers.APIHandler, webhookHandler *handlers.WebhookHandler) {
	// Redirect root to admin dashboard
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/admin/")
	})

	// Admin routes
	admin := app.Group("/admin")

	// Login routes (no CSRF protection) - MUST BE FIRST
	admin.Get("/login", usersHandler.LoginPage)
	admin.Post("/login", usersHandler.Login)
	admin.Get("/logout", usersHandler.Logout)

	// Authentication middleware for protected routes
	adminProtected := admin.Group("/", middleware.RequireAuth)

	adminProtected.Get("/", dashboardHandler.Dashboard)

	// Products
	adminProtected.Get("/products", productsHandler.Index)
	adminProtected.Get("/products/new", productsHandler.New)
	adminProtected.Post("/products", productsHandler.Create)
	adminProtected.Get("/products/:id", productsHandler.Show)
	adminProtected.Get("/products/:id/edit", productsHandler.Edit)
	adminProtected.Put("/products/:id", productsHandler.Update)
	adminProtected.Post("/products/:id", productsHandler.Update) // For form method override
	adminProtected.Delete("/products/:id", productsHandler.Delete)

	// Customers
	adminProtected.Get("/customers", customersHandler.Index)
	adminProtected.Get("/customers/new", customersHandler.New)
	adminProtected.Post("/customers", customersHandler.Create)
	adminProtected.Get("/customers/:id", customersHandler.Show)
	adminProtected.Get("/customers/:id/edit", customersHandler.Edit)
	adminProtected.Put("/customers/:id", customersHandler.Update)
	adminProtected.Post("/customers/:id", customersHandler.Update) // For form method override
	adminProtected.Delete("/customers/:id", customersHandler.Delete)

	// License Keys
	adminProtected.Get("/license-keys", licenseKeysHandler.Index)
	adminProtected.Get("/license-keys/new", licenseKeysHandler.New)
	adminProtected.Post("/license-keys", licenseKeysHandler.Create)
	adminProtected.Get("/license-keys/:id", licenseKeysHandler.Show)
	adminProtected.Get("/license-keys/:id/edit", licenseKeysHandler.Edit)
	adminProtected.Put("/license-keys/:id", licenseKeysHandler.Update)
	adminProtected.Post("/license-keys/:id", licenseKeysHandler.Update) // For form method override
	adminProtected.Delete("/license-keys/:id", licenseKeysHandler.Delete)
	adminProtected.Post("/license-keys/:id/revoke", licenseKeysHandler.Revoke)
	adminProtected.Post("/license-keys/:id/reactivate", licenseKeysHandler.Reactivate)
	adminProtected.Post("/license-keys/:id/send-email", licenseKeysHandler.SendEmail)

	// Settings
	adminProtected.Get("/settings/email", settingsHandler.ShowEmailSettings)
	adminProtected.Post("/settings/email", settingsHandler.CreateEmailSettings)
	adminProtected.Post("/settings/email/:id", settingsHandler.UpdateEmailSettings)
	adminProtected.Put("/settings/email/:id", settingsHandler.UpdateEmailSettings)
	adminProtected.Post("/settings/email/:id/activate", settingsHandler.ActivateEmailSettings)
	adminProtected.Delete("/settings/email/:id", settingsHandler.DeleteEmailSettings)
	adminProtected.Post("/settings/email/test", settingsHandler.TestEmailSettings)

	// Email Configuration (legacy - keeping for compatibility)
	adminProtected.Get("/email-config", dashboardHandler.EmailConfigPage)
	adminProtected.Post("/email-config", dashboardHandler.EmailConfigUpdate)
	adminProtected.Post("/email-config/test", dashboardHandler.EmailTestSend)

	// API routes
	api := app.Group("/api/v1")
	api.Post("/licenses/verify", apiHandler.VerifyLicense)

	// Webhook routes
	api.Post("/webhooks/stripe", webhookHandler.StripeWebhook)
	api.Post("/webhooks/gumroad", webhookHandler.GumroadWebhook)
	api.Post("/webhooks/paypal", webhookHandler.PayPalWebhook)
}
