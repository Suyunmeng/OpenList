package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/OpenListTeam/OpenList/v4/driver-manager/internal/registry"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

// DriverInstance represents an active driver instance
type DriverInstance struct {
	ID       string
	Name     string
	Driver   driver.Driver
	Config   map[string]interface{}
	Enabled  bool
	mutex    sync.RWMutex
}

// DriverManager manages driver instances
type DriverManager struct {
	registry  *registry.DriverRegistry
	instances map[string]*DriverInstance
	configs   map[string]DriverConfig
	mutex     sync.RWMutex
}

// DriverConfig represents configuration for a driver instance
type DriverConfig struct {
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// NewDriverManager creates a new driver manager
func NewDriverManager(registry *registry.DriverRegistry, configs map[string]DriverConfig) *DriverManager {
	return &DriverManager{
		registry:  registry,
		instances: make(map[string]*DriverInstance),
		configs:   configs,
	}
}

// CreateDriverInstance creates a new driver instance
func (dm *DriverManager) CreateDriverInstance(ctx context.Context, instanceID, driverName string, config map[string]interface{}) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	// Check if instance already exists
	if _, exists := dm.instances[instanceID]; exists {
		return fmt.Errorf("driver instance %s already exists", instanceID)
	}

	// Get driver info from registry
	driverInfo, err := dm.registry.GetDriver(driverName)
	if err != nil {
		return fmt.Errorf("failed to get driver info: %w", err)
	}

	// Create driver instance
	driverInstance := driverInfo.Constructor()

	// Create storage model for the driver
	storage := &model.Storage{
		ID:       0, // Will be set by OpenList
		MountPath: fmt.Sprintf("/driver-%s", instanceID),
		Driver:   driverName,
		Addition: config,
		Status:   "work",
		Modified: nil,
	}

	// Set storage in driver
	driverInstance.SetStorage(*storage)

	// Initialize driver
	if err := driverInstance.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize driver: %w", err)
	}

	// Create instance wrapper
	instance := &DriverInstance{
		ID:      instanceID,
		Name:    driverName,
		Driver:  driverInstance,
		Config:  config,
		Enabled: true,
	}

	dm.instances[instanceID] = instance

	return nil
}

// GetDriverInstance returns a driver instance by ID
func (dm *DriverManager) GetDriverInstance(instanceID string) (*DriverInstance, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	instance, exists := dm.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("driver instance %s not found", instanceID)
	}

	return instance, nil
}

// ListDriverInstances returns all driver instances
func (dm *DriverManager) ListDriverInstances() map[string]*DriverInstance {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	result := make(map[string]*DriverInstance)
	for id, instance := range dm.instances {
		result[id] = instance
	}

	return result
}

// RemoveDriverInstance removes a driver instance
func (dm *DriverManager) RemoveDriverInstance(ctx context.Context, instanceID string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	instance, exists := dm.instances[instanceID]
	if !exists {
		return fmt.Errorf("driver instance %s not found", instanceID)
	}

	// Drop the driver
	if err := instance.Driver.Drop(ctx); err != nil {
		return fmt.Errorf("failed to drop driver: %w", err)
	}

	delete(dm.instances, instanceID)

	return nil
}

// GetDriverRegistry returns the driver registry
func (dm *DriverManager) GetDriverRegistry() *registry.DriverRegistry {
	return dm.registry
}

// EnableDriverInstance enables a driver instance
func (dm *DriverManager) EnableDriverInstance(instanceID string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	instance, exists := dm.instances[instanceID]
	if !exists {
		return fmt.Errorf("driver instance %s not found", instanceID)
	}

	instance.mutex.Lock()
	defer instance.mutex.Unlock()

	instance.Enabled = true
	return nil
}

// DisableDriverInstance disables a driver instance
func (dm *DriverManager) DisableDriverInstance(instanceID string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	instance, exists := dm.instances[instanceID]
	if !exists {
		return fmt.Errorf("driver instance %s not found", instanceID)
	}

	instance.mutex.Lock()
	defer instance.mutex.Unlock()

	instance.Enabled = false
	return nil
}

// IsDriverInstanceEnabled checks if a driver instance is enabled
func (dm *DriverManager) IsDriverInstanceEnabled(instanceID string) bool {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	instance, exists := dm.instances[instanceID]
	if !exists {
		return false
	}

	instance.mutex.RLock()
	defer instance.mutex.RUnlock()

	return instance.Enabled
}

// ExecuteDriverOperation executes an operation on a driver instance
func (dm *DriverManager) ExecuteDriverOperation(ctx context.Context, instanceID string, operation string, params map[string]interface{}) (interface{}, error) {
	instance, err := dm.GetDriverInstance(instanceID)
	if err != nil {
		return nil, err
	}

	if !dm.IsDriverInstanceEnabled(instanceID) {
		return nil, fmt.Errorf("driver instance %s is disabled", instanceID)
	}

	// Execute operation based on type
	switch operation {
	case "list":
		return dm.executeListOperation(ctx, instance, params)
	case "link":
		return dm.executeLinkOperation(ctx, instance, params)
	case "get":
		return dm.executeGetOperation(ctx, instance, params)
	case "other":
		return dm.executeOtherOperation(ctx, instance, params)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// executeListOperation executes a list operation
func (dm *DriverManager) executeListOperation(ctx context.Context, instance *DriverInstance, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Create directory object
	dir := &model.Object{
		ID:   path,
		Name: path,
		Path: path,
		IsDir: true,
	}

	// Create list args
	args := model.ListArgs{}
	if refresh, ok := params["refresh"].(bool); ok {
		args.Refresh = refresh
	}

	// Execute list operation
	return instance.Driver.List(ctx, dir, args)
}

// executeLinkOperation executes a link operation
func (dm *DriverManager) executeLinkOperation(ctx context.Context, instance *DriverInstance, params map[string]interface{}) (interface{}, error) {
	// Extract parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Create file object
	file := &model.Object{
		ID:   path,
		Name: path,
		Path: path,
		IsDir: false,
	}

	// Create link args
	args := model.LinkArgs{}

	// Execute link operation
	return instance.Driver.Link(ctx, file, args)
}

// executeGetOperation executes a get operation
func (dm *DriverManager) executeGetOperation(ctx context.Context, instance *DriverInstance, params map[string]interface{}) (interface{}, error) {
	// Check if driver supports Get interface
	getter, ok := instance.Driver.(driver.Getter)
	if !ok {
		return nil, fmt.Errorf("driver does not support get operation")
	}

	// Extract parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Execute get operation
	return getter.Get(ctx, path)
}

// executeOtherOperation executes an other operation
func (dm *DriverManager) executeOtherOperation(ctx context.Context, instance *DriverInstance, params map[string]interface{}) (interface{}, error) {
	// Check if driver supports Other interface
	other, ok := instance.Driver.(driver.Other)
	if !ok {
		return nil, fmt.Errorf("driver does not support other operation")
	}

	// Create other args
	args := model.OtherArgs{
		Obj:    nil, // Will be set based on params
		Method: "",  // Will be set based on params
		Data:   params,
	}

	if method, ok := params["method"].(string); ok {
		args.Method = method
	}

	// Execute other operation
	return other.Other(ctx, args)
}