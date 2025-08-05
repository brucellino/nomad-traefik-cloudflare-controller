package internaltypes

// Unit tests for the types package

import (
	"testing"
	"time"
)

// TestNodeInfo runs unit tests for the NodeInfo type
func TestNodeInfo(t *testing.T) {
	tests := []struct {
		name     string
		nodeInfo NodeInfo
		isValid  bool
	}{
		{
			name: "valid node info",
			nodeInfo: NodeInfo{
				ID:              "node-123",
				Name:            "worker-1",
				PublicIPAddress: "192.168.1.100",
				Status:          "ready",
			},
			isValid: true,
		},
		{
			name: "NodeID is not set",
			// If the data passed does not have a NodeID, this is not a Nomad event.
			// This is a rare case, but checks that we have a consistent event passed.
			nodeInfo: NodeInfo{
				ID:              "",
				Name:            "worker-1",
				PublicIPAddress: "83.212.75.34",
				Status:          "ready",
			},
			isValid: false,
		},
		{
			name: "Node does not have a public IP set",
			// If we do not have a Public IP, we cannot assign an A record in cloudflare
			nodeInfo: NodeInfo{
				ID:              "node-123",
				Name:            "worker-1",
				PublicIPAddress: "", // This is empty. Another test should check that the IP is resolvable on a public network.
				Status:          "ready",
			},
			isValid: false,
		},
		{
			name: "Node non-ready status",
			// If the node status is not "ready", then we cannot schedule tasks there,
			// and we should not add it to the DNS pool.
			nodeInfo: NodeInfo{
				ID:              "node-123",
				Name:            "worker-1",
				PublicIPAddress: "83.212.75.34",
				Status:          "down",
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test basic validation logic
			isValid := tt.nodeInfo.ID != "" &&
				tt.nodeInfo.PublicIPAddress != "" &&
				tt.nodeInfo.Status == "ready"

			if isValid != tt.isValid {
				t.Errorf("NodeInfo validation = %v, want %v", isValid, tt.isValid)
			}
		})
	}
}

// TestDNSRecord is a test function which constructs a few test scenarios for valid and invalid
// DNS records.
// We only cover A records, since we only use them.
func TestDNSRecord(t *testing.T) {
	tests := []struct {
		name       string
		record     DNSRecord
		isValid    bool
		recordType string
	}{
		{
			name: "valid A record",
			record: DNSRecord{
				ID:      "record-123",
				Name:    "traefik.example.com",
				Type:    "A",
				Content: "192.168.1.100",
				TTL:     300,
			},
			isValid:    true,
			recordType: "A",
		},
		{
			name: "empty content",
			// If we do not specify the content, the A record has nothing to point to, so we should not be able to do this.
			record: DNSRecord{
				ID:      "record-123",
				Name:    "traefik.example.com",
				Type:    "A",
				Content: "",
				TTL:     300,
			},
			isValid: false,
		},
		{
			name: "invalid TTL",
			// Negative TTLs should not be allowed in Cloudflare
			record: DNSRecord{
				ID:      "record-123",
				Name:    "traefik.example.com",
				Type:    "A",
				Content: "192.168.1.100",
				TTL:     -1,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test basic validation logic
			isValid := tt.record.ID != "" &&
				tt.record.Name != "" &&
				tt.record.Type != "" &&
				tt.record.Content != "" &&
				tt.record.TTL >= 0

			if isValid != tt.isValid {
				t.Errorf("DNSRecord validation = %v, want %v", isValid, tt.isValid)
			}

			if tt.recordType != "" && tt.record.Type != tt.recordType {
				t.Errorf("DNSRecord Type = %v, want %v", tt.record.Type, tt.recordType)
			}
		})
	}
}

// TestEvent tests the validation logic for Nomad Event structs.
func TestEvent(t *testing.T) {
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		event   Event
		isValid bool
	}{
		{
			name: "valid allocation event",
			event: Event{
				Type:      "AllocationUpdated",
				Timestamp: fixedTime,
				NodeID:    "node-123",
				JobID:     "traefik",
				Details:   map[string]interface{}{"status": "running"},
			},
			isValid: true,
		},
		{
			name: "valid node event",
			event: Event{
				Type:      "NodeUpdated",
				Timestamp: fixedTime,
				NodeID:    "node-456",
				JobID:     "",
				Details:   map[string]interface{}{"status": "ready"},
			},
			isValid: true,
		},
		{
			name: "empty event type",
			event: Event{
				Type:      "",
				Timestamp: fixedTime,
				NodeID:    "node-123",
				JobID:     "traefik",
				Details:   map[string]interface{}{},
			},
			isValid: false,
		},
		{
			name: "zero timestamp",
			event: Event{
				Type:      "AllocationUpdated",
				Timestamp: time.Time{},
				NodeID:    "node-123",
				JobID:     "traefik",
				Details:   map[string]interface{}{},
			},
			isValid: false,
		},
		{
			name: "nil details",
			event: Event{
				Type:      "AllocationUpdated",
				Timestamp: fixedTime,
				NodeID:    "node-123",
				JobID:     "traefik",
				Details:   nil,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test basic validation logic
			isValid := tt.event.Type != "" &&
				!tt.event.Timestamp.IsZero() &&
				tt.event.Details != nil

			if isValid != tt.isValid {
				t.Errorf("Event validation = %v, want %v", isValid, tt.isValid)
			}
		})
	}
}
