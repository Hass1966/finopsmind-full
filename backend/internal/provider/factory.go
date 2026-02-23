package provider

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/finopsmind/backend/internal/crypto"
	"github.com/finopsmind/backend/internal/model"
)

// NewProviderFromEncryptedCreds creates a Provider by decrypting stored credentials
// and instantiating the appropriate provider type.
func NewProviderFromEncryptedCreds(providerType string, encryptedCreds []byte, encryptionKey string, logger *slog.Logger) (Provider, error) {
	key := crypto.DeriveKey(encryptionKey)
	plaintext, err := crypto.Decrypt(encryptedCreds, key)
	if err != nil {
		return nil, fmt.Errorf("factory: failed to decrypt credentials: %w", err)
	}

	switch model.CloudProvider(providerType) {
	case model.CloudProviderAWS:
		var creds model.AWSCredentials
		if err := json.Unmarshal(plaintext, &creds); err != nil {
			return nil, fmt.Errorf("factory: failed to unmarshal AWS credentials: %w", err)
		}
		if AwsFromCredsFunc == nil {
			return nil, fmt.Errorf("factory: AWS provider constructor not registered")
		}
		return AwsFromCredsFunc(creds, logger)

	case model.CloudProviderAzure:
		var creds model.AzureCredentials
		if err := json.Unmarshal(plaintext, &creds); err != nil {
			return nil, fmt.Errorf("factory: failed to unmarshal Azure credentials: %w", err)
		}
		if AzureFromCredsFunc == nil {
			return nil, fmt.Errorf("factory: Azure provider constructor not registered")
		}
		return AzureFromCredsFunc(creds, logger)

	default:
		return nil, fmt.Errorf("factory: unsupported provider type: %s", providerType)
	}
}

// EncryptCredentials serializes and encrypts credentials for storage.
func EncryptCredentials(creds any, encryptionKey string) ([]byte, error) {
	plaintext, err := json.Marshal(creds)
	if err != nil {
		return nil, fmt.Errorf("factory: failed to marshal credentials: %w", err)
	}
	key := crypto.DeriveKey(encryptionKey)
	return crypto.Encrypt(plaintext, key)
}

// AwsFromCredsFunc is set by the aws package init or startup to avoid circular imports.
var AwsFromCredsFunc func(creds model.AWSCredentials, logger *slog.Logger) (Provider, error)

// AzureFromCredsFunc is set by the azure package init or startup to avoid circular imports.
var AzureFromCredsFunc func(creds model.AzureCredentials, logger *slog.Logger) (Provider, error)
