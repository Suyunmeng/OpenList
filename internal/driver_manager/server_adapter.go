package driver_manager

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
)

// RemoteDriverServerAdapter adapts remote driver calls through the server
type RemoteDriverServerAdapter struct {
	server     *DriverManagerServer
	instanceID string
	storage    *model.Storage
	config     driver.Config
	addition   driver.Additional
}

// NewRemoteDriverServerAdapter creates a new remote driver server adapter
func NewRemoteDriverServerAdapter(server *DriverManagerServer, instanceID string, config driver.Config) *RemoteDriverServerAdapter {
	return &RemoteDriverServerAdapter{
		server:     server,
		instanceID: instanceID,
		config:     config,
	}
}

// Config returns the driver configuration
func (rdsa *RemoteDriverServerAdapter) Config() driver.Config {
	return rdsa.config
}

// GetStorage returns the storage
func (rdsa *RemoteDriverServerAdapter) GetStorage() *model.Storage {
	return rdsa.storage
}

// SetStorage sets the storage
func (rdsa *RemoteDriverServerAdapter) SetStorage(storage model.Storage) {
	rdsa.storage = &storage
}

// GetAddition returns the addition configuration
func (rdsa *RemoteDriverServerAdapter) GetAddition() driver.Additional {
	if rdsa.addition == nil {
		// Create a generic addition structure that can hold any configuration
		rdsa.addition = &GenericAddition{}
	}
	return rdsa.addition
}

// Init initializes the remote driver
func (rdsa *RemoteDriverServerAdapter) Init(ctx context.Context) error {
	if rdsa.storage == nil {
		return fmt.Errorf("storage not set")
	}

	// Get the configuration from the addition that was unmarshaled from database
	var config map[string]interface{}
	if addition, ok := rdsa.addition.(*GenericAddition); ok && addition != nil {
		config = addition.GetConfig()
	} else {
		// Fallback: parse directly from storage.Addition if needed
		config = make(map[string]interface{})
		if rdsa.storage.Addition != "" {
			if err := json.Unmarshal([]byte(rdsa.storage.Addition), &config); err != nil {
				return fmt.Errorf("failed to parse addition from database: %w", err)
			}
		}
	}

	return rdsa.server.CreateDriverInstance(ctx, rdsa.instanceID, rdsa.storage.Driver, config)
}

// Drop drops the remote driver
func (rdsa *RemoteDriverServerAdapter) Drop(ctx context.Context) error {
	// Find the manager that has this instance and remove it
	managers := rdsa.server.GetConnectedManagers()
	for _, manager := range managers {
		params := map[string]interface{}{
			"instance_id": rdsa.instanceID,
		}

		_, err := manager.sendRequest(ctx, "remove_instance", params)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to remove driver instance from any manager")
}

// List lists files in the directory
func (rdsa *RemoteDriverServerAdapter) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	params := map[string]interface{}{
		"path":    dir.GetPath(),
		"refresh": args.Refresh,
	}

	result, err := rdsa.server.ExecuteDriverOperation(ctx, rdsa.instanceID, "list", params)
	if err != nil {
		return nil, err
	}

	// Convert result to []model.Obj
	return rdsa.convertToObjects(result)
}

// Link gets the download link for a file
func (rdsa *RemoteDriverServerAdapter) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	params := map[string]interface{}{
		"path": file.GetPath(),
	}

	result, err := rdsa.server.ExecuteDriverOperation(ctx, rdsa.instanceID, "link", params)
	if err != nil {
		return nil, err
	}

	// Convert result to *model.Link
	return rdsa.convertToLink(result)
}

// Get gets a file by path (if supported)
func (rdsa *RemoteDriverServerAdapter) Get(ctx context.Context, path string) (model.Obj, error) {
	params := map[string]interface{}{
		"path": path,
	}

	result, err := rdsa.server.ExecuteDriverOperation(ctx, rdsa.instanceID, "get", params)
	if err != nil {
		return nil, err
	}

	// Convert result to model.Obj
	return rdsa.convertToObject(result)
}

// Other executes other operations (if supported)
func (rdsa *RemoteDriverServerAdapter) Other(ctx context.Context, args model.OtherArgs) (interface{}, error) {
	params := map[string]interface{}{
		"method": args.Method,
		"data":   args.Data,
	}

	if args.Obj != nil {
		params["path"] = args.Obj.GetPath()
	}

	return rdsa.server.ExecuteDriverOperation(ctx, rdsa.instanceID, "other", params)
}

// convertToObjects converts interface{} to []model.Obj
func (rdsa *RemoteDriverServerAdapter) convertToObjects(result interface{}) ([]model.Obj, error) {
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
func (rdsa *RemoteDriverServerAdapter) convertToObject(result interface{}) (model.Obj, error) {
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
func (rdsa *RemoteDriverServerAdapter) convertToLink(result interface{}) (*model.Link, error) {
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