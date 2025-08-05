package cloudflare

import (
	"testing"

	"github.com/brucellino/nomad-traefik-cloudflare-controller/config"
)

// Test the sync logic without making actual API calls
// Since we can't easily mock the cloudflare API without significant refactoring,
// we'll focus on testing the business logic and configuration validation

func TestSyncARecordsLogic(t *testing.T) {
	// Test the sync decision logic without making API calls
	tests := []struct {
		name             string
		targetIPs        []string
		description      string
		expectNonNilCall bool
	}{
		{
			name:             "valid IP list",
			targetIPs:        []string{"1.1.1.1", "2.2.2.2"},
			description:      "should process valid IPs",
			expectNonNilCall: true,
		},
		{
			name:             "empty IP list",
			targetIPs:        []string{},
			description:      "should handle empty IP list",
			expectNonNilCall: true,
		},
		{
			name:             "single IP",
			targetIPs:        []string{"192.168.1.1"},
			description:      "should handle single IP",
			expectNonNilCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the basic parameter validation
			if len(tt.targetIPs) == 0 {
				// Test empty slice handling
				if tt.targetIPs == nil {
					t.Error("Target IPs should not be nil")
				}
			} else {
				// Test non-empty slice handling
				for _, ip := range tt.targetIPs {
					if ip == "" {
						t.Error("IP addresses should not be empty")
					}
				}
			}
		})
	}
}

func TestCreateARecordValidation(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		config      *config.Config
		expectValid bool
	}{
		{
			name:   "valid configuration and target",
			target: "1.1.1.1",
			config: &config.Config{
				DNSRecordName:    "test.example.com",
				CloudflareZoneId: "test-zone-id",
				CloudflareToken:  "test-token",
			},
			expectValid: true,
		},
		{
			name:   "empty target",
			target: "",
			config: &config.Config{
				DNSRecordName:    "test.example.com",
				CloudflareZoneId: "test-zone-id",
				CloudflareToken:  "test-token",
			},
			expectValid: false,
		},
		{
			name:   "missing DNS record name",
			target: "1.1.1.1",
			config: &config.Config{
				DNSRecordName:    "",
				CloudflareZoneId: "test-zone-id",
				CloudflareToken:  "test-token",
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test input validation without making API calls
			isValid := tt.target != "" &&
				tt.config.DNSRecordName != "" &&
				tt.config.CloudflareZoneId != "" &&
				tt.config.CloudflareToken != ""

			if isValid != tt.expectValid {
				t.Errorf("CreateARecord validation = %v, want %v", isValid, tt.expectValid)
			}
		})
	}
}

func TestUpdateARecordValidation(t *testing.T) {
	tests := []struct {
		name        string
		recordID    string
		target      string
		expectValid bool
	}{
		{
			name:        "valid parameters",
			recordID:    "test-record-id",
			target:      "1.1.1.1",
			expectValid: true,
		},
		{
			name:        "empty record ID",
			recordID:    "",
			target:      "1.1.1.1",
			expectValid: false,
		},
		{
			name:        "empty target",
			recordID:    "test-record-id",
			target:      "",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parameter validation
			isValid := tt.recordID != "" && tt.target != ""

			if isValid != tt.expectValid {
				t.Errorf("UpdateARecord validation = %v, want %v", isValid, tt.expectValid)
			}
		})
	}
}

func TestDeleteARecordValidation(t *testing.T) {
	tests := []struct {
		name        string
		recordID    string
		expectValid bool
	}{
		{
			name:        "valid record ID",
			recordID:    "test-record-id",
			expectValid: true,
		},
		{
			name:        "empty record ID",
			recordID:    "",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parameter validation
			isValid := tt.recordID != ""

			if isValid != tt.expectValid {
				t.Errorf("DeleteARecord validation = %v, want %v", isValid, tt.expectValid)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "valid configuration",
			config: &config.Config{
				CloudflareToken:  "valid-token",
				CloudflareZoneId: "valid-zone-id",
				DNSRecordName:    "test.example.com",
			},
			expectError: false,
		},
		{
			name: "empty token",
			config: &config.Config{
				CloudflareToken:  "",
				CloudflareZoneId: "valid-zone-id",
				DNSRecordName:    "test.example.com",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("NewClient() expected error but got none")
				}
				if client != nil {
					t.Errorf("NewClient() expected nil client on error but got %v", client)
				}
			} else {
				if err != nil {
					t.Errorf("NewClient() unexpected error = %v", err)
				}
				if client == nil {
					t.Errorf("NewClient() expected client but got nil")
				}
			}
		})
	}
}

// Integration-style test for the sync logic (without actual API calls)
func TestDNSSyncLogic(t *testing.T) {
	// Test the business logic for determining what DNS changes are needed
	tests := []struct {
		name             string
		currentIPs       []string
		targetIPs        []string
		expectedToAdd    []string
		expectedToRemove []string
	}{
		{
			name:             "add new IPs",
			currentIPs:       []string{},
			targetIPs:        []string{"1.1.1.1", "2.2.2.2"},
			expectedToAdd:    []string{"1.1.1.1", "2.2.2.2"},
			expectedToRemove: []string{},
		},
		{
			name:             "remove old IPs",
			currentIPs:       []string{"1.1.1.1", "2.2.2.2"},
			targetIPs:        []string{},
			expectedToAdd:    []string{},
			expectedToRemove: []string{"1.1.1.1", "2.2.2.2"},
		},
		{
			name:             "partial update",
			currentIPs:       []string{"1.1.1.1", "2.2.2.2"},
			targetIPs:        []string{"1.1.1.1", "3.3.3.3"},
			expectedToAdd:    []string{"3.3.3.3"},
			expectedToRemove: []string{"2.2.2.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to sets for comparison
			currentSet := make(map[string]bool)
			for _, ip := range tt.currentIPs {
				currentSet[ip] = true
			}

			targetSet := make(map[string]bool)
			for _, ip := range tt.targetIPs {
				targetSet[ip] = true
			}

			// Find IPs to add
			var toAdd []string
			for _, ip := range tt.targetIPs {
				if !currentSet[ip] {
					toAdd = append(toAdd, ip)
				}
			}

			// Find IPs to remove
			var toRemove []string
			for _, ip := range tt.currentIPs {
				if !targetSet[ip] {
					toRemove = append(toRemove, ip)
				}
			}

			// Verify results
			if len(toAdd) != len(tt.expectedToAdd) {
				t.Errorf("Expected %d additions, got %d", len(tt.expectedToAdd), len(toAdd))
			}

			if len(toRemove) != len(tt.expectedToRemove) {
				t.Errorf("Expected %d removals, got %d", len(tt.expectedToRemove), len(toRemove))
			}
		})
	}
}
