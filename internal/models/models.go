package models

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Product struct {
	ID                    uint   `gorm:"primaryKey" json:"id"`
	Name                  string `gorm:"not null" json:"name"`
	Description           string `json:"description"`
	Version               string `gorm:"default:1.0.0" json:"version"`
	DefaultExpirationDays int    `gorm:"not null;default:365" json:"default_expiration_days"`
	DefaultUsageLimit     int    `gorm:"not null;default:1" json:"default_usage_limit"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
	LicenseKeys           []LicenseKey `gorm:"foreignKey:ProductID"`
}

type Customer struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Email       string `gorm:"not null;uniqueIndex" json:"email"`
	Name        string `gorm:"not null" json:"name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Company     string `json:"company"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LicenseKeys []LicenseKey `gorm:"foreignKey:CustomerID"`
}

type LicenseKey struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	Key                string     `gorm:"not null;uniqueIndex" json:"key"`
	ProductID          uint       `gorm:"not null" json:"product_id"`
	CustomerID         uint       `gorm:"not null" json:"customer_id"`
	ExpiresAt          *time.Time `json:"expires_at"`
	MaxActivations     int        `gorm:"not null;default:1" json:"max_activations"`
	CurrentActivations int        `gorm:"not null;default:0" json:"current_activations"`
	UsageLimit         int        `gorm:"not null;default:1" json:"usage_limit"`
	UsageCount         int        `gorm:"not null;default:0" json:"usage_count"`
	Metadata           string     `json:"metadata"`
	Status             string     `gorm:"not null;default:active" json:"status"`
	IsTrial            bool       `gorm:"not null;default:false" json:"is_trial"`
	LastValidatedAt    *time.Time `json:"last_validated_at"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Product            Product  `gorm:"foreignKey:ProductID"`
	Customer           Customer `gorm:"foreignKey:CustomerID"`
}

type AdminUser struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"not null;uniqueIndex"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type EmailSettings struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Provider       string `gorm:"not null;default:smtp" json:"provider"`
	SMTPHost       string `json:"smtp_host"`
	SMTPPort       int    `json:"smtp_port"`
	SMTPUsername   string `json:"smtp_username"`
	SMTPPassword   string `json:"smtp_password"`
	SMTPEncryption string `gorm:"default:tls" json:"smtp_encryption"`
	FromEmail      string `gorm:"not null" json:"from_email"`
	FromName       string `json:"from_name"`
	IsActive       bool   `gorm:"default:false" json:"is_active"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Product methods
func (p *Product) GenerateLicenseKeyFor(db *gorm.DB, customer *Customer) (*LicenseKey, error) {
	key := generateRandomKey(32)
	expiresAt := time.Now().AddDate(0, 0, p.DefaultExpirationDays)

	licenseKey := &LicenseKey{
		Key:                key,
		ProductID:          p.ID,
		CustomerID:         customer.ID,
		ExpiresAt:          &expiresAt,
		MaxActivations:     p.DefaultUsageLimit,
		CurrentActivations: 0,
		Status:             "active",
		IsTrial:            false,
	}

	if err := db.Create(licenseKey).Error; err != nil {
		return nil, err
	}

	return licenseKey, nil
}

// Customer methods
func (c *Customer) FindOrCreateByEmail(db *gorm.DB, email, name string) (*Customer, error) {
	var customer Customer
	err := db.Where("email = ?", email).First(&customer).Error
	if err == nil {
		return &customer, nil
	}

	if name == "" {
		// Extract name from email
		name = email[:len(email)-len("@domain.com")]
	}

	customer = Customer{
		Email: email,
		Name:  name,
	}

	if err := db.Create(&customer).Error; err != nil {
		return nil, err
	}

	return &customer, nil
}

// LicenseKey methods
func (lk *LicenseKey) IsValidForUse() bool {
	return lk.Status == "active" && !lk.IsExpired() && lk.CurrentActivations < lk.MaxActivations
}

func (lk *LicenseKey) IsExpired() bool {
	return lk.ExpiresAt != nil && lk.ExpiresAt.Before(time.Now())
}

func (lk *LicenseKey) IsActive() bool {
	return lk.Status == "active"
}

func (lk *LicenseKey) IsRevoked() bool {
	return lk.Status == "revoked"
}

func (lk *LicenseKey) IncrementUsage(db *gorm.DB) error {
	if !lk.IsValidForUse() {
		return fmt.Errorf("license key is not valid for use")
	}

	lk.CurrentActivations++
	if lk.MaxActivations > 0 && lk.CurrentActivations >= lk.MaxActivations {
		lk.Status = "expired"
	}

	// Update validation timestamp
	now := time.Now()
	lk.LastValidatedAt = &now

	return db.Save(lk).Error
}

func (lk *LicenseKey) Revoke(db *gorm.DB) error {
	lk.Status = "revoked"
	return db.Save(lk).Error
}

func (lk *LicenseKey) Reactivate(db *gorm.DB) error {
	if !lk.IsExpired() {
		lk.Status = "active"
		return db.Save(lk).Error
	}
	return fmt.Errorf("cannot reactivate expired license key")
}

func (lk *LicenseKey) UsageRemaining() int {
	if lk.MaxActivations == 0 {
		return -1 // Unlimited
	}
	remaining := lk.MaxActivations - lk.CurrentActivations
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (lk *LicenseKey) ToAPIResponse() map[string]interface{} {
	return map[string]interface{}{
		"success": true,
		"purchase": map[string]interface{}{
			"seller_id":                 "self-hosted",
			"product_id":                fmt.Sprintf("%d", lk.ProductID),
			"product_name":              lk.Product.Name,
			"permalink":                 lk.Product.Name,
			"product_permalink":         fmt.Sprintf("https://localhost/products/%d", lk.ProductID),
			"email":                     lk.Customer.Email,
			"price":                     0,
			"gumroad_fee":               0,
			"currency":                  "usd",
			"quantity":                  1,
			"discover_fee_charged":      false,
			"can_contact":               true,
			"referrer":                  "direct",
			"card":                      map[string]interface{}{},
			"order_number":              lk.ID,
			"sale_id":                   fmt.Sprintf("sale_%d", lk.ID),
			"sale_timestamp":            lk.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"url":                       "",
			"variants":                  map[string]interface{}{},
			"license_key":               lk.Key,
			"ip_country":                "Unknown",
			"is_recurring_billing":      false,
			"is_preorder_authorization": false,
			"is_gift_receiver_purchase": false,
			"refunded":                  false,
			"disputed":                  false,
			"dispute_won":               false,
			"subscription_id":           nil,
			"cancelled":                 lk.IsRevoked(),
			"ended":                     !lk.IsActive(),
			"uses":                      lk.CurrentActivations,
			"test":                      true,
		},
	}
}

// AdminUser methods
func (au *AdminUser) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	au.PasswordHash = string(hash)
	return nil
}

func (au *AdminUser) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(au.PasswordHash), []byte(password))
	return err == nil
}

func CreateDefaultAdmin(db *gorm.DB, username, password string) error {
	var count int64
	db.Model(&AdminUser{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return nil // Admin already exists
	}

	admin := &AdminUser{
		Username: username,
	}
	if err := admin.SetPassword(password); err != nil {
		return err
	}

	return db.Create(admin).Error
}

// Helper functions
func generateRandomKey(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

// JSON marshaling helpers
func (lk *LicenseKey) GetMetadataMap() map[string]interface{} {
	if lk.Metadata == "" {
		return map[string]interface{}{}
	}

	var metadata map[string]interface{}
	_ = json.Unmarshal([]byte(lk.Metadata), &metadata)
	return metadata
}

func (lk *LicenseKey) SetMetadataMap(data map[string]interface{}) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	lk.Metadata = string(bytes)
	return nil
}

// EmailSettings methods
func GetActiveEmailSettings(db *gorm.DB) (*EmailSettings, error) {
	var settings EmailSettings
	err := db.Where("is_active = ?", true).First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (es *EmailSettings) Save(db *gorm.DB) error {
	if es.IsActive {
		db.Model(&EmailSettings{}).Where("id != ?", es.ID).Update("is_active", false)
	}
	return db.Save(es).Error
}

func (es *EmailSettings) Activate(db *gorm.DB) error {
	tx := db.Begin()

	if err := tx.Model(&EmailSettings{}).Where("id != ?", es.ID).Update("is_active", false).Error; err != nil {
		tx.Rollback()
		return err
	}

	es.IsActive = true
	if err := tx.Save(es).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
