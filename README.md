# Matcha 🍵

An **open-source** self-hosted license key management system built with Go, GoFiber, GORM, and SQLite.

> **🎉 Open Source & Free**: This project is completely free and open-source under the MIT License. You can use it commercially, modify it, and distribute it without restrictions.

## Features

- **License Key Management**: Generate, validate, and manage license keys
- **Product Management**: Create products with configurable expiration and usage limits
- **Customer Management**: Automatic customer creation from payments
- **Webhook Integration**: Support for Stripe, Gumroad, and PayPal webhooks
- **Email Delivery**: Send license keys via Mailgun, SendGrid, or SMTP
- **Admin Interface**: Web-based administration panel
- **API**: RESTful API
- **High Performance**: Built with GoFiber for fast HTTP handling

## Quick Start with Docker

1. Clone the repository
2. Copy environment variables:

   ```bash
   cp .env.example .env
   ```

3. Configure email settings in `.env`
4. Start with Docker Compose:

   ```bash
   docker-compose up -d
   ```

5. Access the admin panel at <http://localhost:3001>
   - Username: `admin`
   - Password: `admin123`

## API Usage

### License Verification

```bash
curl -X POST http://localhost:3001/api/v1/licenses/verify \
  -d "product_id=1" \
  -d "license_key=YOUR_LICENSE_KEY" \
  -d "increment_uses_count=true"
```

### Webhooks

- **Stripe**: `POST /api/v1/webhooks/stripe`
- **Gumroad**: `POST /api/v1/webhooks/gumroad`
- **PayPal**: `POST /api/v1/webhooks/paypal`

## Development

```bash
# Install dependencies
go mod download

# Run the application
go run main.go
```

## Environment Variables

See `.env.example` for all configuration options.
