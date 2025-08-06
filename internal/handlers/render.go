package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// SafeRender attempts to render a template with fallback to 500 error page
func SafeRender(c *fiber.Ctx, template string, data fiber.Map) error {
	// Try to render the template
	if err := c.Render(template, data); err != nil {
		// Template rendering failed, return 500 error page
		return render500HTML(c, "Template rendering failed")
	}
	return nil
}

// SafeRenderWithStatus renders a template with a specific status code and fallbacks
func SafeRenderWithStatus(c *fiber.Ctx, statusCode int, template string, data fiber.Map, errorMsg string) error {
	// Try to render the template with status
	if err := c.Status(statusCode).Render(template, data); err != nil {
		// Template rendering failed, return 500 error page
		return render500HTML(c, errorMsg)
	}
	return nil
}

// render500HTML returns a hardcoded 500 error page for production
func render500HTML(c *fiber.Ctx, errorMsg string) error {
	hardcodedHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Internal Server Error</title>
    <style>
        body { 
            font-family: Arial, sans-serif; 
            margin: 0; 
            padding: 40px; 
            background-color: #f8f9fa;
        }
        .error-container { 
            max-width: 600px; 
            margin: 0 auto; 
            text-align: center; 
            background: white;
            padding: 40px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .error-code { 
            font-size: 72px; 
            color: #dc3545; 
            font-weight: bold; 
            margin-bottom: 20px;
        }
        .error-message { 
            font-size: 18px; 
            color: #6c757d; 
            margin-bottom: 20px; 
        }
        .error-description {
            color: #495057;
            margin-bottom: 30px;
            line-height: 1.5;
        }
        .back-link { 
            color: #007bff; 
            text-decoration: none; 
            font-weight: 500;
        }
        .back-link:hover { 
            text-decoration: underline; 
        }
    </style>
</head>
<body>
    <div class="error-container">
        <div class="error-code">500</div>
        <div class="error-message">Internal Server Error</div>
        <div class="error-description">` + errorMsg + `</div>
        <p><a href="/admin/" class="back-link">← Back to Dashboard</a></p>
    </div>
</body>
</html>`
	return c.Status(500).Type("html").SendString(hardcodedHTML)
}
