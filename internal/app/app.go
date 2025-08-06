package app

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	htmlEngine "github.com/gofiber/template/html/v2"
	"gorm.io/gorm"

	"matcha/internal/config"
	"matcha/internal/handlers"
	"matcha/internal/middleware"
	"matcha/internal/services"
)

// NewApp creates and configures a new Fiber application with all middleware and routes
func NewApp(cfg *config.Config, db *gorm.DB, templateFS embed.FS, staticFS embed.FS) *fiber.App {
	// Initialize authentication middleware
	middleware.InitAuth(cfg)

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

	// Initialize template engine - use filesystem in development, embedded in production
	var engine *htmlEngine.Engine
	if cfg.IsDevelopment() {
		// In development, use regular filesystem for template reloading
		engine = htmlEngine.New("./templates", ".gohtml")
		engine.Reload(true) // Enable template reloading in development
	} else {
		// In production, use embedded templates
		engine = htmlEngine.NewFileSystem(http.FS(templateFS), ".gohtml")
		engine.Reload(false) // Disable reloading in production
	}

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

	// Static files - use filesystem in development, embedded in production
	if cfg.IsDevelopment() {
		// In development, serve from regular filesystem
		app.Static("/static", "./static")
	} else {
		// In production, serve from embedded filesystem
		staticSubFS, _ := fs.Sub(staticFS, "static")
		app.Use("/static", filesystem.New(filesystem.Config{
			Root: http.FS(staticSubFS),
		}))
	}

	// Routes
	setupRoutes(app, dashboardHandler, usersHandler, productsHandler, customersHandler, licenseKeysHandler, settingsHandler, apiHandler, webhookHandler)

	return app
}

func setupRoutes(app *fiber.App, dashboardHandler *handlers.DashboardHandler, usersHandler *handlers.UsersHandler, productsHandler *handlers.ProductsHandler, customersHandler *handlers.CustomersHandler, licenseKeysHandler *handlers.LicenseKeysHandler, settingsHandler *handlers.SettingsHandler, apiHandler *handlers.APIHandler, webhookHandler *handlers.WebhookHandler) {
	// Redirect root to admin dashboard
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/admin/")
	})

	// Admin routes
	admin := app.Group("/admin")

	// Public admin routes (no auth required)
	admin.Get("/login", usersHandler.LoginPage)
	admin.Post("/login", usersHandler.Login)
	admin.Get("/logout", usersHandler.Logout)

	// Protected admin routes
	admin.Get("/", middleware.RequireAuth, dashboardHandler.Dashboard)

	// Products
	admin.Get("/products", middleware.RequireAuth, productsHandler.Index)
	admin.Get("/products/new", middleware.RequireAuth, productsHandler.New)
	admin.Post("/products", middleware.RequireAuth, productsHandler.Create)
	admin.Get("/products/:id", middleware.RequireAuth, productsHandler.Show)
	admin.Get("/products/:id/edit", middleware.RequireAuth, productsHandler.Edit)
	admin.Put("/products/:id", middleware.RequireAuth, productsHandler.Update)
	admin.Post("/products/:id", middleware.RequireAuth, productsHandler.Update) // For form method override
	admin.Delete("/products/:id", middleware.RequireAuth, productsHandler.Delete)

	// Customers
	admin.Get("/customers", middleware.RequireAuth, customersHandler.Index)
	admin.Get("/customers/new", middleware.RequireAuth, customersHandler.New)
	admin.Post("/customers", middleware.RequireAuth, customersHandler.Create)
	admin.Get("/customers/:id", middleware.RequireAuth, customersHandler.Show)
	admin.Get("/customers/:id/edit", middleware.RequireAuth, customersHandler.Edit)
	admin.Put("/customers/:id", middleware.RequireAuth, customersHandler.Update)
	admin.Post("/customers/:id", middleware.RequireAuth, customersHandler.Update) // For form method override
	admin.Delete("/customers/:id", middleware.RequireAuth, customersHandler.Delete)

	// License Keys
	admin.Get("/license-keys", middleware.RequireAuth, licenseKeysHandler.Index)
	admin.Get("/license-keys/new", middleware.RequireAuth, licenseKeysHandler.New)
	admin.Post("/license-keys", middleware.RequireAuth, licenseKeysHandler.Create)
	admin.Get("/license-keys/:id", middleware.RequireAuth, licenseKeysHandler.Show)
	admin.Get("/license-keys/:id/edit", middleware.RequireAuth, licenseKeysHandler.Edit)
	admin.Put("/license-keys/:id", middleware.RequireAuth, licenseKeysHandler.Update)
	admin.Post("/license-keys/:id", middleware.RequireAuth, licenseKeysHandler.Update) // For form method override
	admin.Delete("/license-keys/:id", middleware.RequireAuth, licenseKeysHandler.Delete)
	admin.Post("/license-keys/:id/revoke", middleware.RequireAuth, licenseKeysHandler.Revoke)
	admin.Post("/license-keys/:id/reactivate", middleware.RequireAuth, licenseKeysHandler.Reactivate)
	admin.Post("/license-keys/:id/send-email", middleware.RequireAuth, licenseKeysHandler.SendEmail)

	// Settings
	admin.Get("/settings/email", middleware.RequireAuth, settingsHandler.ShowEmailSettings)
	admin.Post("/settings/email", middleware.RequireAuth, settingsHandler.CreateEmailSettings)
	admin.Post("/settings/email/:id", middleware.RequireAuth, settingsHandler.UpdateEmailSettings)
	admin.Put("/settings/email/:id", middleware.RequireAuth, settingsHandler.UpdateEmailSettings)
	admin.Post("/settings/email/:id/activate", middleware.RequireAuth, settingsHandler.ActivateEmailSettings)
	admin.Delete("/settings/email/:id", middleware.RequireAuth, settingsHandler.DeleteEmailSettings)
	admin.Post("/settings/email/test", middleware.RequireAuth, settingsHandler.TestEmailSettings)

	// Email Configuration (legacy - keeping for compatibility)
	admin.Get("/email-config", middleware.RequireAuth, dashboardHandler.EmailConfigPage)
	admin.Post("/email-config", middleware.RequireAuth, dashboardHandler.EmailConfigUpdate)
	admin.Post("/email-config/test", middleware.RequireAuth, dashboardHandler.EmailTestSend)

	// Catch-all for non-existent admin routes - must be last in admin group
	admin.All("/*", func(c *fiber.Ctx) error {
		return c.Status(404).Render("errors/404", fiber.Map{
			"Title": "Page Not Found",
		})
	})

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
