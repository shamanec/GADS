package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SecretStoreInterface defines the interface for secret key storage
type SecretStoreInterface interface {
	AddSecretKey(secretKey *SecretKey, username, justification string) error
	GetSecretKeyByOrigin(origin string) (*SecretKey, error)
	GetDefaultSecretKey() (*SecretKey, error)
	GetAllSecretKeys() ([]*SecretKey, error)
	GetSecretKeyByID(id primitive.ObjectID) (*SecretKey, error)
	UpdateSecretKey(secretKey *SecretKey, username, justification string) error
	DisableSecretKey(id primitive.ObjectID, username, justification string) error
}

// SecretCache provides an in-memory cache for secret keys
type SecretCache struct {
	store        SecretStoreInterface
	keys         map[string][]byte // Map of origin to key
	defaultKey   []byte            // Default key
	mutex        sync.RWMutex      // To protect concurrent access
	lastRefresh  time.Time         // Time of last cache refresh
	refreshEvery time.Duration     // How often to refresh the cache
}

// NewSecretCache creates a new secret key cache
func NewSecretCache(store SecretStoreInterface, refreshInterval time.Duration) *SecretCache {
	cache := &SecretCache{
		store:        store,
		keys:         make(map[string][]byte),
		refreshEvery: refreshInterval,
	}

	// Initialize the cache
	cache.Refresh()

	return cache
}

// Refresh updates the cache with the latest keys from the database
func (c *SecretCache) Refresh() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Get all active secret keys
	secretKeys, err := c.store.GetAllSecretKeys()
	if err != nil {
		return err
	}

	// If there are no keys at all, create a default one
	if len(secretKeys) == 0 {
		// Generate a random key for security
		randomKey, err := generateRandomKey(32) // 32 bytes for good security
		if err != nil {
			return err
		}

		// Create a new default key
		defaultKey := &SecretKey{
			Origin:              "default",
			Key:                 randomKey,
			IsDefault:           true,
			UserIdentifierClaim: "username",
		}

		// Use empty values to not associate with a specific user
		err = c.store.AddSecretKey(defaultKey, "system", "Auto-generated default key")
		if err != nil {
			return err
		}

		// Get the keys again after adding the default
		secretKeys, err = c.store.GetAllSecretKeys()
		if err != nil {
			return err
		}
	}

	// Check if there's a default key among the existing keys
	hasDefault := false
	for _, key := range secretKeys {
		if key.IsDefault {
			hasDefault = true
			break
		}
	}

	// If no default key exists, mark the first one as default
	if !hasDefault && len(secretKeys) > 0 {
		secretKeys[0].IsDefault = true
		// Use empty values to not associate with a specific user
		err = c.store.UpdateSecretKey(secretKeys[0], "system", "Auto-marked as default key")
		if err != nil {
			return err
		}
		// Get the keys again after updating
		secretKeys, err = c.store.GetAllSecretKeys()
		if err != nil {
			return err
		}
	}

	// Clear the current cache
	c.keys = make(map[string][]byte)
	c.defaultKey = nil

	// Populate the cache with the fresh data
	for _, secretKey := range secretKeys {
		c.keys[secretKey.Origin] = []byte(secretKey.Key)

		// Set default key if this is the default
		if secretKey.IsDefault {
			c.defaultKey = []byte(secretKey.Key)
		}
	}

	// Update the refresh timestamp
	c.lastRefresh = time.Now()

	return nil
}

// GetKey returns the secret key for a given origin
// If the origin doesn't have a specific key, returns the default key
func (c *SecretCache) GetKey(origin string) []byte {
	// Check if refresh is needed
	if time.Since(c.lastRefresh) > c.refreshEvery {
		// Non-blocking refresh attempt
		go c.Refresh()
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Check if we have a key for this origin
	if key, exists := c.keys[origin]; exists {
		return key
	}

	// Otherwise return the default key
	return c.defaultKey
}

// GetDefaultKey returns the default secret key
func (c *SecretCache) GetDefaultKey() []byte {
	// Check if refresh is needed
	if time.Since(c.lastRefresh) > c.refreshEvery {
		// Non-blocking refresh attempt
		go c.Refresh()
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.defaultKey
}

// generateRandomKey creates a secure random key with the specified number of bytes
func generateRandomKey(bytes int) (string, error) {
	b := make([]byte, bytes)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
