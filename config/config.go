package config

import (
	"fmt"
	"os"
)

// Config holds all of the configuration for the application.
type Config struct {
	// Nomad configuration
	NomadAddress string
	NomadToken   string

	// Cloudflare configuration
	CloudflareToken  string
	CloudflareZoneId string

	// Application configuration
	TraefikJobName string // Name of the Traefik job in the Nomad cluster that we are watching
	DNSRecordName  string // Name of the DNS A Record we need to create. This is the same as the "instance" variable in the Terraform module
	LogLevel       string
}

// getEnvOrDefault is a helper function to use default values for environment variables if they are not explicitly passed.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// LoadConfig is a function which loads the configuration from envirionment variables.
// The configuration is loaded into the struct created above.
func LoadConfig() (*Config, error) {
	config := &Config{
		NomadAddress:     getEnvOrDefault("NOMAD_ADDR", "http://localhost:8686"), // This could be nomad.service.consul in a service-discovery cluster.
		NomadToken:       os.Getenv("NOMAD_TOKEN"),
		CloudflareToken:  os.Getenv("CLOUDFLARE_API_TOKEN"),
		CloudflareZoneId: os.Getenv("CLOUDFLARE_ZONE_ID"),
		TraefikJobName:   os.Getenv("TRAEFIK_JOB_NAME"),
		DNSRecordName:    os.Getenv("DNS_RECORD_NAME"),
		LogLevel:         getEnvOrDefault("LOG_LEVEL", "info"),
	}

	// Check if required values are not set
	if config.CloudflareToken == "" {
		return nil, fmt.Errorf("CLOUDFLARE_API_TOKEN is not set and is required.")
	}

	if config.CloudflareZoneId == "" {
		return nil, fmt.Errorf("CLOUDFLARE_ZONE_ID is not set and is required.")
	}

	if config.NomadToken == "" {
		return nil, fmt.Errorf("Nomad token is not set and is required.")
	}

	return config, nil
}
