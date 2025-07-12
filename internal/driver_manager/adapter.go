package driver_manager

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

// RemoteDriverAdapter adapts remote driver calls to the local driver interface
type RemoteDriverAdapter struct {
	client     *DriverManagerClient
	instanceID string
	storage    *model.Storage
	config     driver.Config
	addition   driver.Additional
}

// NewRemoteDriverAdapter creates a new remote driver adapter
func NewRemoteDriverAdapter(client *DriverManagerClient, instanceID string, config driver.Config) *RemoteDriverAdapter {
	return &RemoteDriverAdapter{
		client:     client,
		instanceID: instanceID,
		config:     config,
	}
}

// Config returns the driver configuration
func (rda *RemoteDriverAdapter) Config() driver.Config {
	return rda.config
}

// GetStorage returns the storage
func (rda *RemoteDriverAdapter) GetStorage() *model.Storage {
	return rda.storage
}

// SetStorage sets the storage
func (rda *RemoteDriverAdapter) SetStorage(storage model.Storage) {
	rda.storage = &storage
}

// GetAddition returns the addition configuration
func (rda *RemoteDriverAdapter) GetAddition() driver.Additional {
	if rda.addition == nil {
		// Create a generic addition structure that can hold any configuration
		rda.addition = &GenericAddition{}
	}
	return rda.addition
}

// Init initializes the remote driver
func (rda *RemoteDriverAdapter) Init(ctx context.Context) error {
	if rda.storage == nil {
		return fmt.Errorf("storage not set")
	}

	// Get the configuration from the addition that was unmarshaled from database
	var config map[string]interface{}
	if addition, ok := rda.addition.(*GenericAddition); ok && addition != nil {
		config = addition.GetConfig()
	} else {
		// Fallback: parse directly from storage.Addition if needed
		config = make(map[string]interface{})
		if rda.storage.Addition != "" {
			if err := json.Unmarshal([]byte(rda.storage.Addition), &config); err != nil {
				return fmt.Errorf("failed to parse addition from database: %w", err)
			}
		}
	}

	return rda.client.CreateDriverInstance(ctx, rda.instanceID, rda.storage.Driver, config)
}

// Drop drops the remote driver
func (rda *RemoteDriverAdapter) Drop(ctx context.Context) error {
	params := map[string]interface{}{
		"instance_id": rda.instanceID,
	}

	_, err := rda.client.sendRequest(ctx, "remove_instance", params)
	return err
}

// List lists files in the directory
func (rda *RemoteDriverAdapter) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	params := map[string]interface{}{
		"path":    dir.GetPath(),
		"refresh": args.Refresh,
	}

	result, err := rda.client.ExecuteDriverOperation(ctx, rda.instanceID, "list", params)
	if err != nil {
		return nil, err
	}

	// Convert result to []model.Obj
	return rda.convertToObjects(result)
}

// Link gets the download link for a file
func (rda *RemoteDriverAdapter) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	params := map[string]interface{}{
		"path": file.GetPath(),
	}

	result, err := rda.client.ExecuteDriverOperation(ctx, rda.instanceID, "link", params)
	if err != nil {
		return nil, err
	}

	// Convert result to *model.Link
	return rda.convertToLink(result)
}

// Get gets a file by path (if supported)
func (rda *RemoteDriverAdapter) Get(ctx context.Context, path string) (model.Obj, error) {
	params := map[string]interface{}{
		"path": path,
	}

	result, err := rda.client.ExecuteDriverOperation(ctx, rda.instanceID, "get", params)
	if err != nil {
		return nil, err
	}

	// Convert result to model.Obj
	return rda.convertToObject(result)
}

// Other executes other operations (if supported)
func (rda *RemoteDriverAdapter) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	params := map[string]interface{}{
		"method": args.Method,
		"data":   args.Data,
	}

	if args.Obj != nil {
		params["path"] = args.Obj.GetPath()
	}

	return rda.client.ExecuteDriverOperation(ctx, rda.instanceID, "other", params)
}

// convertToObjects converts interface{} to []model.Obj
func (rda *RemoteDriverAdapter) convertToObjects(result interface{}) ([]model.Obj, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var objects []model.Object
	if err := json.Unmarshal(data, &objects); err != nil {
		return nil, fmt.Errorf("failed to unmarshal objects: %w", err)
	}

	// Convert to []model.Obj
	var result_objs []model.Obj
	for i := range objects {
		result_objs = append(result_objs, &objects[i])
	}

	return result_objs, nil
}

// convertToObject converts interface{} to model.Obj
func (rda *RemoteDriverAdapter) convertToObject(result interface{}) (model.Obj, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var object model.Object
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %w", err)
	}

	return &object, nil
}

// convertToLink converts interface{} to *model.Link
func (rda *RemoteDriverAdapter) convertToLink(result interface{}) (*model.Link, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var link model.Link
	if err := json.Unmarshal(data, &link); err != nil {
		return nil, fmt.Errorf("failed to unmarshal link: %w", err)
	}

	return &link, nil
}

// RemoteDriverFactory creates remote driver adapters
type RemoteDriverFactory struct {
	pool   *DriverManagerPool
	server *DriverManagerServer
}

// NewRemoteDriverFactory creates a new remote driver factory with pool
func NewRemoteDriverFactory(pool *DriverManagerPool) *RemoteDriverFactory {
	return &RemoteDriverFactory{
		pool: pool,
	}
}

// NewRemoteDriverServerFactory creates a new remote driver factory with server
func NewRemoteDriverServerFactory(server *DriverManagerServer) *RemoteDriverFactory {
	return &RemoteDriverFactory{
		server: server,
	}
}

// CreateDriver creates a remote driver adapter for the given storage
func (rdf *RemoteDriverFactory) CreateDriver(storage *model.Storage) (driver.Driver, error) {
	ctx := context.Background()
	
	if rdf.server != nil {
		// Use server-based approach
		return rdf.createDriverWithServer(ctx, storage)
	} else if rdf.pool != nil {
		// Use pool-based approach (legacy)
		return rdf.createDriverWithPool(ctx, storage)
	}
	
	return nil, fmt.Errorf("no driver manager available")
}

// createDriverWithServer creates driver using server
func (rdf *RemoteDriverFactory) createDriverWithServer(ctx context.Context, storage *model.Storage) (driver.Driver, error) {
	managers := rdf.server.GetConnectedManagers()
	if len(managers) == 0 {
		return nil, fmt.Errorf("no connected driver managers")
	}

	// Try to find the driver in any manager
	var selectedManager *DriverManagerConnection
	var driverConfig driver.Config

	for _, manager := range managers {
		if !manager.IsConnected() {
			continue
		}

		driverInfo, err := rdf.server.GetDriverInfo(ctx, storage.Driver)
		if err != nil {
			continue
		}

		// Parse driver config
		configData, err := json.Marshal(driverInfo["config"])
		if err != nil {
			continue
		}

		if err := json.Unmarshal(configData, &driverConfig); err != nil {
			continue
		}

		selectedManager = manager
		break
	}

	if selectedManager == nil {
		return nil, fmt.Errorf("driver %s not found in any connected manager", storage.Driver)
	}

	// Create instance ID
	instanceID := fmt.Sprintf("storage-%d", storage.ID)

	// Create adapter with server connection
	adapter := NewRemoteDriverServerAdapter(rdf.server, instanceID, driverConfig)
	adapter.SetStorage(*storage)

	return adapter, nil
}

// createDriverWithPool creates driver using pool (legacy)
func (rdf *RemoteDriverFactory) createDriverWithPool(ctx context.Context, storage *model.Storage) (driver.Driver, error) {
	managers := rdf.pool.GetConnectedManagers()
	if len(managers) == 0 {
		return nil, fmt.Errorf("no connected driver managers")
	}

	// Try to find the driver in any manager
	var selectedManager *DriverManagerClient
	var driverConfig driver.Config

	for i := range managers {
		manager := &managers[i]
		if !manager.IsConnected() {
			continue
		}

		driverInfo, err := manager.GetDriverInfo(ctx, storage.Driver)
		if err != nil {
			continue
		}

		// Parse driver config
		configData, err := json.Marshal(driverInfo["config"])
		if err != nil {
			continue
		}

		if err := json.Unmarshal(configData, &driverConfig); err != nil {
			continue
		}

		selectedManager = manager
		break
	}

	if selectedManager == nil {
		return nil, fmt.Errorf("driver %s not found in any connected manager", storage.Driver)
	}

	// Create instance ID
	instanceID := fmt.Sprintf("storage-%d", storage.ID)

	// Create adapter
	adapter := NewRemoteDriverAdapter(selectedManager, instanceID, driverConfig)
	adapter.SetStorage(*storage)

	return adapter, nil
}