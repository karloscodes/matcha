package handlers

import (
	"encoding/json"
	"log"
	"matcha/internal/models"
	"matcha/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	db           *gorm.DB
	emailService *services.EmailService
}

func NewWebhookHandler(db *gorm.DB, emailService *services.EmailService) *WebhookHandler {
	return &WebhookHandler{
		db:           db,
		emailService: emailService,
	}
}

func (h *WebhookHandler) StripeWebhook(c *fiber.Ctx) error {
	var eventData map[string]interface{}
	if err := json.Unmarshal(c.Body(), &eventData); err != nil {
		log.Printf("Stripe webhook error parsing JSON: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
	}

	eventType, ok := eventData["type"].(string)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Missing event type"})
	}

	if eventType == "checkout.session.completed" || eventType == "payment_intent.succeeded" {
		data, ok := eventData["data"].(map[string]interface{})
		if !ok {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid data structure"})
		}

		object, ok := data["object"].(map[string]interface{})
		if !ok {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid object structure"})
		}

		var email, name, productID string

		// Try to get email from customer_details
		if customerDetails, ok := object["customer_details"].(map[string]interface{}); ok {
			if e, ok := customerDetails["email"].(string); ok {
				email = e
			}
			if n, ok := customerDetails["name"].(string); ok {
				name = n
			}
		}

		// Fallback to receipt_email
		if email == "" {
			if e, ok := object["receipt_email"].(string); ok {
				email = e
			}
		}

		// Get product ID from metadata
		if metadata, ok := object["metadata"].(map[string]interface{}); ok {
			if p, ok := metadata["product_id"].(string); ok {
				productID = p
			}
		}

		if err := h.processSuccessfulPayment(email, name, productID, eventData); err != nil {
			log.Printf("Stripe webhook processing error: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return c.JSON(fiber.Map{"received": true})
}

func (h *WebhookHandler) GumroadWebhook(c *fiber.Ctx) error {
	email := c.FormValue("email")
	name := c.FormValue("full_name")
	if name == "" {
		name = c.FormValue("purchaser_name")
	}
	productID := c.FormValue("product_id")

	// Convert form data to map for storage
	formData := make(map[string]interface{})
	c.Request().PostArgs().VisitAll(func(key, value []byte) {
		formData[string(key)] = string(value)
	})

	if err := h.processSuccessfulPayment(email, name, productID, formData); err != nil {
		log.Printf("Gumroad webhook processing error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"received": true})
}

func (h *WebhookHandler) PayPalWebhook(c *fiber.Ctx) error {
	var eventData map[string]interface{}
	if err := json.Unmarshal(c.Body(), &eventData); err != nil {
		log.Printf("PayPal webhook error parsing JSON: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
	}

	eventType, ok := eventData["event_type"].(string)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "Missing event type"})
	}

	if eventType == "PAYMENT.SALE.COMPLETED" {
		resource, ok := eventData["resource"].(map[string]interface{})
		if !ok {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid resource structure"})
		}

		var email, name, productID string

		if payer, ok := resource["payer"].(map[string]interface{}); ok {
			if payerInfo, ok := payer["payer_info"].(map[string]interface{}); ok {
				if e, ok := payerInfo["email"].(string); ok {
					email = e
				}
				if fn, ok := payerInfo["first_name"].(string); ok {
					if ln, ok := payerInfo["last_name"].(string); ok {
						name = fn + " " + ln
					} else {
						name = fn
					}
				}
			}
		}

		if custom, ok := resource["custom"].(string); ok {
			productID = custom
		}

		if err := h.processSuccessfulPayment(email, name, productID, eventData); err != nil {
			log.Printf("PayPal webhook processing error: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return c.JSON(fiber.Map{"received": true})
}

func (h *WebhookHandler) processSuccessfulPayment(email, name, productIDStr string, paymentData interface{}) error {
	if email == "" || productIDStr == "" {
		log.Printf("Missing email or product ID: email=%s, productID=%s", email, productIDStr)
		return nil // Don't error out, just log and continue
	}

	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		log.Printf("Invalid product ID: %s", productIDStr)
		return nil
	}

	var product models.Product
	if err := h.db.First(&product, productID).Error; err != nil {
		log.Printf("Product not found: %d", productID)
		return nil
	}

	// Find or create customer
	customer, err := (&models.Customer{}).FindOrCreateByEmail(h.db, email, name)
	if err != nil {
		return err
	}

	// Generate license key
	licenseKey, err := product.GenerateLicenseKeyFor(h.db, customer)
	if err != nil {
		return err
	}

	// Store payment metadata
	if paymentData != nil {
		if data, err := json.Marshal(paymentData); err == nil {
			licenseKey.Metadata = string(data)
			h.db.Save(licenseKey)
		}
	}

	// Send email with license key
	if err := h.emailService.SendLicenseKey(customer.Email, licenseKey.Key, product.Name); err != nil {
		log.Printf("Failed to send license key email: %v", err)
		// Don't return error here - the license key was created successfully
	}

	log.Printf("Generated license key %s for %s", licenseKey.Key, email)
	return nil
}
