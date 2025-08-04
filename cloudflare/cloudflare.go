package cloudflare

import (
	"context"
	"fmt"

	"github.com/brucellino/nomad-traefik-cloudflare-controller/config"
	"github.com/brucellino/nomad-traefik-cloudflare-controller/types"
	"github.com/charmbracelet/log"
	"github.com/cloudflare/cloudflare-go"
)

// Client wraps the Cloudflare API client
type Client struct {
	api    *cloudflare.API
	config *config.Config
}

// NewClient is a function which returns a new cloudflare client and an optional error
func NewClient(cfg *config.Config) (*Client, error) {
	api, err := cloudflare.NewWithAPIToken(cfg.CloudflareToken)
	if err != nil {
		return nil, fmt.Errorf("Failed to create cloudflare client: %w", err)
	}

	return &Client{
		api:    api,
		config: cfg,
	}, nil
}

// getARecords is a function of type cloudflare client which takes a context and returns all A records in a zone
func (c *Client) getARecords(ctx context.Context) ([]types.DNSRecord, error) {
	records, _, err := c.api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(c.config.CloudflareZoneId), cloudflare.ListDNSRecordsParams{
		Name: c.config.DNSRecordName,
		Type: "A",
	})

	if err != nil {
		return nil, fmt.Errorf("Failed to list DNS records: %w", err)
	}

	// result is a list of DNSRecords to contain the results of the lookup
	var result []types.DNSRecord
	// Loop over all of the records we've found and add them to the list of results
	for _, record := range records {
		result = append(result, types.DNSRecord{
			ID:      record.ID,
			Name:    record.Name,
			Type:    record.Type,
			Content: record.Content,
			TTL:     record.TTL,
		})
	}

	return result, nil
}

// CreateARecord is a function of type cloudflare client
// which takes a context and a string as parameters
// and returns an error.
// It creates a A record in Cloudflare with the specified target as content.

func (c *Client) CreateARecord(ctx context.Context, target string) error {
	record := cloudflare.CreateDNSRecordParams{
		Type:    "A",
		Name:    c.config.DNSRecordName,
		Content: target,
		TTL:     0,
	}

	_, err := c.api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(c.config.CloudflareZoneId), record)
	if err != nil {
		return fmt.Errorf("Failed to create A record %w", err)
	}

	log.Info("Created A record", "name", c.config.DNSRecordName, "target", target)
	return nil
}

// UpdateARecord is a function of type Cloudflare client
// which takes a context, a recordID and a target as parameters
// and returns an error
// It updates an existing record with a new target.
func (c *Client) UpdateARecord(ctx context.Context, recordID, target string) error {
	record := cloudflare.UpdateDNSRecordParams{
		ID:      recordID,
		Type:    "A",
		Name:    c.config.DNSRecordName,
		Content: target,
		TTL:     0,
	}

	_, err := c.api.UpdateDNSRecord(ctx, cloudflare.ZoneIdentifier(c.config.CloudflareZoneId), record)
	if err != nil {
		return fmt.Errorf("Unable to update DNS Record: %w", err)
	}

	log.Info("Updated A record", "name", c.config.DNSRecordName, "target", target)
	return nil

}

// DeleteARecord is a function of type cloudflare client which takes a context and a record ID as parameters and returns an error
func (c *Client) DeleteARecord(ctx context.Context, recordID string) error {
	err := c.api.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(c.config.CloudflareZoneId), recordID)
	if err != nil {
		return fmt.Errorf("Failed to delete A record: %w", err)
	}
	return nil
}

// SyncARecords synchronizes A records with the given target IPs
func (c *Client) SyncARecords(ctx context.Context, targetIPs []string) error {
	// Get current A records
	currentRecords, err := c.getARecords(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current A records: %w", err)
	}

	log.Info("Syncing A records", "current_count", len(currentRecords), "target_ips", targetIPs)

	// If no target IPs, delete all records
	if len(targetIPs) == 0 {
		for _, record := range currentRecords {
			if err := c.DeleteARecord(ctx, record.ID); err != nil {
				log.Error("Error deleting record", "record_id", record.ID, "error", err)
			}
		}
		return nil
	}

	// Create maps for easier comparison
	currentTargets := make(map[string]string) // target -> recordID
	for _, record := range currentRecords {
		currentTargets[record.Content] = record.ID
	}

	targetSet := make(map[string]bool)
	for _, ip := range targetIPs {
		targetSet[ip] = true
	}

	// Delete records that are no longer needed
	for target, recordID := range currentTargets {
		if !targetSet[target] {
			if err := c.DeleteARecord(ctx, recordID); err != nil {
				log.Error("Error deleting record", "record_id", recordID, "error", err)
			}
		}
	}

	// Create records for new targets
	for _, target := range targetIPs {
		if _, exists := currentTargets[target]; !exists {
			fmt.Print(exists)
			if err := c.CreateARecord(ctx, target); err != nil {
				log.Error("Error creating record", "target", target, "error", err)
			}
		}
	}

	return nil
}
