// Package testdata provides test fixtures for schema generation.
package testdata

import "time"

// Custom type aliases for testing
type UserID string
type Status string
type Counter int
type Milliseconds int64
type Percentage float64

// +schema:inline
// Inline Version of User
type InlineUser struct {
	// Unique identifier
	ID string `json:"id" validate:"required,uuid"`
	// User's email address
	Email string `json:"email" validate:"required,email"`
	// Age in years
	Age int `json:"age" validate:"gte=0,lte=150"`
	// User's display name
	Name string `json:"name" validate:"required,min=1,max=100"`
	// User's address
	Address Address `json:"address"`
}

// +schema
// User represents a system user
type User struct {
	// Unique identifier
	ID string `json:"id" validate:"required,uuid"`
	// User's email address
	Email string `json:"email" validate:"required,email"`
	// Age in years
	Age int `json:"age" validate:"gte=0,lte=150"`
	// User's display name
	Name string `json:"name" validate:"required,min=1,max=100"`
	// User's address
	Address Address `json:"address"`
	// List of roles
	Roles []string `json:"roles" validate:"dive,oneof=admin user guest"`
	// Account creation time
	CreatedAt time.Time `json:"created_at"`
	// Optional metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Address represents a physical address
type Address struct {
	// Street address
	Street string `json:"street" validate:"required"`
	// City name
	City string `json:"city" validate:"required"`
	// ZIP or postal code
	ZipCode string `json:"zip_code" validate:"required,numeric,len=5"`
	// Country code
	Country string `json:"country" validate:"required,len=2,uppercase"`
}

// Product represents a product in the catalog
type Product struct {
	// Product SKU
	SKU string `json:"sku" validate:"required,alphanum,min=3,max=20"`
	// Product name
	Name string `json:"name" validate:"required"`
	// Price in cents
	Price int64 `json:"price" validate:"required,gt=0"`
	// Discount percentage
	Discount float64 `json:"discount" validate:"gte=0,lte=100"`
	// Product URL
	URL string `json:"url" validate:"url"`
	// Product tags
	Tags []string `json:"tags,omitempty"`
}

// +schema
// ServiceConfig demonstrates custom types and time.Duration support
type ServiceConfig struct {
	// Service identifier using custom type
	ID UserID `json:"id" validate:"required"`
	// Service status using custom enum type
	Status Status `json:"status" validate:"required,oneof=active inactive pending"`
	// Request timeout duration
	Timeout time.Duration `json:"timeout"`
	// Retry delay duration
	RetryDelay time.Duration `json:"retry_delay,omitempty"`
	// Maximum retry count
	MaxRetries Counter `json:"max_retries" validate:"gte=0,lte=10"`
	// Delay in milliseconds
	DelayMs Milliseconds `json:"delay_ms"`
	// Success rate percentage
	SuccessRate Percentage `json:"success_rate" validate:"gte=0,lte=100"`
	// List of allowed statuses
	AllowedStatuses []Status `json:"allowed_statuses,omitempty"`
	// Map of timeouts by operation
	OperationTimeouts map[string]time.Duration `json:"operation_timeouts,omitempty"`
	// Custom external type with schema override
	CustomData interface{} `json:"custom_data,omitempty" schema:"type=object"`
}
