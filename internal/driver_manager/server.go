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

// DriverManagerServer manages incoming connections from driver managers
type DriverManagerServer struct {
	listener    net.Listener
	clients     map[string]*DriverManagerConnection
	clientsMux  sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// DriverManagerConnection represents a connection from a driver manager
type DriverManagerConnection struct {
	ID          string
	conn        net.Conn
	scanner     *bufio.Scanner
	responses   map[string]chan Message
	respMux     sync.RWMutex
	handshake   *HandshakeInfo
	connected   bool
	connMux     sync.RWMutex
}

// NewDriverManagerServer creates a new driver manager server
func NewDriverManagerServer() *DriverManagerServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &DriverManagerServer{
		clients: make(map[string]*DriverManagerConnection),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the driver manager server
func (dms *DriverManagerServer) Start(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	dms.listener = listener
	utils.Log.Infof("Driver Manager Server listening on %s", address)

	go dms.acceptConnections()
	return nil
}

// Stop stops the driver manager server
func (dms *DriverManagerServer) Stop() error {
	dms.cancel()
	
	if dms.listener != nil {
		dms.listener.Close()
	}

	// Close all client connections
	dms.clientsMux.Lock()
	for _, client := range dms.clients {
		client.Close()
	}
	dms.clientsMux.Unlock()

	return nil
}

// acceptConnections accepts incoming connections from driver managers
func (dms *DriverManagerServer) acceptConnections() {
	for {
		select {
		case <-dms.ctx.Done():
			return
		default:
		}

		conn, err := dms.listener.Accept()
		if err != nil {
			if dms.ctx.Err() != nil {
				return
			}
			utils.Log.Errorf("Failed to accept connection: %v", err)
			continue
		}

		go dms.handleConnection(conn)
	}
}

// handleConnection handles a new connection from a driver manager
func (dms *DriverManagerServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	utils.Log.Infof("New driver manager connection from %s", conn.RemoteAddr())

	client := &DriverManagerConnection{
		ID:        fmt.Sprintf("dm-%d", time.Now().UnixNano()),
		conn:      conn,
		scanner:   bufio.NewScanner(conn),
		responses: make(map[string]chan Message),
		connected: true,
	}

	// Wait for handshake
	if err := dms.waitForHandshake(client); err != nil {
		utils.Log.Errorf("Handshake failed from %s: %v", conn.RemoteAddr(), err)
		return
	}

	// Register client
	dms.clientsMux.Lock()
	dms.clients[client.ID] = client
	dms.clientsMux.Unlock()

	utils.Log.Infof("Driver manager %s registered with %d drivers", 
		client.ID, client.handshake.DriverCount)

	// Start message processing
	go client.processMessages(dms.ctx)

	// Keep connection alive
	<-dms.ctx.Done()
}

// waitForHandshake waits for the handshake message from driver manager
func (dms *DriverManagerServer) waitForHandshake(client *DriverManagerConnection) error {
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("handshake timeout")
		default:
			if client.scanner.Scan() {
				line := strings.TrimSpace(client.scanner.Text())
				if line == "" {
					continue
				}

				var msg Message
				if err := json.Unmarshal([]byte(line), &msg); err != nil {
					continue
				}

				if msg.Type == "handshake" {
					// Parse handshake data
					resultData, err := json.Marshal(msg.Result)
					if err != nil {
						return fmt.Errorf("failed to marshal handshake result: %w", err)
					}

					var handshake HandshakeInfo
					if err := json.Unmarshal(resultData, &handshake); err != nil {
						return fmt.Errorf("failed to parse handshake: %w", err)
					}

					client.handshake = &handshake
					return nil
				}
			}

			if err := client.scanner.Err(); err != nil {
				return fmt.Errorf("scanner error: %w", err)
			}
		}
	}
}

// processMessages processes incoming messages from driver manager
func (dmc *DriverManagerConnection) processMessages(ctx context.Context) {
	defer func() {
		dmc.connMux.Lock()
		dmc.connected = false
		dmc.connMux.Unlock()
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
			dmc.respMux.RLock()
			ch, exists := dmc.responses[msg.ID]
			dmc.respMux.RUnlock()

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

// sendRequest sends a request to the driver manager and waits for response
func (dmc *DriverManagerConnection) sendRequest(ctx context.Context, method string, params map[string]interface{}) (interface{}, error) {
	if !dmc.IsConnected() {
		return nil, fmt.Errorf("not connected to driver manager")
	}

	// Generate unique ID
	id := fmt.Sprintf("%s-%d", method, time.Now().UnixNano())

	// Create response channel
	respCh := make(chan Message, 1)
	dmc.respMux.Lock()
	dmc.responses[id] = respCh
	dmc.respMux.Unlock()

	defer func() {
		dmc.respMux.Lock()
		delete(dmc.responses, id)
		dmc.respMux.Unlock()
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

// IsConnected returns whether the connection is active
func (dmc *DriverManagerConnection) IsConnected() bool {
	dmc.connMux.RLock()
	defer dmc.connMux.RUnlock()
	return dmc.connected
}

// Close closes the connection
func (dmc *DriverManagerConnection) Close() error {
	dmc.connMux.Lock()
	defer dmc.connMux.Unlock()

	dmc.connected = false
	if dmc.conn != nil {
		return dmc.conn.Close()
	}
	return nil
}

// GetHandshakeInfo returns the handshake information
func (dmc *DriverManagerConnection) GetHandshakeInfo() *HandshakeInfo {
	return dmc.handshake
}

// GetConnectedManagers returns all connected driver managers
func (dms *DriverManagerServer) GetConnectedManagers() []*DriverManagerConnection {
	dms.clientsMux.RLock()
	defer dms.clientsMux.RUnlock()

	var connected []*DriverManagerConnection
	for _, client := range dms.clients {
		if client.IsConnected() {
			connected = append(connected, client)
		}
	}

	return connected
}

// GetAllDrivers returns all drivers from all connected managers
func (dms *DriverManagerServer) GetAllDrivers(ctx context.Context) (map[string]interface{}, error) {
	managers := dms.GetConnectedManagers()
	allDrivers := make(map[string]interface{})

	for _, manager := range managers {
		drivers, err := manager.sendRequest(ctx, "list_drivers", nil)
		if err != nil {
			utils.Log.Errorf("Failed to get drivers from manager %s: %v", manager.ID, err)
			continue
		}

		if driversMap, ok := drivers.(map[string]interface{}); ok {
			// Merge drivers
			for name, info := range driversMap {
				allDrivers[name] = info
			}
		}
	}

	return allDrivers, nil
}

// GetDriverInfo gets driver information from any connected manager
func (dms *DriverManagerServer) GetDriverInfo(ctx context.Context, driverName string) (map[string]interface{}, error) {
	managers := dms.GetConnectedManagers()

	for _, manager := range managers {
		params := map[string]interface{}{
			"driver": driverName,
		}

		info, err := manager.sendRequest(ctx, "get_driver_info", params)
		if err == nil {
			if infoMap, ok := info.(map[string]interface{}); ok {
				return infoMap, nil
			}
		}
	}

	return nil, fmt.Errorf("driver %s not found in any connected manager", driverName)
}

// CreateDriverInstance creates a driver instance on any available manager
func (dms *DriverManagerServer) CreateDriverInstance(ctx context.Context, instanceID, driverName string, config map[string]interface{}) error {
	managers := dms.GetConnectedManagers()
	if len(managers) == 0 {
		return fmt.Errorf("no connected driver managers")
	}

	// Try to find a manager that has this driver
	for _, manager := range managers {
		params := map[string]interface{}{
			"instance_id": instanceID,
			"driver":      driverName,
			"config":      config,
		}

		_, err := manager.sendRequest(ctx, "create_instance", params)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to create driver instance on any manager")
}

// ExecuteDriverOperation executes an operation on a driver instance
func (dms *DriverManagerServer) ExecuteDriverOperation(ctx context.Context, instanceID, operation string, operationParams map[string]interface{}) (interface{}, error) {
	managers := dms.GetConnectedManagers()

	for _, manager := range managers {
		params := map[string]interface{}{
			"instance_id": instanceID,
			"operation":   operation,
			"params":      operationParams,
		}

		result, err := manager.sendRequest(ctx, "execute_operation", params)
		if err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("failed to execute operation on any manager")
}

// Global driver manager server instance
var globalDriverManagerServer *DriverManagerServer
var serverMutex sync.Mutex

// GetDriverManagerServer returns the global driver manager server
func GetDriverManagerServer() *DriverManagerServer {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if globalDriverManagerServer == nil {
		globalDriverManagerServer = NewDriverManagerServer()
	}

	return globalDriverManagerServer
}