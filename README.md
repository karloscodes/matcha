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

## License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

### Why Open Source?

Matcha is open-source because we believe:

- 🔓 **Transparency**: You should know exactly how your license management works
- 🚀 **Community**: Open-source projects improve faster with community contributions
- 💰 **Cost-Effective**: No licensing fees - use it for free, forever
- 🔧 **Customizable**: Modify it to fit your specific needs
- 🤝 **Trust**: Open code builds trust with your customers

## Contributing

We welcome contributions! Here's how you can help:

1. 🐛 **Report Bugs**: [Create an issue](https://github.com/yourusername/matcha/issues)
2. 💡 **Suggest Features**: [Start a discussion](https://github.com/yourusername/matcha/discussions)
3. 🛠️ **Submit Code**: Fork, code, and create a pull request
4. 📖 **Improve Docs**: Help make the documentation better
5. ⭐ **Star the Repo**: Show your support!

### Development Guidelines

- Follow Go best practices and conventions
- Add tests for new features
- Update documentation when needed
- Keep commits clean and descriptive

## Production Deployment

> ⚠️ **Note**: While functional, this project is still in active development. Test thoroughly before production use.

Deployment guides coming soon...

