package remediation

import "time"

// WaitTimeout is the maximum time to wait for instance state changes.
const WaitTimeout = 5 * time.Minute

// AWSCreds holds decrypted AWS credentials for remediation actions.
type AWSCreds struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty"`
	Region          string `json:"region"`
}

// AzureCreds holds decrypted Azure credentials for remediation actions.
type AzureCreds struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	SubscriptionID string `json:"subscription_id"`
}
