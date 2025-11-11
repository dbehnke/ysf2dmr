package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dbehnke/ysf2dmr/internal/config"
	"github.com/dbehnke/ysf2dmr/internal/network"
)

const (
	VERSION_GOROUTINE = "1.0.0-go-goroutines"
)

// GoroutineGateway represents the YSF2DMR gateway with Go-native concurrency
type GoroutineGateway struct {
	config    *config.Config
	dmrClient *network.DMRClient
	ysfClient *network.YSFClient

	// Channels for inter-component communication
	dmrToYsf chan []byte // DMR data to forward to YSF
	ysfToDmr chan []byte // YSF data to forward to DMR
	events   chan string // Status events

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
	running bool
}

// NewGoroutineGateway creates a new goroutine-based gateway
func NewGoroutineGateway(configFile string) (*GoroutineGateway, error) {
	cfg := config.NewConfig(configFile)
	if err := cfg.Load(); err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	gateway := &GoroutineGateway{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,

		// Inter-component channels
		dmrToYsf: make(chan []byte, 50),
		ysfToDmr: make(chan []byte, 50),
		events:   make(chan string, 100),
	}

	// Create DMR client
	dmrConfig := &network.DMRConfig{
		ServerAddress: cfg.GetDMRNetworkAddress(),
		ServerPort:    int(cfg.GetDMRNetworkPort()),
		LocalPort:     int(cfg.GetDMRNetworkLocal()),
		RepeaterID:    cfg.GetDMRId(),
		Password:      cfg.GetDMRNetworkPassword(),
		Callsign:      cfg.GetCallsign(),
		RxFrequency:   cfg.GetRxFrequency(),
		TxFrequency:   cfg.GetTxFrequency(),
		Power:         cfg.GetPower(),
		ColorCode:     1, // TODO: add to config
		Latitude:      float32(cfg.GetLatitude()),
		Longitude:     float32(cfg.GetLongitude()),
		Height:        int(cfg.GetHeight()),
		Location:      cfg.GetLocation(),
		Description:   cfg.GetDescription(),
		URL:           cfg.GetURL(),
		Options:       cfg.GetDMRNetworkOptions(),
	}

	var err error
	gateway.dmrClient, err = network.NewDMRClient(dmrConfig, cfg.GetDMRNetworkDebug())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create DMR client: %v", err)
	}

	// Create YSF client
	ysfConfig := &network.YSFConfig{
		ServerAddress: cfg.GetDstAddress(),
		ServerPort:    int(cfg.GetDstPort()),
		LocalAddress:  cfg.GetLocalAddress(),
		LocalPort:     int(cfg.GetLocalPort()),
		Callsign:      cfg.GetCallsign(),
	}

	gateway.ysfClient, err = network.NewYSFClient(ysfConfig, cfg.GetYSFDebug())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create YSF client: %v", err)
	}

	log.Printf("Goroutine Gateway created: DMR=%s:%d, YSF=%s:%d",
		dmrConfig.ServerAddress, dmrConfig.ServerPort,
		ysfConfig.ServerAddress, ysfConfig.ServerPort)

	return gateway, nil
}

// Run starts the gateway with Go-native concurrency
func (g *GoroutineGateway) Run() error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return fmt.Errorf("gateway already running")
	}
	g.running = true
	g.mu.Unlock()

	log.Printf("YSF2DMR Goroutine Gateway v%s starting", VERSION_GOROUTINE)
	log.Printf("Using Go-native concurrency with goroutines and channels")

	// Start network clients
	if err := g.dmrClient.Start(g.ctx); err != nil {
		return fmt.Errorf("failed to start DMR client: %v", err)
	}

	if err := g.ysfClient.Start(g.ctx); err != nil {
		return fmt.Errorf("failed to start YSF client: %v", err)
	}

	// Start processing goroutines
	g.wg.Add(4)
	go g.dmrPacketProcessor()
	go g.ysfPacketProcessor()
	go g.eventProcessor()
	go g.statusReporter()

	log.Printf("All goroutines started - Gateway running")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		log.Printf("Received shutdown signal")
	case <-g.ctx.Done():
		log.Printf("Context cancelled")
	}

	// Graceful shutdown
	g.Stop()
	return nil
}

// dmrPacketProcessor handles incoming DMR packets
func (g *GoroutineGateway) dmrPacketProcessor() {
	defer g.wg.Done()

	dmrInbound := g.dmrClient.GetInbound()
	dmrEvents := g.dmrClient.GetEvents()

	for {
		select {
		case <-g.ctx.Done():
			return

		case packet := <-dmrInbound:
			// Process DMR packet and potentially forward to YSF
			log.Printf("Processing DMR packet: %d bytes", packet.Length)
			// TODO: Implement protocol conversion logic
			// For now, just log the packet

		case event := <-dmrEvents:
			g.events <- fmt.Sprintf("DMR: %s", event)

		case data := <-g.ysfToDmr:
			// Forward YSF data to DMR (after conversion)
			log.Printf("Forwarding YSF→DMR: %d bytes", len(data))
			// TODO: Implement YSF to DMR conversion and transmission
		}
	}
}

// ysfPacketProcessor handles incoming YSF packets
func (g *GoroutineGateway) ysfPacketProcessor() {
	defer g.wg.Done()

	ysfInbound := g.ysfClient.GetInbound()
	ysfEvents := g.ysfClient.GetEvents()

	for {
		select {
		case <-g.ctx.Done():
			return

		case packet := <-ysfInbound:
			// Process YSF packet and potentially forward to DMR
			log.Printf("Processing YSF packet: %d bytes", packet.Length)
			// TODO: Implement protocol conversion logic
			// For now, just log the packet

		case event := <-ysfEvents:
			g.events <- fmt.Sprintf("YSF: %s", event)

		case data := <-g.dmrToYsf:
			// Forward DMR data to YSF (after conversion)
			log.Printf("Forwarding DMR→YSF: %d bytes", len(data))
			// TODO: Implement DMR to YSF conversion and transmission
			if err := g.ysfClient.WriteData(data); err != nil {
				log.Printf("YSF write error: %v", err)
			}
		}
	}
}

// eventProcessor handles status events from all components
func (g *GoroutineGateway) eventProcessor() {
	defer g.wg.Done()

	for {
		select {
		case <-g.ctx.Done():
			return
		case event := <-g.events:
			log.Printf("Event: %s", event)
			// TODO: Implement event-based logic (reconnection, status updates, etc.)
		}
	}
}

// statusReporter provides periodic status updates
func (g *GoroutineGateway) statusReporter() {
	defer g.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			dmrStatus := "DISCONNECTED"
			if g.dmrClient.IsConnected() {
				dmrStatus = "CONNECTED"
			}

			log.Printf("Status: DMR=%s, YSF=ACTIVE, Goroutines=Running", dmrStatus)
		}
	}
}

// Stop gracefully shuts down the gateway
func (g *GoroutineGateway) Stop() {
	g.mu.Lock()
	if !g.running {
		g.mu.Unlock()
		return
	}
	g.running = false
	g.mu.Unlock()

	log.Printf("Shutting down goroutine gateway...")

	// Stop clients
	g.dmrClient.Stop()
	g.ysfClient.Stop()

	// Cancel context to stop all goroutines
	g.cancel()

	// Wait for all goroutines to finish
	g.wg.Wait()

	log.Printf("Goroutine gateway stopped")
}

// Demo main function for the goroutine-based implementation
func mainGoroutine() {
	var configFile string
	flag.StringVar(&configFile, "config", "YSF2DMR.ini", "Configuration file path")
	flag.Parse()

	if configFile == "" {
		fmt.Println("Usage: ysf2dmr -config <config_file>")
		os.Exit(1)
	}

	gateway, err := NewGoroutineGateway(configFile)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	if err := gateway.Run(); err != nil {
		log.Fatalf("Gateway error: %v", err)
	}
}

// Test the goroutine-based implementation
func main() { mainGoroutine() }