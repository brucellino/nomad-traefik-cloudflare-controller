// Package main is the main package for this repository
package main

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brucellino/nomad-traefik-cloudflare-controller/cloudflare"
	"github.com/brucellino/nomad-traefik-cloudflare-controller/config"
	"github.com/brucellino/nomad-traefik-cloudflare-controller/metrics"
	"github.com/brucellino/nomad-traefik-cloudflare-controller/nomad"
	internaltypes "github.com/brucellino/nomad-traefik-cloudflare-controller/types"
	"github.com/charmbracelet/log"
)

// Controller is the main wrapper for the nomad and cloudflare APIs
type Controller struct {
	nomadClient      *nomad.Client
	cloudflareClient *cloudflare.Client
	config           *config.Config
	metricsServer    *metrics.Server
}

func main() {
	// Configure logger.
	// This application uses the Charm Bracelet Log package.
	logLevel := log.InfoLevel
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		switch strings.ToLower(envLevel) {
		case "debug":
			logLevel = log.DebugLevel
		case "info":
			logLevel = log.InfoLevel
		case "warn", "warning":
			logLevel = log.WarnLevel
		case "error":
			logLevel = log.ErrorLevel
		case "fatal":
			logLevel = log.FatalLevel
		}
	}

	log.SetLevel(logLevel)
	log.SetReportTimestamp(true)
	log.SetReportCaller(false)

	log.Info("Starting Traefik Cloudflare Controller", "log_level", logLevel)

	// Load configuration
	cfg, err := config.LoadConfig()

	if err != nil {
		log.Fatal("Failed to load configuration", "error", err)
	}

	// Create Nomad client
	nomadClient, err := nomad.NewClient(cfg)

	if err != nil {
		log.Fatal("Failed to create nomad client", "error", err)
	}

	// Create Cloudflare client
	cloudflareClient, err := cloudflare.NewClient(cfg)

	if err != nil {
		log.Fatal("Failed to create cloudflare client", "error", err)
	}

	// Get metrics port from config
	metricsPort := 8080
	if port, err := strconv.Atoi(cfg.MetricsPort); err == nil {
		metricsPort = port
	}

	// Create metrics server
	metricsServer := metrics.NewServer(metricsPort)

	// Create controller instance
	controller := &Controller{
		nomadClient:      nomadClient,
		cloudflareClient: cloudflareClient,
		config:           cfg,
		metricsServer:    metricsServer,
	}

	// Set up a context so that we can send signals and have a graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start metrics server
	go func() {
		if err := controller.metricsServer.Start(ctx); err != nil {
			log.Error("Metrics server error", "error", err)
		}
	}()

	// anonymous function to receive messages in the channel
	go func() {
		<-sigChan
		log.Info("Received shutdown signal. Stopping...")
		cancel()
	}()

	// Start the controller
	if err := controller.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal("Controller error", "error", err)
	}

	log.Info("Controller stopped")
}

// Run is the main work function
func (c *Controller) Run(ctx context.Context) error {
	log.Info("Controller starting",
		"nomad", c.config.NomadAddress,
		"job", c.config.TraefikJobName,
		"dns", c.config.DNSRecordName)

	// Initial sync
	//
	log.Debug("Running with config", "config", c.config)
	if err := c.syncDNSRecords(ctx); err != nil {
		log.Error("Initial sync failed", "error", err)
	} else {
		// Mark application as ready after successful initial sync
		c.metricsServer.SetReady(true)
	}

	// Set up event watching
	eventChan := make(chan internaltypes.Event, 100)
	eventErrorChan := make(chan error, 1)
	go func() {
		if err := c.nomadClient.WatchEvents(ctx, eventChan); err != nil {
			log.Error("Event watcher fatal error", "error", err)
			select {
			case eventErrorChan <- err:
			case <-ctx.Done():
			}
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

		// Event watcher fatal error - shut down gracefully
		case err := <-eventErrorChan:
			log.Error("Event watcher exceeded error threshold, shutting down", "error", err)
			return err

		// Nomad event in channel
		case event := <-eventChan:
			log.Info("Received event", "type", event.Type)
			// Debounce events by waiting a bit before syncing
			time.Sleep(2 * time.Second)
			if err := c.syncDNSRecords(ctx); err != nil {
				log.Error("Sync after event failed", "error", err)
			}
		// Ticker event in channel
		case <-ticker.C:
			log.Info("Performing periodic sync...")
			if err := c.syncDNSRecords(ctx); err != nil {
				log.Error("Periodic sync failed", "error", err)
			}
		}
	}
}

func (c *Controller) syncDNSRecords(ctx context.Context) error {
	log.Info("Syncing DNS records...")

	// Record sync metrics
	recordMetrics := metrics.RecordSyncStart()

	// Get current Traefik nodes
	nodes, err := c.nomadClient.GetTraefikNodes()
	if err != nil {
		recordMetrics(err, 0, 0)
		return err
	}

	log.Info("Found Traefik nodes", "count", len(nodes))

	// Extract IP addresses
	var ips []string
	for _, node := range nodes {
		if node.Status == "ready" && node.PublicIPAddress != "" {
			ips = append(ips, node.PublicIPAddress)
			log.Debug("Traefik node", "name", node.Name, "id", node.ID, "ip", node.PublicIPAddress)
		}
	}

	// Sync with Cloudflare
	if err := c.cloudflareClient.SyncARecords(ctx, ips); err != nil {
		recordMetrics(err, len(ips), len(nodes))
		return err
	}

	// Record successful sync
	recordMetrics(nil, len(ips), len(nodes))

	log.Info("DNS sync completed", "ip_count", len(ips))
	return nil
}
