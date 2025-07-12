package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

// ExampleDriverManagerSetup shows how to configure driver managers
func ExampleDriverManagerSetup() {
	// Initialize the driver manager pool
	op.InitDriverManagerPool()

	// Get driver manager addresses from environment variable
	// Format: "localhost:8081,localhost:8082,remote-host:8081"
	managerAddresses := os.Getenv("DRIVER_MANAGER_ADDRESSES")
	if managerAddresses == "" {
		// Default to local driver manager
		managerAddresses = "localhost:8081"
	}

	// Connect to each driver manager
	addresses := strings.Split(managerAddresses, ",")
	for _, address := range addresses {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}

		log.Printf("Connecting to driver manager at %s", address)
		err := op.AddDriverManager(context.Background(), address)
		if err != nil {
			log.Printf("Failed to connect to driver manager at %s: %v", address, err)
			// Continue with other managers even if one fails
			continue
		}
		log.Printf("Successfully connected to driver manager at %s", address)
	}

	// Verify connections by getting driver count
	ctx := context.Background()
	drivers, err := op.GetAllDriversFromManagers(ctx)
	if err != nil {
		log.Printf("Failed to get drivers from managers: %v", err)
	} else {
		log.Printf("Total drivers available from managers: %d", len(drivers))
	}
}

// This function can be called in cmd/server.go after op.InitDriverManagerPool()
func init() {
	// Uncomment the following line to enable automatic driver manager setup
	// ExampleDriverManagerSetup()
}