package nomad

import (
	"context"
	"fmt"
	"time"

	"github.com/brucellino/nomad-traefik-cloudflare-controller/config"
	"github.com/brucellino/nomad-traefik-cloudflare-controller/types"
	"github.com/charmbracelet/log"
	nomadapi "github.com/hashicorp/nomad/api"
)

// This Client type wraps the Nomad API
type Client struct {
	client *nomadapi.Client
	config *config.Config
}

// NewClient takes a Config and returns a  client and error
func NewClient(cfg *config.Config) (*Client, error) {
	nomadConfig := nomadapi.DefaultConfig()
	nomadConfig.Address = cfg.NomadAddress
	nomadConfig.SecretID = cfg.NomadToken

	client, err := nomadapi.NewClient(nomadConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Nomad client %w", err)
	}

	return &Client{
		client: client,
		config: cfg,
	}, nil
}

// GetTraefikNodes is a function of type NomadClient
// which takes a context as argument
// and returns a list of Nodes on which Traefik is deployed, as an error

func (c *Client) GetTraefikNodes(ctx context.Context) ([]types.NodeInfo, error) {
	allocations, _, err := c.client.Jobs().Allocations(c.config.TraefikJobName, true, nil)

	if err != nil {
		return nil, fmt.Errorf("Failed to get allocations for job %s: %w", c.config.TraefikJobName, err)
	}

	var nodes []types.NodeInfo
	nodeMap := make(map[string]types.NodeInfo) // avoid duplicate node names?

	// loop over allocations to get nodes
	for _, alloc := range allocations {
		// only consider running allocations
		if alloc.ClientStatus != "running" {
			continue
		}

		// get node information
		node, _, err := c.client.Nodes().Info(alloc.NodeID, nil)
		if err != nil {
			log.Warn("Failed to get node info", "node_id", alloc.NodeID, "error", err)
			continue
		}

		// Get the node's ip info. This is in the unique attributes
		// nodeAddr := c.extractNodeAddress(node)
		// // check for errors
		// if nodeAddr == "" {
		// 	log.Warn("Could not get the public IP address of the node", "node_id", node.ID)
		// 	continue
		// }

		// now we can create a nodeinfo object
		nodeInfo := types.NodeInfo{
			ID:              node.ID,
			Name:            node.Name,
			PublicIPAddress: node.Attributes["unique.network.ip-address"],
			Status:          node.Status,
		}
		nodeMap[node.ID] = nodeInfo
	} // loop over allocations

	// convert the map to a slice. Why didn't we just have a slice to start with???
	for _, node := range nodeMap {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// WatchEvents is a function of type Nomad client
// which takes a context and channel as arguments and returns an error
// It consumes the Nomad Events api described in types
func (c *Client) WatchEvents(ctx context.Context, eventChan chan<- types.Event) error {

	log.Info("Starting Nomad Event consumer")

	// Create query options for event streaming
	queryOpts := &nomadapi.QueryOptions{
		Namespace: nomadapi.DefaultNamespace,
	}
	queryOpts = queryOpts.WithContext(ctx)

	// Set up event topics we want to monitor
	topics := map[nomadapi.Topic][]string{
		nomadapi.TopicJob:        {c.config.TraefikJobName, "*"},
		nomadapi.TopicAllocation: {"*"},
		nomadapi.TopicNode:       {"*"},
	}

	// Start streaming events
	eventStream, err := c.client.EventStream().Stream(ctx, topics, 0, queryOpts)
	if err != nil {
		return fmt.Errorf("failed to start event stream: %w", err)
	}

	// Process events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case eventWrapper := <-eventStream:
			if eventWrapper.Err != nil {
				log.Error("Event stream error", "error", eventWrapper.Err)
				continue
			}

			// Process each event in the wrapper
			for _, event := range eventWrapper.Events {
				if processedEvent := c.processEvent(&event); processedEvent != nil {
					select {
					case eventChan <- *processedEvent:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}
		}
	}
}

// processEvent is a function of type nomad client which takes a nomad event as argument and returns an internal Event type
func (c *Client) processEvent(event *nomadapi.Event) *types.Event {
	// filter only for events we care about
	switch event.Type {
	// when things happen to a node or the job:
	case "AllocationUpdated", "NodeUpdated", "JobRegistered", "JobDeregistered":
		processedEvent := &types.Event{
			Type:      event.Type,
			Timestamp: time.Unix(0, int64(event.Index)),
			Details:   map[string]interface{}{"raw": event},
		}

		// Extract additional fields if available
		if event.Payload != nil {
			if nodeID, ok := event.Payload["NodeID"]; ok {
				if nodeIDStr, ok := nodeID.(string); ok {
					processedEvent.NodeID = nodeIDStr
				}
			}
			if jobID, ok := event.Payload["JobID"]; ok {
				if jobIDStr, ok := jobID.(string); ok {
					processedEvent.JobID = jobIDStr
				}
			}
		}

		return processedEvent
	}
	return nil
}
