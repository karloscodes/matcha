package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"license-key-manager/internal/config"
	"license-key-manager/internal/models"

	"github.com/mailgun/mailgun-go/v4"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"gopkg.in/gomail.v2"
)

type EmailService struct {
	config *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		config: cfg,
	}
}

func (es *EmailService) SendLicenseKey(licenseKey *models.LicenseKey) error {
	subject := fmt.Sprintf("Your License Key for %s", licenseKey.Product.Name)
	htmlBody, err := es.generateLicenseKeyEmail(licenseKey)
	if err != nil {
		return err
	}

	switch strings.ToLower(es.config.EmailService) {
	case "mailgun":
		return es.sendViaMailgun(licenseKey.Customer.Email, subject, htmlBody)
	case "sendgrid":
		return es.sendViaSendGrid(licenseKey.Customer.Email, subject, htmlBody)
	case "smtp":
		return es.sendViaSMTP(licenseKey.Customer.Email, subject, htmlBody)
	default:
		return fmt.Errorf("unsupported email service: %s", es.config.EmailService)
	}
}

func (es *EmailService) sendViaMailgun(to, subject, htmlBody string) error {
	if es.config.MailgunAPIKey == "" || es.config.MailgunDomain == "" {
		return fmt.Errorf("mailgun configuration missing")
	}

	mg := mailgun.NewMailgun(es.config.MailgunDomain, es.config.MailgunAPIKey)
	message := mg.NewMessage(es.config.FromEmail, subject, "", to)
	message.SetHtml(htmlBody)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, _, err := mg.Send(ctx, message)
	return err
}

func (es *EmailService) sendViaSendGrid(to, subject, htmlBody string) error {
	if es.config.SendGridAPIKey == "" {
		return fmt.Errorf("sendgrid configuration missing")
	}

	from := mail.NewEmail("License Key Manager", es.config.FromEmail)
	toEmail := mail.NewEmail("", to)
	message := mail.NewSingleEmail(from, subject, toEmail, "", htmlBody)

	client := sendgrid.NewSendClient(es.config.SendGridAPIKey)
	_, err := client.Send(message)
	return err
}

func (es *EmailService) sendViaSMTP(to, subject, htmlBody string) error {
	if es.config.SMTPServer == "" {
		return fmt.Errorf("smtp configuration missing")
	}

	m := gomail.NewMessage()
	m.SetHeader("From", es.config.FromEmail)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	port, _ := strconv.Atoi(es.config.SMTPPort)
	d := gomail.NewDialer(es.config.SMTPServer, port, es.config.SMTPUsername, es.config.SMTPPassword)

	if es.config.SMTPTLS == "false" {
		d.TLSConfig = nil
	}

	return d.DialAndSend(m)
}

func (es *EmailService) generateLicenseKeyEmail(licenseKey *models.LicenseKey) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .key { font-family: monospace; font-size: 18px; background: #f4f4f4; padding: 10px; border-radius: 4px; }
        .details { background: #f9f9f9; padding: 15px; border-radius: 4px; margin: 15px 0; }
    </style>
</head>
<body>
    <h2>Your License Key for {{.Product.Name}}</h2>
    
    <p>Hello {{.Customer.Name}},</p>
    
    <p>Thank you for your purchase! Here is your license key:</p>
    
    <div class="key">
        <strong>{{.Key}}</strong>
    </div>
    
    <div class="details">
        <h3>License Details</h3>
        <ul>
            <li><strong>Product:</strong> {{.Product.Name}}</li>
            <li><strong>Usage Limit:</strong> {{.UsageLimit}} activations</li>
            <li><strong>Expires:</strong> {{.ExpiresAt.Format "January 2, 2006"}}</li>
            <li><strong>Current Usage:</strong> {{.UsageCount}} / {{.UsageLimit}}</li>
        </ul>
    </div>
    
    <p>Please keep this email safe as you will need the license key to use your software.</p>
    
    <hr>
    <p><small>This email was sent automatically. Please do not reply to this email.</small></p>
</body>
</html>`

	t, err := template.New("license_key_email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, licenseKey); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (es *EmailService) SendTestEmail(to string) error {
	subject := "Test Email from License Key Manager"
	htmlBody := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #3b82f6; color: white; padding: 20px; border-radius: 8px 8px 0 0; }
        .content { background: #f9f9f9; padding: 20px; border-radius: 0 0 8px 8px; }
        .success { color: #10b981; font-weight: bold; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>Email Configuration Test</h2>
        </div>
        <div class="content">
            <p>Hello!</p>
            <p>This is a test email to verify that your License Key Manager email configuration is working correctly.</p>
            <p class="success">✅ Email delivery is functioning properly!</p>
            <p>If you received this email, your email service configuration is set up correctly and ready to send license keys to your customers.</p>
            <hr style="margin: 20px 0; border: none; border-top: 1px solid #ddd;">
            <p><small>This is an automated test email from License Key Manager. Please do not reply to this email.</small></p>
        </div>
    </div>
</body>
</html>`

	switch strings.ToLower(es.config.EmailService) {
	case "mailgun":
		return es.sendViaMailgun(to, subject, htmlBody)
	case "sendgrid":
		return es.sendViaSendGrid(to, subject, htmlBody)
	case "smtp":
		return es.sendViaSMTP(to, subject, htmlBody)
	default:
		return fmt.Errorf("unsupported email service: %s", es.config.EmailService)
	}
}
