# License Key Manager - Go Version

A self-hosted license key management system similar to Gumroad's license functionality, built with Go, GoFiber, GORM, and SQLite.

## Features

- **License Key Management**: Generate, validate, and manage license keys
- **Product Management**: Create products with configurable expiration and usage limits
- **Customer Management**: Automatic customer creation from payments
- **Webhook Integration**: Support for Stripe, Gumroad, and PayPal webhooks
- **Email Delivery**: Send license keys via Mailgun, SendGrid, or SMTP
- **Admin Interface**: Web-based administration panel
- **API**: RESTful API compatible with Gumroad's license verification
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
5. Access the admin panel at http://localhost:3001
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

## Database Schema

- **Products**: Store product information and default settings
- **Customers**: Customer information from payments
- **License Keys**: Generated keys with usage tracking
- **Admin Users**: Admin authentication

## Tech Stack

- **Framework**: GoFiber (Express.js-like for Go)
- **Database**: SQLite with GORM
- **Templates**: HTML templates
- **Email**: Mailgun, SendGrid, or SMTP support
- **Authentication**: Secure sessions
- **Rate Limiting**: Built-in rate limiting

## Production Deployment

1. Set production environment variables
2. Configure email service
3. Set secure `SECRET_KEY`
4. Use Docker or build binary for your platform
5. Configure reverse proxy (nginx/caddy) if needed

## Performance

The Go version typically offers:
- Faster startup times
- Lower memory usage
- Higher throughput for API requests
- Better concurrent request handling