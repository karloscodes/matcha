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
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			
			switch code {
			case 404:
				return c.Status(404).Render("errors/404", fiber.Map{
					"Title": "Page Not Found",
				})
			case 500:
				return c.Status(500).Render("errors/500", fiber.Map{
					"Title": "Server Error", 
					"Error": err.Error(),
				})
			default:
				return c.Status(code).Render("errors/500", fiber.Map{
					"Title": "Error",
					"Error": err.Error(),
				})
			}
		},
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

	// Admin login routes (no auth required)
	app.Get("/admin/login", usersHandler.LoginPage)
	app.Post("/admin/login", usersHandler.Login)
	app.Get("/admin/logout", usersHandler.Logout)

	// Dashboard route with auth
	app.Get("/admin/", middleware.RequireAuth, dashboardHandler.Dashboard)

	// Products
	app.Get("/admin/products", middleware.RequireAuth, productsHandler.Index)
	app.Get("/admin/products/new", middleware.RequireAuth, productsHandler.New)
	app.Post("/admin/products", middleware.RequireAuth, productsHandler.Create)
	app.Get("/admin/products/:id", middleware.RequireAuth, productsHandler.Show)
	app.Get("/admin/products/:id/edit", middleware.RequireAuth, productsHandler.Edit)
	app.Put("/admin/products/:id", middleware.RequireAuth, productsHandler.Update)
	app.Post("/admin/products/:id", middleware.RequireAuth, productsHandler.Update) // For form method override
	app.Delete("/admin/products/:id", middleware.RequireAuth, productsHandler.Delete)

	// Customers
	app.Get("/admin/customers", middleware.RequireAuth, customersHandler.Index)
	app.Get("/admin/customers/new", middleware.RequireAuth, customersHandler.New)
	app.Post("/admin/customers", middleware.RequireAuth, customersHandler.Create)
	app.Get("/admin/customers/:id", middleware.RequireAuth, customersHandler.Show)
	app.Get("/admin/customers/:id/edit", middleware.RequireAuth, customersHandler.Edit)
	app.Put("/admin/customers/:id", middleware.RequireAuth, customersHandler.Update)
	app.Post("/admin/customers/:id", middleware.RequireAuth, customersHandler.Update) // For form method override
	app.Delete("/admin/customers/:id", middleware.RequireAuth, customersHandler.Delete)

	// License Keys
	app.Get("/admin/license-keys", middleware.RequireAuth, licenseKeysHandler.Index)
	app.Get("/admin/license-keys/new", middleware.RequireAuth, licenseKeysHandler.New)
	app.Post("/admin/license-keys", middleware.RequireAuth, licenseKeysHandler.Create)
	app.Get("/admin/license-keys/:id", middleware.RequireAuth, licenseKeysHandler.Show)
	app.Get("/admin/license-keys/:id/edit", middleware.RequireAuth, licenseKeysHandler.Edit)
	app.Put("/admin/license-keys/:id", middleware.RequireAuth, licenseKeysHandler.Update)
	app.Post("/admin/license-keys/:id", middleware.RequireAuth, licenseKeysHandler.Update) // For form method override
	app.Delete("/admin/license-keys/:id", middleware.RequireAuth, licenseKeysHandler.Delete)
	app.Post("/admin/license-keys/:id/revoke", middleware.RequireAuth, licenseKeysHandler.Revoke)
	app.Post("/admin/license-keys/:id/reactivate", middleware.RequireAuth, licenseKeysHandler.Reactivate)
	app.Post("/admin/license-keys/:id/send-email", middleware.RequireAuth, licenseKeysHandler.SendEmail)

	// Settings
	app.Get("/admin/settings/email", middleware.RequireAuth, settingsHandler.ShowEmailSettings)
	app.Post("/admin/settings/email", middleware.RequireAuth, settingsHandler.CreateEmailSettings)
	app.Post("/admin/settings/email/:id", middleware.RequireAuth, settingsHandler.UpdateEmailSettings)
	app.Put("/admin/settings/email/:id", middleware.RequireAuth, settingsHandler.UpdateEmailSettings)
	app.Post("/admin/settings/email/:id/activate", middleware.RequireAuth, settingsHandler.ActivateEmailSettings)
	app.Delete("/admin/settings/email/:id", middleware.RequireAuth, settingsHandler.DeleteEmailSettings)
	app.Post("/admin/settings/email/test", middleware.RequireAuth, settingsHandler.TestEmailSettings)

	// Email Configuration (legacy - keeping for compatibility)
	app.Get("/admin/email-config", middleware.RequireAuth, dashboardHandler.EmailConfigPage)
	app.Post("/admin/email-config", middleware.RequireAuth, dashboardHandler.EmailConfigUpdate)
	app.Post("/admin/email-config/test", middleware.RequireAuth, dashboardHandler.EmailTestSend)

	// API routes
	api := app.Group("/api/v1")
	api.Post("/licenses/verify", apiHandler.VerifyLicense)

	// Webhook routes
	api.Post("/webhooks/stripe", webhookHandler.StripeWebhook)
	api.Post("/webhooks/gumroad", webhookHandler.GumroadWebhook)
	api.Post("/webhooks/paypal", webhookHandler.PayPalWebhook)
	
	
	// 404 handler - must be last
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).Render("errors/404", fiber.Map{
			"Title": "Page Not Found",
		})
	})
}
