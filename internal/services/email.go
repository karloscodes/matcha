package services

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"matcha/internal/config"
	"matcha/internal/models"

	"gorm.io/gorm"
)

type EmailService struct {
	config *config.Config
	db     *gorm.DB
}

func NewEmailService(cfg *config.Config, db *gorm.DB) *EmailService {
	return &EmailService{
		config: cfg,
		db:     db,
	}
}

func (es *EmailService) SendTestEmail(toEmail string) error {
	settings, err := models.GetActiveEmailSettings(es.db)
	if err != nil {
		return fmt.Errorf("no active email settings found: %w", err)
	}

	subject := "Test Email from Matcha"
	body := `
<html>
<body>
	<h2>Test Email</h2>
	<p>This is a test email to verify your email configuration is working correctly.</p>
	<p>If you received this email, your SMTP settings are properly configured.</p>
</body>
</html>`

	return es.sendEmail(settings, toEmail, subject, body)
}

func (es *EmailService) SendLicenseKey(toEmail, licenseKey, productName string) error {
	settings, err := models.GetActiveEmailSettings(es.db)
	if err != nil {
		return fmt.Errorf("no active email settings found: %w", err)
	}

	subject := fmt.Sprintf("Your License Key for %s", productName)
	body := fmt.Sprintf(`
<html>
<body>
	<h2>Your License Key</h2>
	<p>Thank you for your purchase! Here are your license details:</p>
	
	<div style="background-color: #f5f5f5; padding: 20px; margin: 20px 0; border-radius: 5px;">
		<h3>Product: %s</h3>
		<p><strong>License Key:</strong> <code style="background-color: #e8e8e8; padding: 4px 8px; border-radius: 3px;">%s</code></p>
	</div>
	
	<p>Please keep this license key safe and secure. You'll need it to activate your software.</p>
	
	<p>If you have any questions or need support, please don't hesitate to contact us.</p>
	
	<p>Best regards,<br>
	The Matcha Team</p>
</body>
</html>`, productName, licenseKey)

	return es.sendEmail(settings, toEmail, subject, body)
}

func (es *EmailService) sendEmail(settings *models.EmailSettings, to, subject, body string) error {
	if settings.Provider != "smtp" {
		return fmt.Errorf("unsupported email provider: %s", settings.Provider)
	}

	auth := smtp.PlainAuth("", settings.SMTPUsername, settings.SMTPPassword, settings.SMTPHost)

	fromName := settings.FromName
	if fromName == "" {
		fromName = "Matcha"
	}

	msg := []string{
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("From: %s <%s>", fromName, settings.FromEmail),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"",
		body,
	}

	message := []byte(strings.Join(msg, "\r\n"))

	addr := fmt.Sprintf("%s:%d", settings.SMTPHost, settings.SMTPPort)

	switch settings.SMTPEncryption {
	case "tls", "starttls":
		return es.sendWithTLS(addr, auth, settings.FromEmail, []string{to}, message)
	case "ssl":
		return es.sendWithSSL(addr, auth, settings.FromEmail, []string{to}, message)
	default:
		return smtp.SendMail(addr, auth, settings.FromEmail, []string{to}, message)
	}
}

func (es *EmailService) sendWithTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if err = client.StartTLS(&tls.Config{ServerName: strings.Split(addr, ":")[0]}); err != nil {
		return err
	}

	if err = client.Auth(auth); err != nil {
		return err
	}

	if err = client.Mail(from); err != nil {
		return err
	}

	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()

	_, err = writer.Write(msg)
	return err
}

func (es *EmailService) sendWithSSL(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         strings.Split(addr, ":")[0],
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client, err := smtp.NewClient(conn, strings.Split(addr, ":")[0])
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if err = client.Auth(auth); err != nil {
		return err
	}

	if err = client.Mail(from); err != nil {
		return err
	}

	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()

	_, err = writer.Write(msg)
	return err
}

// Legacy compatibility functions for existing config-based approach
func NewEmailServiceWithConfig(cfg *config.Config) *EmailService {
	return &EmailService{
		config: cfg,
	}
}

func (es *EmailService) SendTestEmailLegacy(toEmail string) error {
	// This is a stub for backward compatibility
	// In a real implementation, you'd fall back to environment variables
	// or provide a migration path
	return fmt.Errorf("please configure email settings in the database")
}

// Helper function to migrate from config to database
func (es *EmailService) MigrateConfigToDatabase() error {
	if es.db == nil {
		return fmt.Errorf("database connection required for migration")
	}

	// Check if we already have active settings
	_, err := models.GetActiveEmailSettings(es.db)
	if err == nil {
		return nil // Already have settings
	}

	// Create default settings from environment (you'd read from env vars here)
	settings := &models.EmailSettings{
		Provider:       "smtp",
		SMTPHost:       "smtp.gmail.com", // Default or from env
		SMTPPort:       587,
		SMTPUsername:   "",
		SMTPPassword:   "",
		SMTPEncryption: "tls",
		FromEmail:      "",
		FromName:       "Matcha",
		IsActive:       false, // Require manual activation
	}

	return es.db.Create(settings).Error
}
