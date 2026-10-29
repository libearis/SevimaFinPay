package institusi

import (
	"time"

	"github.com/google/uuid"
)

// Institution represents a university/campus tenant in the multi-institution database.
type Institution struct {
	ID               uuid.UUID `json:"id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	HMACSecretEnvKey string    `json:"-"` // Never expose env key name in public JSON responses
	CreatedAt        time.Time `json:"created_at"`
}
