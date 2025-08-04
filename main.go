package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brucellino/traefik-cloudflare-controller/cloudflare"
	"github.com/brucellino/traefik-cloudflare-controller/config"
	"github.com/brucellino/traefik-cloudflare-controller/nomad"
	"github.com/brucellino/traefik-cloudflare-controller/types"
)

type Controller struct {
	nomadClient      *nomad.Client
	cloudflareClient *cloudflare.Client
	config           *config.Config
}

func main() {
	log.Println("Starting Traefik Cloudflare Controller")

	// Load configuration
	cfg, err := config.LoadConfig()

	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create Nomad client
	nomadClient, err := nomad.NewClient(cfg)

	if err != nil {
		log.Fatalf("Failed to create nomad client: %v", err)
	}

	// Create Cloudflare client
	cloudflareClient, err := cloudflare.NewClient(cfg)

	if err != nil {
		log.Fatalf("Failed to create cloudflare client: %v", err)
	}

	// Create controller instance
	controller := &Controller{
		nomadClient:      nomadClient,
		cloudflareClient: cloudflareClient,
		config:           cfg,
	}

	// Set up a context so that we can send signals and have a graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// anonymous function to receive messages in the channel
	go func() {
		<-sigChan
		log.Println("Received shutdown signal. Stopping... ")
		cancel()
	}()

	// Start the controller
	if err := controller.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Controller error: %v", err)
	}

	log.Println("Controller stopped")
}

// Run is the main work function
func (c *Controller) Run(ctx context.Context) error {
	log.Printf("Controller starting with config: Nomad=%s, Job=%s, DNS=%s",
		c.config.NomadAddress, c.config.TraefikJobName, c.config.DNSRecordName)

	// Initial sync
	//
	log.Printf("running with config %s", c.config)
	if err := c.syncDNSRecords(ctx); err != nil {
		log.Printf("Initial sync failed: %v", err)
	}

	// Set up event watching
	eventChan := make(chan types.Event, 100)
	go func() {
		if err := c.nomadClient.WatchEvents(ctx, eventChan); err != nil {
			log.Printf("Event watcher error: %v", err)
		}
	}()

	// Set up periodic sync (fallback mechanism)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		// Nomad event in channel
		case event := <-eventChan:
			log.Printf("Received event: %s", event.Type)
			// Debounce events by waiting a bit before syncing
			time.Sleep(2 * time.Second)
			if err := c.syncDNSRecords(ctx); err != nil {
				log.Printf("Sync after event failed: %v", err)
			}
		// Ticker event in channel
		case <-ticker.C:
			log.Println("Performing periodic sync...")
			if err := c.syncDNSRecords(ctx); err != nil {
				log.Printf("Periodic sync failed: %v", err)
			}
		}
	}
}

func (c *Controller) syncDNSRecords(ctx context.Context) error {
	log.Println("Syncing DNS records...")

	// Get current Traefik nodes
	nodes, err := c.nomadClient.GetTraefikNodes(ctx)
	if err != nil {
		return err
	}

	log.Printf("Found %d Traefik nodes", len(nodes))

	// Extract IP addresses
	var ips []string
	for _, node := range nodes {
		if node.Status == "ready" && node.PublicIPAddress != "" {
			ips = append(ips, node.PublicIPAddress)
			log.Printf("Traefik node: %s (%s) - %s", node.Name, node.ID, node.PublicIPAddress)
		}
	}

	// Sync with Cloudflare
	if err := c.cloudflareClient.SyncARecords(ctx, ips); err != nil {
		return err
	}

	log.Printf("DNS sync completed. %d IP addresses configured.", len(ips))
	return nil
}
