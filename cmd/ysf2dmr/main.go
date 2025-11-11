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

	"github.com/dbehnke/ysf2dmr/internal/codec"
	"github.com/dbehnke/ysf2dmr/internal/config"
	"github.com/dbehnke/ysf2dmr/internal/database"
	"github.com/dbehnke/ysf2dmr/internal/lookup"
	"github.com/dbehnke/ysf2dmr/internal/network"
	"github.com/dbehnke/ysf2dmr/internal/protocol"
	"github.com/dbehnke/ysf2dmr/internal/protocol/ysf"
	"github.com/dbehnke/ysf2dmr/internal/radioid"
	"github.com/dbehnke/ysf2dmr/internal/wiresx"
)

const (
	VERSION     = "1.0.0-go"
	DMR_FRAME_PER = 55 * time.Millisecond  // DMR frame period
	YSF_FRAME_PER = 90 * time.Millisecond  // YSF frame period
)

var (
	HEADER1 = "This software is for use on amateur radio networks only,"
	HEADER2 = "it is to be used for educational purposes only. Its use on"
	HEADER3 = "commercial networks is strictly prohibited."
	HEADER4 = "Copyright(C) 2018,2019 by CA6JAU, EA7EE, G4KLX, AD8DP and others"
	HEADER5 = "Go implementation by Claude"
)

// CallState represents the current call state
type CallState int

const (
	CallStateIdle CallState = iota
	CallStateYSF  // Receiving YSF, transmitting DMR
	CallStateDMR  // Receiving DMR, transmitting YSF
)

// Gateway represents the YSF2DMR gateway
type Gateway struct {
	config      *config.Config
	wiresX      *wiresx.WiresX
	codec       *codec.AMBEConverter
	ysfNetwork  *network.YSFNetwork
	dmrNetwork  *network.DMRNetwork
	dmrLookup   lookup.DMRLookupInterface  // Can be file-based or database-backed
	running     bool
	mu          sync.RWMutex

	// Database components (when database mode is enabled)
	db          *database.DB
	syncer      *radioid.Syncer

	// Advanced codec chain with error correction and timing
	frameRatioConverter *codec.FrameRatioConverter
	ysfExtractor       *codec.YSFAMBEExtractor
	dmrExtractor       *codec.DMRAMBEExtractor

	// Conversion state
	ysfFrames   uint32
	dmrFrames   uint32

	// Network state
	networkWatchdog time.Time
	ysfWatch        time.Time
	dmrWatch        time.Time

	// Current call state
	callState      CallState
	currentSrcID   uint32
	currentDstID   uint32
	currentStream  uint32
	hangTimer      *time.Timer
	hangTime       time.Duration

	// Network timing for Clock() calls
	lastClock     time.Time

	// Network error recovery
	dmrReconnectTimer *time.Timer
	dmrLastConnected  time.Time
	ysfErrorCount     int
	dmrErrorCount     int
}

// Define call hang time constants
const (
	DEFAULT_HANG_TIME = 3 * time.Second
	DMR_SLOT_1 = 1
	DMR_SLOT_2 = 2

	// Network error recovery constants
	DMR_RECONNECT_INTERVAL    = 30 * time.Second
	DMR_CONNECTION_CHECK      = 60 * time.Second
	MAX_NETWORK_ERRORS        = 5
	NETWORK_ERROR_RESET_TIME  = 5 * time.Minute
)

// NewGateway creates a new YSF2DMR gateway
func NewGateway(configFile string) (*Gateway, error) {
	cfg := config.NewConfig(configFile)
	if err := cfg.Load(); err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Initialize codec converter
	ambeCodec := codec.NewAMBEConverter()

	// Initialize advanced codec chain with error correction and timing
	frameRatioConverter := codec.NewFrameRatioConverter()
	ysfExtractor := codec.NewYSFAMBEExtractor()
	dmrExtractor := codec.NewDMRAMBEExtractor()

	// Initialize YSF Network - use server mode to listen for incoming YSF packets
	ysfNet := network.NewYSFNetworkServer(
		cfg.GetLocalAddress(),
		int(cfg.GetLocalPort()),
		cfg.GetCallsign(),
		cfg.GetYSFDebug(),
	)

	// Set destination for outgoing YSF packets
	err := ysfNet.SetDestinationByString(cfg.GetDstAddress(), int(cfg.GetDstPort()))
	if err != nil {
		return nil, fmt.Errorf("failed to set YSF destination: %v", err)
	}

	// Initialize DMR Network
	dmrNet, err := network.NewDMRNetwork(
		cfg.GetDMRNetworkAddress(),
		int(cfg.GetDMRNetworkPort()),
		cfg.GetDMRNetworkLocal(), // Local port for DMR socket binding (0 = any port)
		cfg.GetDMRId(),
		cfg.GetDMRNetworkPassword(),
		cfg.GetDMRNetworkOptions() != "", // duplex mode if options exist
		VERSION,
		cfg.GetDMRNetworkDebug(),
		true,  // slot1 - use default for now
		true,  // slot2 - use default for now
		protocol.HW_TYPE_HOMEBREW, // Default to homebrew for now
		int(cfg.GetDMRNetworkJitter()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create DMR network: %v", err)
	}

	// Set DMR network configuration
	dmrNet.SetConfig(
		cfg.GetCallsign(),
		cfg.GetRxFrequency(),
		cfg.GetTxFrequency(),
		cfg.GetPower(),
		1, // Color code default - TODO: add to config
		float32(cfg.GetLatitude()),
		float32(cfg.GetLongitude()),
		int(cfg.GetHeight()),
		cfg.GetLocation(),
		cfg.GetDescription(),
		cfg.GetURL(),
	)

	// Set DMR options if provided
	if cfg.GetDMRNetworkOptions() != "" {
		dmrNet.SetOptions(cfg.GetDMRNetworkOptions())
	}

	// Initialize WiresX if enabled
	var wx *wiresx.WiresX
	if cfg.GetEnableWiresX() {
		wx = wiresx.NewWiresX(
			cfg.GetCallsign(),
			cfg.GetSuffix(),
			nil, // Network writer will be set later
			cfg.GetDMRTGListFile(),
			cfg.GetWiresXMakeUpper(),
		)
		wx.SetInfo(
			cfg.GetDescription(),
			cfg.GetTxFrequency(),
			cfg.GetRxFrequency(),
			cfg.GetDMRDstId(),
		)
	}

	// Initialize DMR Lookup (database-backed or file-based)
	dmrLookup, db, syncer := initializeDMRLookup(cfg)

	now := time.Now()
	gateway := &Gateway{
		config:              cfg,
		wiresX:              wx,
		codec:               ambeCodec,
		ysfNetwork:          ysfNet,
		dmrNetwork:          dmrNet,
		dmrLookup:           dmrLookup,
		db:                  db,
		syncer:              syncer,
		frameRatioConverter: frameRatioConverter,
		ysfExtractor:        ysfExtractor,
		dmrExtractor:        dmrExtractor,
		callState:           CallStateIdle,
		networkWatchdog:     now,
		ysfWatch:            now,
		dmrWatch:            now,
		lastClock:           now,
		hangTime:            time.Duration(cfg.GetHangTime()) * time.Second,
		currentDstID:        cfg.GetDMRDstId(), // Default destination
		dmrLastConnected:    now,
		ysfErrorCount:       0,
		dmrErrorCount:       0,
	}

	// Set default hang time if not configured
	if gateway.hangTime == 0 {
		gateway.hangTime = DEFAULT_HANG_TIME
	}

	return gateway, nil
}

// formatDMRAddress formats a DMR ID with callsign lookup (matching C++ behavior)
func (g *Gateway) formatDMRAddress(id uint32, isGroup bool) string {
	if g.dmrLookup != nil {
		callsign := g.dmrLookup.FindCS(id)
		if isGroup {
			return fmt.Sprintf("TG %s", callsign)
		}
		return callsign
	}

	// Fallback if no lookup available
	if isGroup {
		return fmt.Sprintf("TG %d", id)
	}
	return fmt.Sprintf("%d", id)
}

// Run starts the gateway main loop
func (g *Gateway) Run(ctx context.Context) error {
	g.mu.Lock()
	g.running = true
	g.mu.Unlock()

	log.Printf("YSF2DMR Gateway v%s starting", VERSION)
	log.Printf("Callsign: %s-%s", g.config.GetCallsign(), g.config.GetSuffix())
	log.Printf("YSF: %s:%d -> %s:%d",
		g.config.GetLocalAddress(), g.config.GetLocalPort(),
		g.config.GetDstAddress(), g.config.GetDstPort())
	log.Printf("DMR: %s:%d (ID: %d)",
		g.config.GetDMRNetworkAddress(), g.config.GetDMRNetworkPort(),
		g.config.GetDMRId())

	if g.config.GetEnableWiresX() {
		log.Printf("WiresX enabled")
	}

	// Open networks
	if err := g.ysfNetwork.Open(); err != nil {
		return fmt.Errorf("failed to open YSF network: %v", err)
	}

	if err := g.dmrNetwork.Open(); err != nil {
		g.ysfNetwork.Close()
		return fmt.Errorf("failed to open DMR network: %v", err)
	}

	// Enable DMR network
	g.dmrNetwork.Enable(true)

	// Setup periodic timers
	ysfTicker := time.NewTicker(YSF_FRAME_PER)
	dmrTicker := time.NewTicker(DMR_FRAME_PER)
	statsTicker := time.NewTicker(30 * time.Second)
	networkTicker := time.NewTicker(10 * time.Millisecond) // Network Clock() timing
	ysfPollTicker := time.NewTicker(5 * time.Second) // YSF keep-alive poll messages

	defer func() {
		ysfTicker.Stop()
		dmrTicker.Stop()
		statsTicker.Stop()
		networkTicker.Stop()
		ysfPollTicker.Stop()
		if g.hangTimer != nil {
			g.hangTimer.Stop()
		}
		if g.dmrReconnectTimer != nil {
			g.dmrReconnectTimer.Stop()
		}
		g.ysfNetwork.Close()
		g.dmrNetwork.Close()
		if g.dmrLookup != nil {
			g.dmrLookup.Stop()
		}
	}()

	log.Printf("Gateway running - press Ctrl+C to stop")

	for {
		select {
		case <-ctx.Done():
			log.Printf("Shutdown requested")
			g.mu.Lock()
			g.running = false
			g.mu.Unlock()
			return nil

		case <-networkTicker.C:
			// Call Clock() methods for networks - this is critical for DMR authentication
			now := time.Now()
			elapsed := int(now.Sub(g.lastClock).Milliseconds())
			g.lastClock = now

			g.ysfNetwork.Clock(elapsed)
			g.dmrNetwork.Clock(elapsed)

			// Process network data after Clock() calls
			if err := g.processNetworks(); err != nil {
				log.Printf("Network processing error: %v", err)
			}

		case <-ysfTicker.C:
			if err := g.processYSFTimer(); err != nil {
				log.Printf("YSF timer error: %v", err)
			}

		case <-dmrTicker.C:
			if err := g.processDMRTimer(); err != nil {
				log.Printf("DMR timer error: %v", err)
			}

		case <-statsTicker.C:
			g.printStats()

		case <-ysfPollTicker.C:
			// Send YSF poll message for keep-alive
			if err := g.ysfNetwork.WritePoll(); err != nil {
				log.Printf("YSF poll error: %v", err)
				g.ysfErrorCount++
			}

		default:
			// Process WiresX if enabled
			if g.wiresX != nil {
				g.wiresX.Clock(uint32(time.Since(g.ysfWatch).Milliseconds()))
			}

			// Check hang timer
			g.checkHangTimer()

			// Monitor network health and handle recovery
			g.monitorNetworkHealth()

			// Small sleep to prevent busy loop
			time.Sleep(time.Millisecond)
		}
	}
}

// processNetworks handles incoming data from both networks
func (g *Gateway) processNetworks() error {
	// Process YSF network data
	ysfBuffer := make([]byte, 200) // Buffer for YSF frames
	if bytesRead := g.ysfNetwork.Read(ysfBuffer); bytesRead > 0 {
		ysfData := ysfBuffer[:bytesRead]
		if err := g.processYSFData(ysfData); err != nil {
			log.Printf("YSF data processing error: %v", err)
		}
	}

	// Process DMR network data
	dmrData := protocol.NewDMRData()
	if g.dmrNetwork.Read(dmrData) {
		if err := g.processDMRData(dmrData); err != nil {
			log.Printf("DMR data processing error: %v", err)
		}
	}

	return nil
}

// processYSFData processes incoming YSF data
func (g *Gateway) processYSFData(data []byte) error {
	// Parse YSF frame
	frame := &ysf.Frame{}
	if err := frame.Parse(data); err != nil {
		return fmt.Errorf("YSF frame parse error: %v", err)
	}

	log.Printf("YSF: %s -> %s (%s)", frame.SourceCallsign, frame.DestCallsign, frame.FICH.String())

	// Update call state if this is the start of a new call (header frame)
	if frame.IsHeader() {
		g.startYSFCall(frame.SourceCallsign)
	}

	// Handle terminator frames
	if frame.IsTerminator() {
		g.endCall()
	}

	// Process WiresX if enabled and this is a data frame
	if g.wiresX != nil && frame.IsData() {
		status := g.wiresX.Process(frame.Payload, []byte(frame.SourceCallsign),
			frame.FICH.FI, frame.FICH.DT, frame.FICH.FN, frame.FICH.FT)

		switch status {
		case wiresx.StatusConnect:
			dstID := g.wiresX.GetDstID()
			tgStr := g.formatDMRAddress(dstID, true) // TG is always a group
			log.Printf("WiresX connect to %s", tgStr)
			g.currentDstID = dstID
			g.wiresX.SendConnectReply(dstID)
		case wiresx.StatusDisconnect:
			log.Printf("WiresX disconnect")
			g.currentDstID = 0
			g.wiresX.SendDisconnectReply()
		case wiresx.StatusDX:
			log.Printf("WiresX DX request")
		case wiresx.StatusAll:
			log.Printf("WiresX ALL request")
		}
	}

	// Extract audio and convert to DMR if this is a voice frame
	if frame.IsVoice() {
		// Use advanced codec chain with Frame Ratio Converter for proper 3:5 timing
		dmrFrames, err := g.frameRatioConverter.ConvertYSFToDMR(frame.Payload)
		if err != nil {
			log.Printf("YSF to DMR conversion error: %v", err)
		} else if len(dmrFrames) > 0 {
			// Frame Ratio Converter has produced DMR frames (3 YSF → 5 DMR)
			log.Printf("Generated %d DMR frames from YSF frame buffer", len(dmrFrames))
			for i, dmrFrame := range dmrFrames {
				if err := g.sendDMRFrame(dmrFrame); err != nil {
					log.Printf("DMR send error (frame %d): %v", i, err)
				}
			}
		}
		// If len(dmrFrames) == 0, the frame is buffered waiting for complete 3-frame set
	}

	g.ysfFrames++
	return nil
}

// processDMRData processes incoming DMR data
func (g *Gateway) processDMRData(data *protocol.DMRData) error {
	// Format source and destination with callsign lookup (matching C++ behavior)
	srcStr := g.formatDMRAddress(data.GetSrcId(), false) // Source is never a group
	dstStr := g.formatDMRAddress(data.GetDstId(), data.IsGroupCall())

	log.Printf("DMR: Slot %d, Src %s, Dst %s, FLCO %s, DT %s, Seq %d",
		data.GetSlotNo(), srcStr, dstStr,
		data.GetFLCOString(), data.GetDataTypeString(), data.GetSeqNo())

	// Update call state if this is the start of a new call
	if data.IsVoiceLCHeader() {
		g.startDMRCall(data.GetSrcId(), data.GetDstId(), data.GetStreamId())
	}

	// Extract audio and convert to YSF if this is a voice frame
	if data.IsVoice() {
		dmrPayload := data.GetData()

		// Use advanced codec chain with Frame Ratio Converter for proper 5:3 timing
		ysfFrames, err := g.frameRatioConverter.ConvertDMRToYSF(dmrPayload[:])
		if err != nil {
			log.Printf("DMR to YSF conversion error: %v", err)
		} else if len(ysfFrames) > 0 {
			// Frame Ratio Converter has produced YSF frames (5 DMR → 3 YSF)
			log.Printf("Generated %d YSF frames from DMR frame buffer", len(ysfFrames))
			for i, ysfFrame := range ysfFrames {
				if err := g.sendYSFFrame(ysfFrame); err != nil {
					log.Printf("YSF send error (frame %d): %v", i, err)
				}
			}
		}
		// If len(ysfFrames) == 0, the frame is buffered waiting for complete 5-frame set
	}

	// Handle call termination
	if data.IsTerminator() {
		g.endCall()
	}

	g.dmrFrames++
	g.networkWatchdog = time.Now()
	return nil
}

// sendDMRFrame sends a DMR frame
func (g *Gateway) sendDMRFrame(audioData []byte) error {
	// Create DMR data structure
	dmrData := protocol.NewDMRData()
	dmrData.SetSlotNo(2) // Use slot 2 for XLX
	dmrData.SetSrcId(g.config.GetDMRId())
	dmrData.SetDstId(g.currentDstID)
	dmrData.SetFLCO(protocol.FLCO_GROUP)
	dmrData.SetDataType(protocol.DT_VOICE)
	dmrData.SetSeqNo(uint8(g.dmrFrames % 256))

	// Copy audio data to payload - truncate if necessary
	var payload [33]byte
	copyLen := len(audioData)
	if copyLen > 33 {
		copyLen = 33
	}
	copy(payload[:], audioData[:copyLen])
	dmrData.SetData(payload[:])

	// Send via network
	return g.dmrNetwork.Write(dmrData)
}

// sendYSFFrame sends a YSF frame
func (g *Gateway) sendYSFFrame(audioData []byte) error {
	// Create YSF frame
	frame := &ysf.Frame{
		SourceCallsign: g.config.GetCallsign(),
		DestCallsign:   "ALL",
		FICH: ysf.FICH{
			FI: 1, // Communications
			DT: 0, // VD Mode 1
			CM: 0, // Group call
			FN: uint8(g.ysfFrames % 8),
		},
		Payload: make([]byte, 90),
	}

	// Copy audio data to payload
	copy(frame.Payload, audioData)

	// Build and send frame
	frameData := frame.Build()
	return g.ysfNetwork.Write(frameData)
}

// processYSFTimer handles YSF timing events
func (g *Gateway) processYSFTimer() error {
	g.ysfWatch = time.Now()
	// YSF timing logic would go here
	return nil
}

// processDMRTimer handles DMR timing events
func (g *Gateway) processDMRTimer() error {
	g.dmrWatch = time.Now()

	// Check network watchdog
	if time.Since(g.networkWatchdog) > 30*time.Second {
		log.Printf("Network watchdog expired")
		g.networkWatchdog = time.Now()
		g.dmrFrames = 0
	}

	return nil
}

// printStats prints periodic statistics
func (g *Gateway) printStats() {
	connectionStatus := "Disconnected"
	dmrState := g.dmrNetwork.GetStatusString()
	if g.dmrNetwork.IsConnected() {
		connectionStatus = "Connected"
	}

	// Get Frame Ratio Converter statistics
	ysfToDmr, dmrToYsf, convErrors := g.frameRatioConverter.GetConversionStats()

	log.Printf("Stats: YSF frames: %d, DMR frames: %d, Current TG: %d, DMR: %s (%s), State: %v",
		g.ysfFrames, g.dmrFrames, g.currentDstID, connectionStatus, dmrState, g.callState)
	log.Printf("Codec: YSF→DMR: %d, DMR→YSF: %d, Conv Errors: %d, YSF Buffer: %v, DMR Buffer: %v",
		ysfToDmr, dmrToYsf, convErrors,
		g.frameRatioConverter.IsYSFBufferReady(), g.frameRatioConverter.IsDMRBufferReady())
}

// startYSFCall starts a new call from YSF
func (g *Gateway) startYSFCall(srcCallsign string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	log.Printf("Starting YSF call from %s", srcCallsign)
	g.callState = CallStateYSF

	// Reset frame ratio converter for clean state
	g.frameRatioConverter.Reset()

	// Stop any existing hang timer
	if g.hangTimer != nil {
		g.hangTimer.Stop()
	}
}

// startDMRCall starts a new call from DMR
func (g *Gateway) startDMRCall(srcId, dstId, streamId uint32) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Format IDs with callsign lookup (matching C++ behavior)
	srcStr := g.formatDMRAddress(srcId, false) // Source is never a group
	dstStr := g.formatDMRAddress(dstId, true)  // Destination could be group or user, assume group for now

	log.Printf("Starting DMR call from %s to %s (stream 0x%08X)", srcStr, dstStr, streamId)
	g.callState = CallStateDMR
	g.currentSrcID = srcId
	g.currentStream = streamId

	// Reset frame ratio converter for clean state
	g.frameRatioConverter.Reset()

	// Stop any existing hang timer
	if g.hangTimer != nil {
		g.hangTimer.Stop()
	}
}

// endCall ends the current call and starts hang timer
func (g *Gateway) endCall() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.callState != CallStateIdle {
		log.Printf("Ending call, starting hang timer (%v)", g.hangTime)
		g.callState = CallStateIdle

		// Start hang timer
		if g.hangTimer != nil {
			g.hangTimer.Stop()
		}
		g.hangTimer = time.AfterFunc(g.hangTime, func() {
			log.Printf("Hang timer expired")
			// Additional cleanup if needed
		})
	}
}

// checkHangTimer checks and manages the hang timer
func (g *Gateway) checkHangTimer() {
	// Hang timer is managed by time.AfterFunc, no action needed here
	// This method exists for future enhancements if needed
}

// monitorNetworkHealth checks network connection status and handles recovery
func (g *Gateway) monitorNetworkHealth() {
	now := time.Now()

	// Check DMR network connection
	if g.dmrNetwork.IsConnected() {
		g.dmrLastConnected = now
		g.dmrErrorCount = 0 // Reset error count when connected
	} else {
		// DMR not connected - check if we need to attempt reconnection
		if now.Sub(g.dmrLastConnected) > DMR_CONNECTION_CHECK {
			if g.dmrReconnectTimer == nil {
				log.Printf("DMR network disconnected, scheduling reconnection...")
				g.scheduleReconnect()
			}
		}
	}

	// Reset error counts periodically
	if now.Sub(g.networkWatchdog) > NETWORK_ERROR_RESET_TIME {
		if g.ysfErrorCount > 0 || g.dmrErrorCount > 0 {
			log.Printf("Resetting network error counts (YSF: %d, DMR: %d)",
				g.ysfErrorCount, g.dmrErrorCount)
			g.ysfErrorCount = 0
			g.dmrErrorCount = 0
		}
		g.networkWatchdog = now
	}
}

// scheduleReconnect schedules a DMR network reconnection attempt
func (g *Gateway) scheduleReconnect() {
	if g.dmrReconnectTimer != nil {
		g.dmrReconnectTimer.Stop()
	}

	g.dmrReconnectTimer = time.AfterFunc(DMR_RECONNECT_INTERVAL, func() {
		g.attemptReconnect()
	})
}

// attemptReconnect attempts to reconnect the DMR network
func (g *Gateway) attemptReconnect() {
	log.Printf("Attempting DMR network reconnection...")

	g.mu.Lock()
	defer g.mu.Unlock()

	// Close existing connection
	g.dmrNetwork.Close()

	// Attempt to reopen
	if err := g.dmrNetwork.Open(); err != nil {
		log.Printf("DMR reconnection failed: %v", err)
		g.dmrErrorCount++

		if g.dmrErrorCount < MAX_NETWORK_ERRORS {
			g.scheduleReconnect() // Try again
		} else {
			log.Printf("Maximum DMR reconnection attempts reached, giving up")
		}
	} else {
		log.Printf("DMR network reconnected successfully")
		g.dmrNetwork.Enable(true)
		g.dmrErrorCount = 0
		g.dmrLastConnected = time.Now()

		if g.dmrReconnectTimer != nil {
			g.dmrReconnectTimer.Stop()
			g.dmrReconnectTimer = nil
		}
	}
}

// handleNetworkError increments error count and triggers recovery if needed
func (g *Gateway) handleNetworkError(network string, err error) {
	if err == nil {
		return
	}

	log.Printf("%s network error: %v", network, err)

	if network == "YSF" {
		g.ysfErrorCount++
		// YSF is simpler - just log errors for now
		// Could add YSF reconnection logic here if needed
	} else if network == "DMR" {
		g.dmrErrorCount++
		if !g.dmrNetwork.IsConnected() && g.dmrReconnectTimer == nil {
			g.scheduleReconnect()
		}
	}
}

func mainOriginal() { // Temporarily renamed to test goroutine version
	var (
		configFile = flag.String("config", getDefaultConfig(), "Configuration file path")
		version    = flag.Bool("version", false, "Show version information")
		verbose    = flag.Bool("v", false, "Show version information")
	)
	flag.Parse()

	if *version || *verbose {
		fmt.Printf("YSF2DMR Gateway v%s\n", VERSION)
		fmt.Println(HEADER1)
		fmt.Println(HEADER2)
		fmt.Println(HEADER3)
		fmt.Println(HEADER4)
		fmt.Println(HEADER5)
		return
	}

	// Handle non-flag arguments (config file)
	if flag.NArg() > 0 {
		*configFile = flag.Arg(0)
	}

	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("YSF2DMR Gateway v%s starting with config: %s", VERSION, *configFile)

	// Create gateway
	gateway, err := NewGateway(*configFile)
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Run gateway
	if err := gateway.Run(ctx); err != nil {
		log.Fatalf("Gateway error: %v", err)
	}

	log.Printf("YSF2DMR Gateway stopped")
}

// initializeDMRLookup creates either a database-backed or file-based DMR lookup service
// Returns the lookup interface, database instance (if database mode), and syncer (if database mode)
func initializeDMRLookup(cfg *config.Config) (lookup.DMRLookupInterface, *database.DB, *radioid.Syncer) {
	// Check if database mode is enabled
	if cfg.GetDatabaseEnabled() {
		log.Printf("Initializing database-backed DMR lookup...")

		// Create database with configuration
		dbConfig := database.Config{
			Path: cfg.GetDatabasePath(),
		}

		db, err := database.NewDB(dbConfig, log.New(os.Stdout, "[DB] ", log.LstdFlags))
		if err != nil {
			log.Printf("Failed to initialize database: %v", err)
			log.Printf("Falling back to file-based lookup...")
			return initializeFileLookup(cfg), nil, nil
		}

		// Create repository
		userRepo := database.NewDMRUserRepository(db.GetDB())

		// Create database adapter with configuration
		cacheSize := cfg.GetDatabaseCacheSize()
		if cacheSize == 0 {
			cacheSize = 1000 // Default
		}

		adapterConfig := lookup.DMRDatabaseAdapterConfig{
			EnableCache:   true,
			CacheSize:     int(cacheSize),
			CacheExpiry:   5 * time.Minute,
		}
		adapter := lookup.NewDMRDatabaseAdapterWithConfig(userRepo, adapterConfig)
		adapter.SetDebug(cfg.GetDatabaseDebug())

		// Start the adapter
		if err := adapter.Start(); err != nil {
			log.Printf("Failed to start database adapter: %v", err)
			log.Printf("Falling back to file-based lookup...")
			db.Close()
			return initializeFileLookup(cfg), nil, nil
		}

		// Create and start RadioID syncer
		syncHours := cfg.GetDatabaseSyncHours()
		if syncHours == 0 {
			syncHours = 24 // Default
		}

		syncerConfig := radioid.SyncerConfig{
			SyncInterval: time.Duration(syncHours) * time.Hour,
			HTTPTimeout:  30 * time.Second,
		}

		syncer := radioid.NewSyncerWithConfig(userRepo, log.New(os.Stdout, "[SYNC] ", log.LstdFlags), syncerConfig)

		// Start syncer in background
		go syncer.Start(context.Background())

		count := adapter.GetEntryCount()
		log.Printf("Database-backed DMR lookup initialized with %d entries", count)

		return adapter, db, syncer
	}

	// Fall back to file-based lookup
	return initializeFileLookup(cfg), nil, nil
}

// initializeFileLookup creates a traditional file-based DMR lookup
func initializeFileLookup(cfg *config.Config) lookup.DMRLookupInterface {
	if cfg.GetDMRIdLookupFile() == "" {
		log.Printf("DMR ID lookup disabled (no file configured and database mode disabled)")
		return nil
	}

	dmrLookup := lookup.NewDMRLookup(
		cfg.GetDMRIdLookupFile(),
		cfg.GetDMRIdLookupTime(),
	)
	dmrLookup.SetDebug(cfg.GetDatabaseDebug()) // Use same debug setting

	// Start the lookup service
	if err := dmrLookup.Start(); err != nil {
		log.Printf("Warning: Failed to start file-based DMR ID lookup: %v", err)
		return nil // Disable lookup on error
	}

	log.Printf("File-based DMR ID lookup initialized with %d entries from %s",
		dmrLookup.GetEntryCount(), cfg.GetDMRIdLookupFile())

	return dmrLookup
}

// getDefaultConfig returns the default configuration file path
func getDefaultConfig() string {
	// Check for config file in current directory first
	if _, err := os.Stat("YSF2DMR.ini"); err == nil {
		return "YSF2DMR.ini"
	}

	// Check system location
	systemConfig := "/etc/YSF2DMR.ini"
	if _, err := os.Stat(systemConfig); err == nil {
		return systemConfig
	}

	// Default to current directory
	return "YSF2DMR.ini"
}