# Build stage
FROM node:18-alpine AS css-builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install Node dependencies
RUN npm ci --only=production

# Copy source files needed for CSS build
COPY . .

# Build CSS
RUN npm run build-css-prod

# Go build stage
FROM golang:1.21-alpine AS go-builder

WORKDIR /app

# Install git (needed for go modules)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download go dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy built CSS from previous stage
COPY --from=css-builder /app/static ./static

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -a -installsuffix cgo -o matcha main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder
COPY --from=go-builder /app/matcha .

# Copy templates and static files
COPY --from=go-builder /app/templates ./templates
COPY --from=go-builder /app/static ./static

# Create database directory
RUN mkdir -p /root/db

# Expose port
EXPOSE 8080

# Set environment variables
ENV GO_ENV=production
ENV PORT=8080

# Run the binary
CMD ["./matcha"]