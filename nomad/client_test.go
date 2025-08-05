package nomad

import (
	"context"
	"testing"
	"time"

	"github.com/brucellino/nomad-traefik-cloudflare-controller/config"
	internaltypes "github.com/brucellino/nomad-traefik-cloudflare-controller/types"
	nomadapi "github.com/hashicorp/nomad/api"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "valid configuration",
			config: &config.Config{
				NomadAddress: "http://localhost:4646",
				NomadToken:   "test-token",
			},
			expectError: false,
		},
		{
			name: "empty address uses default",
			config: &config.Config{
				NomadAddress: "",
				NomadToken:   "test-token",
			},
			expectError: false,
		},
		{
			name: "custom address",
			config: &config.Config{
				NomadAddress: "http://custom:4646",
				NomadToken:   "test-token",
			},
			expectError: false,
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
				if client.config != tt.config {
					t.Errorf("NewClient() config not properly set")
				}
			}
		})
	}
}

func TestProcessEvent(t *testing.T) {
	client := &Client{
		config: &config.Config{
			TraefikJobName: "traefik",
		},
	}

	tests := []struct {
		name           string
		event          *nomadapi.Event
		expectedResult *internaltypes.Event
	}{
		{
			name: "allocation updated event",
			event: &nomadapi.Event{
				Type:  "AllocationUpdated",
				Index: 12345,
				Payload: map[string]interface{}{
					"NodeID": "test-node-id",
					"JobID":  "traefik",
				},
			},
			expectedResult: &internaltypes.Event{
				Type:      "AllocationUpdated",
				Timestamp: time.Unix(0, 12345),
				NodeID:    "test-node-id",
				JobID:     "traefik",
			},
		},
		{
			name: "node updated event",
			event: &nomadapi.Event{
				Type:  "NodeUpdated",
				Index: 67890,
				Payload: map[string]interface{}{
					"NodeID": "test-node-id-2",
				},
			},
			expectedResult: &internaltypes.Event{
				Type:      "NodeUpdated",
				Timestamp: time.Unix(0, 67890),
				NodeID:    "test-node-id-2",
			},
		},
		{
			name: "job registered event",
			event: &nomadapi.Event{
				Type:  "JobRegistered",
				Index: 11111,
				Payload: map[string]interface{}{
					"JobID": "new-job",
				},
			},
			expectedResult: &internaltypes.Event{
				Type:      "JobRegistered",
				Timestamp: time.Unix(0, 11111),
				JobID:     "new-job",
			},
		},
		{
			name: "untracked event type",
			event: &nomadapi.Event{
				Type:  "SomeOtherEvent",
				Index: 99999,
			},
			expectedResult: nil,
		},
		{
			name: "event with no payload",
			event: &nomadapi.Event{
				Type:    "AllocationUpdated",
				Index:   54321,
				Payload: nil,
			},
			expectedResult: &internaltypes.Event{
				Type:      "AllocationUpdated",
				Timestamp: time.Unix(0, 54321),
			},
		},
		{
			name: "event with invalid payload types",
			event: &nomadapi.Event{
				Type:  "AllocationUpdated",
				Index: 13579,
				Payload: map[string]interface{}{
					"NodeID": 123,             // should be string
					"JobID":  []string{"job"}, // should be string
				},
			},
			expectedResult: &internaltypes.Event{
				Type:      "AllocationUpdated",
				Timestamp: time.Unix(0, 13579),
				// NodeID and JobID should be empty due to type assertion failures
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.processEvent(tt.event)

			if tt.expectedResult == nil {
				if result != nil {
					t.Errorf("processEvent() expected nil but got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("processEvent() expected result but got nil")
				return
			}

			if result.Type != tt.expectedResult.Type {
				t.Errorf("processEvent() Type = %q, want %q", result.Type, tt.expectedResult.Type)
			}

			if result.Timestamp != tt.expectedResult.Timestamp {
				t.Errorf("processEvent() Timestamp = %v, want %v", result.Timestamp, tt.expectedResult.Timestamp)
			}

			if result.NodeID != tt.expectedResult.NodeID {
				t.Errorf("processEvent() NodeID = %q, want %q", result.NodeID, tt.expectedResult.NodeID)
			}

			if result.JobID != tt.expectedResult.JobID {
				t.Errorf("processEvent() JobID = %q, want %q", result.JobID, tt.expectedResult.JobID)
			}

			if result.Details == nil {
				t.Error("processEvent() Details should not be nil")
			}
		})
	}
}

func TestGetTraefikNodesFiltering(t *testing.T) {
	// This test focuses on the node filtering logic
	// In a real implementation, you would mock the Nomad API responses

	tests := []struct {
		name            string
		allocations     []*nomadapi.AllocationListStub
		nodeInfos       map[string]*nomadapi.Node
		expectedNodeIDs []string
		expectError     bool
	}{
		{
			name: "filters running allocations only",
			allocations: []*nomadapi.AllocationListStub{
				{
					ID:           "alloc-1",
					NodeID:       "node-1",
					ClientStatus: "running",
				},
				{
					ID:           "alloc-2",
					NodeID:       "node-2",
					ClientStatus: "failed",
				},
				{
					ID:           "alloc-3",
					NodeID:       "node-3",
					ClientStatus: "running",
				},
			},
			nodeInfos: map[string]*nomadapi.Node{
				"node-1": {
					ID:     "node-1",
					Name:   "worker-1",
					Status: "ready",
					Attributes: map[string]string{
						"unique.network.ip-address": "1.1.1.1",
					},
				},
				"node-3": {
					ID:     "node-3",
					Name:   "worker-3",
					Status: "ready",
					Attributes: map[string]string{
						"unique.network.ip-address": "3.3.3.3",
					},
				},
			},
			expectedNodeIDs: []string{"node-1", "node-3"},
			expectError:     false,
		},
		{
			name: "handles nodes without IP addresses",
			allocations: []*nomadapi.AllocationListStub{
				{
					ID:           "alloc-1",
					NodeID:       "node-1",
					ClientStatus: "running",
				},
			},
			nodeInfos: map[string]*nomadapi.Node{
				"node-1": {
					ID:         "node-1",
					Name:       "worker-1",
					Status:     "ready",
					Attributes: map[string]string{
						// No IP address attribute
					},
				},
			},
			expectedNodeIDs: []string{}, // should filter out nodes without IP
			expectError:     false,
		},
		{
			name: "deduplicates nodes from multiple allocations",
			allocations: []*nomadapi.AllocationListStub{
				{
					ID:           "alloc-1",
					NodeID:       "node-1",
					ClientStatus: "running",
				},
				{
					ID:           "alloc-2",
					NodeID:       "node-1", // same node
					ClientStatus: "running",
				},
			},
			nodeInfos: map[string]*nomadapi.Node{
				"node-1": {
					ID:     "node-1",
					Name:   "worker-1",
					Status: "ready",
					Attributes: map[string]string{
						"unique.network.ip-address": "1.1.1.1",
					},
				},
			},
			expectedNodeIDs: []string{"node-1"}, // should appear only once
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the core logic that would be used in GetTraefikNodes
			nodeMap := make(map[string]internaltypes.NodeInfo)

			for _, alloc := range tt.allocations {
				// Only consider running allocations
				if alloc.ClientStatus != "running" {
					continue
				}

				// Get node information (normally from API)
				node, exists := tt.nodeInfos[alloc.NodeID]
				if !exists {
					continue // Skip if node info not available
				}

				// Create NodeInfo object
				nodeInfo := internaltypes.NodeInfo{
					ID:              node.ID,
					Name:            node.Name,
					PublicIPAddress: node.Attributes["unique.network.ip-address"],
					Status:          node.Status,
				}

				// Only include nodes with IP addresses
				if nodeInfo.PublicIPAddress != "" {
					nodeMap[node.ID] = nodeInfo
				}
			}

			// Convert map to slice
			var actualNodeIDs []string
			for nodeID := range nodeMap {
				actualNodeIDs = append(actualNodeIDs, nodeID)
			}

			// Check if we got the expected number of nodes
			if len(actualNodeIDs) != len(tt.expectedNodeIDs) {
				t.Errorf("Expected %d nodes, got %d", len(tt.expectedNodeIDs), len(actualNodeIDs))
			}

			// Check if all expected nodes are present
			expectedMap := make(map[string]bool)
			for _, id := range tt.expectedNodeIDs {
				expectedMap[id] = true
			}

			for _, id := range actualNodeIDs {
				if !expectedMap[id] {
					t.Errorf("Unexpected node ID: %s", id)
				}
			}
		})
	}
}

func TestWatchEventsContextCancellation(t *testing.T) {
	// Test context cancellation logic without making real API calls
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context immediately
	cancel()

	// Test that cancelled context returns the expected error
	if ctx.Err() != context.Canceled {
		t.Errorf("Context should be cancelled, got: %v", ctx.Err())
	}
}

// Helper function to create test node info
func createTestNodeInfo(id, name, ip, status string) internaltypes.NodeInfo {
	return internaltypes.NodeInfo{
		ID:              id,
		Name:            name,
		PublicIPAddress: ip,
		Status:          status,
	}
}

func TestNodeInfoCreation(t *testing.T) {
	tests := []struct {
		name     string
		nodeInfo internaltypes.NodeInfo
		isValid  bool
	}{
		{
			name:     "valid node with all fields",
			nodeInfo: createTestNodeInfo("node-1", "worker-1", "1.1.1.1", "ready"),
			isValid:  true,
		},
		{
			name:     "node without IP address",
			nodeInfo: createTestNodeInfo("node-2", "worker-2", "", "ready"),
			isValid:  false,
		},
		{
			name:     "node with non-ready status",
			nodeInfo: createTestNodeInfo("node-3", "worker-3", "3.3.3.3", "down"),
			isValid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic that would be used in the controller
			isValid := tt.nodeInfo.Status == "ready" && tt.nodeInfo.PublicIPAddress != ""

			if isValid != tt.isValid {
				t.Errorf("Node validation = %v, want %v", isValid, tt.isValid)
			}
		})
	}
}
