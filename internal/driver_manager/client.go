package driver_manager

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
)

// Message represents a protocol message
type Message struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Method  string                 `json:"method,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Result  interface{}            `json:"result,omitempty"`
	Error   *ErrorInfo             `json:"error,omitempty"`
}

// ErrorInfo represents error information
type ErrorInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// DriverManagerClient represents a client connection to a driver manager
type DriverManagerClient struct {
	address    string
	conn       net.Conn
	scanner    *bufio.Scanner
	responses  map[string]chan Message
	mutex      sync.RWMutex
	connected  bool
	handshake  *HandshakeInfo
}

// HandshakeInfo contains information received during handshake
type HandshakeInfo struct {
	DriverCount int                    `json:"driver_count"`
	Drivers     map[string]interface{} `json:"drivers"`
}

// DriverManagerPool manages multiple driver manager connections
type DriverManagerPool struct {
	clients []DriverManagerClient
	mutex   sync.RWMutex
}

var (
	globalPool *DriverManagerPool
	poolMutex  sync.Mutex
)

// GetDriverManagerPool returns the global driver manager pool
func GetDriverManagerPool() *DriverManagerPool {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if globalPool == nil {
		globalPool = &DriverManagerPool{
			clients: make([]DriverManagerClient, 0),
		}
	}

	return globalPool
}

// NewDriverManagerClient creates a new driver manager client
func NewDriverManagerClient(address string) *DriverManagerClient {
	return &DriverManagerClient{
		address:   address,
		responses: make(map[string]chan Message),
	}
}

// Connect connects to the driver manager
func (dmc *DriverManagerClient) Connect(ctx context.Context) error {
	conn, err := net.DialTimeout("tcp", dmc.address, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to driver manager at %s: %w", dmc.address, err)
	}

	dmc.conn = conn
	dmc.scanner = bufio.NewScanner(conn)
	dmc.connected = true

	// Start message processing goroutine
	go dmc.processMessages(ctx)

	// Wait for handshake
	if err := dmc.waitForHandshake(ctx); err != nil {
		dmc.Close()
		return fmt.Errorf("handshake failed: %w", err)
	}

	utils.Log.Infof("Connected to driver manager at %s with %d drivers", 
		dmc.address, dmc.handshake.DriverCount)

	return nil
}

// Close closes the connection
func (dmc *DriverManagerClient) Close() error {
	dmc.mutex.Lock()
	defer dmc.mutex.Unlock()

	dmc.connected = false

	if dmc.conn != nil {
		return dmc.conn.Close()
	}

	return nil
}

// IsConnected returns whether the client is connected
func (dmc *DriverManagerClient) IsConnected() bool {
	dmc.mutex.RLock()
	defer dmc.mutex.RUnlock()
	return dmc.connected
}

// GetHandshakeInfo returns the handshake information
func (dmc *DriverManagerClient) GetHandshakeInfo() *HandshakeInfo {
	dmc.mutex.RLock()
	defer dmc.mutex.RUnlock()
	return dmc.handshake
}

// waitForHandshake waits for the initial handshake message
func (dmc *DriverManagerClient) waitForHandshake(ctx context.Context) error {
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("handshake timeout")
		default:
			if dmc.scanner.Scan() {
				line := strings.TrimSpace(dmc.scanner.Text())
				if line == "" {
					continue
				}

				var msg Message
				if err := json.Unmarshal([]byte(line), &msg); err != nil {
					continue
				}

				if msg.Type == "handshake" && msg.ID == "handshake" {
					// Parse handshake data
					resultData, err := json.Marshal(msg.Result)
					if err != nil {
						return fmt.Errorf("failed to marshal handshake result: %w", err)
					}

					var handshake HandshakeInfo
					if err := json.Unmarshal(resultData, &handshake); err != nil {
						return fmt.Errorf("failed to parse handshake: %w", err)
					}

					dmc.handshake = &handshake
					return nil
				}
			}

			if err := dmc.scanner.Err(); err != nil {
				return fmt.Errorf("scanner error: %w", err)
			}
		}
	}
}

// processMessages processes incoming messages
func (dmc *DriverManagerClient) processMessages(ctx context.Context) {
	defer func() {
		dmc.mutex.Lock()
		dmc.connected = false
		dmc.mutex.Unlock()
	}()

	for dmc.scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := strings.TrimSpace(dmc.scanner.Text())
		if line == "" {
			continue
		}

		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			utils.Log.Errorf("Failed to parse message: %v", err)
			continue
		}

		// Skip handshake messages (already processed)
		if msg.Type == "handshake" {
			continue
		}

		// Handle response messages
		if msg.Type == "response" && msg.ID != "" {
			dmc.mutex.RLock()
			ch, exists := dmc.responses[msg.ID]
			dmc.mutex.RUnlock()

			if exists {
				select {
				case ch <- msg:
				case <-time.After(5 * time.Second):
					utils.Log.Warnf("Response channel timeout for message ID: %s", msg.ID)
				}
			}
		}
	}

	if err := dmc.scanner.Err(); err != nil {
		utils.Log.Errorf("Scanner error: %v", err)
	}
}

// sendRequest sends a request and waits for response
func (dmc *DriverManagerClient) sendRequest(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	if !dmc.IsConnected() {
		return nil, fmt.Errorf("not connected to driver manager")
	}

	// Generate unique ID
	id := fmt.Sprintf("%s-%d", method, time.Now().UnixNano())

	// Create response channel
	respCh := make(chan Message, 1)
	dmc.mutex.Lock()
	dmc.responses[id] = respCh
	dmc.mutex.Unlock()

	defer func() {
		dmc.mutex.Lock()
		delete(dmc.responses, id)
		dmc.mutex.Unlock()
		close(respCh)
	}()

	// Create request message
	request := Message{
		ID:     id,
		Type:   "request",
		Method: method,
		Params: params,
	}

	// Send request
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := dmc.conn.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timeout.C:
		return nil, fmt.Errorf("request timeout")
	case response := <-respCh:
		if response.Error != nil {
			return nil, fmt.Errorf("driver manager error %d: %s", response.Error.Code, response.Error.Message)
		}
		return response.Result, nil
	}
}

// ListDrivers lists all available drivers
func (dmc *DriverManagerClient) ListDrivers(ctx context.Context) (map[string]interface{}, error) {
	result, err := dmc.sendRequest(ctx, "list_drivers", nil)
	if err != nil {
		return nil, err
	}

	drivers, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return drivers, nil
}

// GetDriverInfo gets information about a specific driver
func (dmc *DriverManagerClient) GetDriverInfo(ctx context.Context, driverName string) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"driver": driverName,
	}

	result, err := dmc.sendRequest(ctx, "get_driver_info", params)
	if err != nil {
		return nil, err
	}

	info, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	return info, nil
}

// CreateDriverInstance creates a new driver instance
func (dmc *DriverManagerClient) CreateDriverInstance(ctx context.Context, instanceID, driverName string, config map[string]interface{}) error {
	params := map[string]interface{}{
		"instance_id": instanceID,
		"driver":      driverName,
		"config":      config,
	}

	_, err := dmc.sendRequest(ctx, "create_instance", params)
	return err
}

// ExecuteDriverOperation executes an operation on a driver instance
func (dmc *DriverManagerClient) ExecuteDriverOperation(ctx context.Context, instanceID, operation string, operationParams map[string]interface{}) (interface{}, error) {
	params := map[string]interface{}{
		"instance_id": instanceID,
		"operation":   operation,
		"params":      operationParams,
	}

	return dmc.sendRequest(ctx, "execute_operation", params)
}

// AddDriverManager adds a driver manager to the pool
func (dmp *DriverManagerPool) AddDriverManager(ctx context.Context, address string) error {
	dmp.mutex.Lock()
	defer dmp.mutex.Unlock()

	client := NewDriverManagerClient(address)
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to driver manager: %w", err)
	}

	dmp.clients = append(dmp.clients, *client)
	utils.Log.Infof("Added driver manager: %s", address)

	return nil
}

// GetConnectedManagers returns all connected driver managers
func (dmp *DriverManagerPool) GetConnectedManagers() []DriverManagerClient {
	dmp.mutex.RLock()
	defer dmp.mutex.RUnlock()

	var connected []DriverManagerClient
	for _, client := range dmp.clients {
		if client.IsConnected() {
			connected = append(connected, client)
		}
	}

	return connected
}

// GetAllDrivers returns all drivers from all connected managers
func (dmp *DriverManagerPool) GetAllDrivers(ctx context.Context) (map[string]interface{}, error) {
	dmp.mutex.RLock()
	defer dmp.mutex.RUnlock()

	allDrivers := make(map[string]interface{})

	for _, client := range dmp.clients {
		if !client.IsConnected() {
			continue
		}

		drivers, err := client.ListDrivers(ctx)
		if err != nil {
			utils.Log.Errorf("Failed to get drivers from manager %s: %v", client.address, err)
			continue
		}

		// Merge drivers
		for name, info := range drivers {
			allDrivers[name] = info
		}
	}

	return allDrivers, nil
}

// GetDriverInfo gets driver information from any connected manager
func (dmp *DriverManagerPool) GetDriverInfo(ctx context.Context, driverName string) (map[string]interface{}, error) {
	dmp.mutex.RLock()
	defer dmp.mutex.RUnlock()

	for _, client := range dmp.clients {
		if !client.IsConnected() {
			continue
		}

		info, err := client.GetDriverInfo(ctx, driverName)
		if err == nil {
			return info, nil
		}
	}

	return nil, fmt.Errorf("driver %s not found in any connected manager", driverName)
}