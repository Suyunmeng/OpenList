package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/driver-manager/internal/manager"
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

// ProtocolHandler handles the communication protocol
type ProtocolHandler struct {
	conn          net.Conn
	driverManager *manager.DriverManager
	scanner       *bufio.Scanner
}

// NewProtocolHandler creates a new protocol handler
func NewProtocolHandler(conn net.Conn, driverManager *manager.DriverManager) *ProtocolHandler {
	return &ProtocolHandler{
		conn:          conn,
		driverManager: driverManager,
		scanner:       bufio.NewScanner(conn),
	}
}

// Handle handles the connection
func (ph *ProtocolHandler) Handle(ctx context.Context) error {
	// Process messages
	for ph.scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := strings.TrimSpace(ph.scanner.Text())
		if line == "" {
			continue
		}

		if err := ph.processMessage(ctx, line); err != nil {
			return fmt.Errorf("failed to process message: %w", err)
		}
	}

	return ph.scanner.Err()
}

// SendHandshake sends initial handshake message with manager ID
func (ph *ProtocolHandler) SendHandshake(managerID string) error {
	registry := ph.driverManager.GetDriverRegistry()
	drivers := registry.ListDrivers()

	// Create driver info for handshake
	driverInfo := make(map[string]interface{})
	for name, driver := range drivers {
		driverInfo[name] = map[string]interface{}{
			"name":   driver.Name,
			"config": driver.Config,
			"items":  driver.Items,
			"i18n":   driver.I18n,
		}
	}

	handshake := Message{
		ID:   "handshake",
		Type: "handshake",
		Result: map[string]interface{}{
			"manager_id":   managerID,
			"driver_count": registry.GetDriverCount(),
			"drivers":      driverInfo,
		},
	}

	return ph.sendMessage(handshake)
}

// processMessage processes a received message
func (ph *ProtocolHandler) processMessage(ctx context.Context, line string) error {
	var msg Message
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return ph.sendError(msg.ID, 400, fmt.Sprintf("Invalid JSON: %v", err))
	}

	switch msg.Type {
	case "request":
		return ph.handleRequest(ctx, msg)
	case "ping":
		return ph.handlePing(msg)
	default:
		return ph.sendError(msg.ID, 400, fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

// handleRequest handles a request message
func (ph *ProtocolHandler) handleRequest(ctx context.Context, msg Message) error {
	switch msg.Method {
	case "list_drivers":
		return ph.handleListDrivers(msg)
	case "get_driver_info":
		return ph.handleGetDriverInfo(msg)
	case "create_instance":
		return ph.handleCreateInstance(ctx, msg)
	case "remove_instance":
		return ph.handleRemoveInstance(ctx, msg)
	case "list_instances":
		return ph.handleListInstances(msg)
	case "enable_instance":
		return ph.handleEnableInstance(msg)
	case "disable_instance":
		return ph.handleDisableInstance(msg)
	case "execute_operation":
		return ph.handleExecuteOperation(ctx, msg)
	default:
		return ph.sendError(msg.ID, 400, fmt.Sprintf("Unknown method: %s", msg.Method))
	}
}

// handlePing handles a ping message
func (ph *ProtocolHandler) handlePing(msg Message) error {
	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: "pong",
	}
	return ph.sendMessage(response)
}

// handleListDrivers handles list drivers request
func (ph *ProtocolHandler) handleListDrivers(msg Message) error {
	registry := ph.driverManager.GetDriverRegistry()
	drivers := registry.ListDrivers()

	result := make(map[string]interface{})
	for name, driver := range drivers {
		result[name] = map[string]interface{}{
			"name":   driver.Name,
			"config": driver.Config,
			"i18n":   driver.I18n,
		}
	}

	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: result,
	}
	return ph.sendMessage(response)
}

// handleGetDriverInfo handles get driver info request
func (ph *ProtocolHandler) handleGetDriverInfo(msg Message) error {
	driverName, ok := msg.Params["driver"].(string)
	if !ok {
		return ph.sendError(msg.ID, 400, "driver parameter is required")
	}

	registry := ph.driverManager.GetDriverRegistry()
	driver, err := registry.GetDriver(driverName)
	if err != nil {
		return ph.sendError(msg.ID, 404, err.Error())
	}

	result := map[string]interface{}{
		"name":   driver.Name,
		"config": driver.Config,
		"items":  driver.Items,
		"i18n":   driver.I18n,
	}

	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: result,
	}
	return ph.sendMessage(response)
}

// handleCreateInstance handles create instance request
func (ph *ProtocolHandler) handleCreateInstance(ctx context.Context, msg Message) error {
	instanceID, ok := msg.Params["instance_id"].(string)
	if !ok {
		return ph.sendError(msg.ID, 400, "instance_id parameter is required")
	}

	driverName, ok := msg.Params["driver"].(string)
	if !ok {
		return ph.sendError(msg.ID, 400, "driver parameter is required")
	}

	config, ok := msg.Params["config"].(map[string]interface{})
	if !ok {
		config = make(map[string]interface{})
	}

	err := ph.driverManager.CreateDriverInstance(ctx, instanceID, driverName, config)
	if err != nil {
		return ph.sendError(msg.ID, 500, err.Error())
	}

	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: "success",
	}
	return ph.sendMessage(response)
}

// handleRemoveInstance handles remove instance request
func (ph *ProtocolHandler) handleRemoveInstance(ctx context.Context, msg Message) error {
	instanceID, ok := msg.Params["instance_id"].(string)
	if !ok {
		return ph.sendError(msg.ID, 400, "instance_id parameter is required")
	}

	err := ph.driverManager.RemoveDriverInstance(ctx, instanceID)
	if err != nil {
		return ph.sendError(msg.ID, 500, err.Error())
	}

	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: "success",
	}
	return ph.sendMessage(response)
}

// handleListInstances handles list instances request
func (ph *ProtocolHandler) handleListInstances(msg Message) error {
	instances := ph.driverManager.ListDriverInstances()

	result := make(map[string]interface{})
	for id, instance := range instances {
		result[id] = map[string]interface{}{
			"id":      instance.ID,
			"name":    instance.Name,
			"enabled": instance.Enabled,
			"config":  instance.Config,
		}
	}

	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: result,
	}
	return ph.sendMessage(response)
}

// handleEnableInstance handles enable instance request
func (ph *ProtocolHandler) handleEnableInstance(msg Message) error {
	instanceID, ok := msg.Params["instance_id"].(string)
	if !ok {
		return ph.sendError(msg.ID, 400, "instance_id parameter is required")
	}

	err := ph.driverManager.EnableDriverInstance(instanceID)
	if err != nil {
		return ph.sendError(msg.ID, 500, err.Error())
	}

	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: "success",
	}
	return ph.sendMessage(response)
}

// handleDisableInstance handles disable instance request
func (ph *ProtocolHandler) handleDisableInstance(msg Message) error {
	instanceID, ok := msg.Params["instance_id"].(string)
	if !ok {
		return ph.sendError(msg.ID, 400, "instance_id parameter is required")
	}

	err := ph.driverManager.DisableDriverInstance(instanceID)
	if err != nil {
		return ph.sendError(msg.ID, 500, err.Error())
	}

	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: "success",
	}
	return ph.sendMessage(response)
}

// handleExecuteOperation handles execute operation request
func (ph *ProtocolHandler) handleExecuteOperation(ctx context.Context, msg Message) error {
	instanceID, ok := msg.Params["instance_id"].(string)
	if !ok {
		return ph.sendError(msg.ID, 400, "instance_id parameter is required")
	}

	operation, ok := msg.Params["operation"].(string)
	if !ok {
		return ph.sendError(msg.ID, 400, "operation parameter is required")
	}

	operationParams, ok := msg.Params["params"].(map[string]interface{})
	if !ok {
		operationParams = make(map[string]interface{})
	}

	result, err := ph.driverManager.ExecuteDriverOperation(ctx, instanceID, operation, operationParams)
	if err != nil {
		return ph.sendError(msg.ID, 500, err.Error())
	}

	response := Message{
		ID:     msg.ID,
		Type:   "response",
		Result: result,
	}
	return ph.sendMessage(response)
}

// sendMessage sends a message
func (ph *ProtocolHandler) sendMessage(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = ph.conn.Write(append(data, '\n'))
	return err
}

// sendError sends an error response
func (ph *ProtocolHandler) sendError(id string, code int, message string) error {
	response := Message{
		ID:   id,
		Type: "response",
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	}
	return ph.sendMessage(response)
}