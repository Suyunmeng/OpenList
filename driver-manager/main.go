package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/OpenListTeam/OpenList/v4/driver-manager/internal/manager"
	"github.com/OpenListTeam/OpenList/v4/driver-manager/internal/protocol"
	"github.com/OpenListTeam/OpenList/v4/driver-manager/internal/registry"
	
	// Import all drivers
	_ "github.com/OpenListTeam/OpenList/v4/drivers"
)

var (
	openlistHost = flag.String("openlist-host", "localhost", "OpenList server host to connect to")
	openlistPort = flag.Int("openlist-port", 5245, "OpenList server port to connect to")
	managerID    = flag.String("manager-id", "", "Unique manager ID (auto-generated if empty)")
	reconnectInterval = flag.Duration("reconnect-interval", 5*time.Second, "Reconnection interval")
)

func main() {
	flag.Parse()

	// Generate manager ID if not provided
	if *managerID == "" {
		*managerID = fmt.Sprintf("dm-%d", time.Now().UnixNano())
	}

	// Initialize driver registry
	driverRegistry := registry.NewDriverRegistry()
	
	// Create driver manager without external config (all config comes from OpenList database)
	driverManager := manager.NewDriverManager(driverRegistry, make(map[string]DriverConfig))

	log.Printf("Driver Manager %s starting...", *managerID)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Start connection manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		runConnectionManager(ctx, driverManager)
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down driver manager...")
	cancel()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("Driver manager stopped gracefully")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting for shutdown")
	}
}

func runConnectionManager(ctx context.Context, driverManager *manager.DriverManager) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("Connecting to OpenList at %s:%d", *openlistHost, *openlistPort)
		
		if err := connectToOpenList(ctx, driverManager); err != nil {
			log.Printf("Connection failed: %v", err)
			log.Printf("Retrying in %v", *reconnectInterval)
			
			select {
			case <-ctx.Done():
				return
			case <-time.After(*reconnectInterval):
				continue
			}
		}
	}
}

func connectToOpenList(ctx context.Context, driverManager *manager.DriverManager) error {
	addr := fmt.Sprintf("%s:%d", *openlistHost, *openlistPort)
	
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()

	log.Printf("Connected to OpenList at %s", addr)

	// Create protocol handler
	protocolHandler := protocol.NewProtocolHandler(conn, driverManager)

	// Send handshake immediately
	if err := protocolHandler.SendHandshake(*managerID); err != nil {
		return fmt.Errorf("failed to send handshake: %w", err)
	}

	log.Printf("Handshake sent to OpenList")

	// Handle the connection
	if err := protocolHandler.Handle(ctx); err != nil {
		return fmt.Errorf("connection error: %w", err)
	}

	return nil
}

// DriverConfig represents configuration for a specific driver
// This is kept for compatibility with the manager package
type DriverConfig struct {
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}