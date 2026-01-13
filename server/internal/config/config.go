package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the Octopus server
type Config struct {
	DatabasePath string `yaml:"database_path"`

	// AD Authentication
	ADServer   string `yaml:"ad_server"`
	ADBaseDN   string `yaml:"ad_base_dn"`
	ADBindUser string `yaml:"ad_bind_user"`
	ADBindPass string `yaml:"ad_bind_pass"`
	ADDomain   string `yaml:"ad_domain"`

	// JWT Settings
	JWTSecret     string `yaml:"jwt_secret"`
	JWTExpiration int    `yaml:"jwt_expiration_hours"`

	// Session Settings
	SessionKey string `yaml:"session_key"`

	// VMware Settings
	VMwareDefaults VMwareConfig `yaml:"vmware_defaults"`

	// Cloud Provider Settings
	AWSDefaults   AWSConfig   `yaml:"aws_defaults"`
	GCPDefaults   GCPConfig   `yaml:"gcp_defaults"`
	AzureDefaults AzureConfig `yaml:"azure_defaults"`
}

// VMwareConfig holds VMware vCenter configuration
type VMwareConfig struct {
	Host       string `yaml:"host"`
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	Datacenter string `yaml:"datacenter"`
	Insecure   bool   `yaml:"insecure"`
}

// AWSConfig holds AWS configuration
type AWSConfig struct {
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
}

// GCPConfig holds GCP configuration
type GCPConfig struct {
	ProjectID       string `yaml:"project_id"`
	Zone            string `yaml:"zone"`
	CredentialsFile string `yaml:"credentials_file"`
}

// AzureConfig holds Azure configuration
type AzureConfig struct {
	SubscriptionID string `yaml:"subscription_id"`
	ResourceGroup  string `yaml:"resource_group"`
	TenantID       string `yaml:"tenant_id"`
	ClientID       string `yaml:"client_id"`
	ClientSecret   string `yaml:"client_secret"`
}

// Load reads configuration from file or environment
func Load() (*Config, error) {
	cfg := &Config{
		DatabasePath:  getEnv("DATABASE_PATH", "/data/octopus.db"),
		ADServer:      getEnv("AD_SERVER", ""),
		ADBaseDN:      getEnv("AD_BASE_DN", ""),
		ADBindUser:    getEnv("AD_BIND_USER", ""),
		ADBindPass:    getEnv("AD_BIND_PASS", ""),
		ADDomain:      getEnv("AD_DOMAIN", ""),
		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiration: 24,
		SessionKey:    getEnv("SESSION_KEY", "change-me-in-production-too"),
	}

	// Try to load from config file if it exists
	configPath := getEnv("CONFIG_PATH", "/etc/octopus/config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
