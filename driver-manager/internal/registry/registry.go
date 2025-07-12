package registry

import (
	"fmt"
	"sync"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
)

// DriverInfo contains information about a driver
type DriverInfo struct {
	Name        string                 `json:"name"`
	Config      driver.Config          `json:"config"`
	Items       []driver.Item          `json:"items"`
	I18n        map[string]interface{} `json:"i18n"`
	Constructor op.DriverConstructor   `json:"-"`
}

// DriverRegistry manages available drivers
type DriverRegistry struct {
	drivers map[string]*DriverInfo
	mutex   sync.RWMutex
}

// NewDriverRegistry creates a new driver registry
func NewDriverRegistry() *DriverRegistry {
	registry := &DriverRegistry{
		drivers: make(map[string]*DriverInfo),
	}
	
	// Load all available drivers from op package
	registry.loadDrivers()
	
	return registry
}

// loadDrivers loads all drivers from the op package
func (r *DriverRegistry) loadDrivers() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Get driver info map from op package
	driverInfoMap := op.GetDriverInfoMap()
	
	for name, info := range driverInfoMap {
		constructor, err := op.GetDriver(name)
		if err != nil {
			continue
		}

		// Combine common and additional items
		var allItems []driver.Item
		allItems = append(allItems, info.Common...)
		allItems = append(allItems, info.Additional...)

		driverInfo := &DriverInfo{
			Name:        name,
			Config:      info.Config,
			Items:       allItems,
			I18n:        r.generateI18n(name, allItems, info.Config),
			Constructor: constructor,
		}

		r.drivers[name] = driverInfo
	}
}

// generateI18n generates internationalization data for a driver
func (r *DriverRegistry) generateI18n(driverName string, items []driver.Item, config driver.Config) map[string]interface{} {
	i18n := make(map[string]interface{})
	
	// Create English and Chinese translations
	enUS := make(map[string]interface{})
	zhCN := make(map[string]interface{})

	// Driver name translation
	enUS["driver_name"] = driverName
	zhCN["driver_name"] = driverName // You can add Chinese translations here

	// Alert message if exists
	if config.Alert != "" {
		enUS["alert"] = config.Alert
		zhCN["alert"] = config.Alert
	}

	// Item translations
	for _, item := range items {
		// Field name
		enUS[item.Name] = r.convertToDisplayName(item.Name)
		zhCN[item.Name] = r.convertToDisplayName(item.Name) // Add Chinese translations as needed

		// Help text
		if item.Help != "" {
			enUS[item.Name+"_help"] = item.Help
			zhCN[item.Name+"_help"] = item.Help
		}

		// Options for select type
		if item.Type == "select" && item.Options != "" {
			options := make(map[string]string)
			for _, option := range parseOptions(item.Options) {
				options[option] = r.convertToDisplayName(option)
			}
			enUS[item.Name+"_options"] = options
			zhCN[item.Name+"_options"] = options
		}
	}

	i18n["en_us"] = enUS
	i18n["zh_cn"] = zhCN

	return i18n
}

// convertToDisplayName converts snake_case to display name
func (r *DriverRegistry) convertToDisplayName(name string) string {
	// Simple conversion: replace underscores with spaces and capitalize first letter
	result := ""
	capitalize := true
	
	for _, char := range name {
		if char == '_' {
			result += " "
			capitalize = true
		} else if capitalize {
			result += string(char - 32) // Convert to uppercase
			capitalize = false
		} else {
			result += string(char)
		}
	}
	
	return result
}

// parseOptions parses comma-separated options string
func parseOptions(options string) []string {
	if options == "" {
		return nil
	}
	
	var result []string
	current := ""
	
	for _, char := range options {
		if char == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		result = append(result, current)
	}
	
	return result
}

// GetDriver returns a driver by name
func (r *DriverRegistry) GetDriver(name string) (*DriverInfo, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	driver, exists := r.drivers[name]
	if !exists {
		return nil, fmt.Errorf("driver %s not found", name)
	}

	return driver, nil
}

// ListDrivers returns all available drivers
func (r *DriverRegistry) ListDrivers() map[string]*DriverInfo {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]*DriverInfo)
	for name, driver := range r.drivers {
		result[name] = driver
	}

	return result
}

// GetDriverNames returns all driver names
func (r *DriverRegistry) GetDriverNames() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var names []string
	for name := range r.drivers {
		names = append(names, name)
	}

	return names
}

// GetDriverCount returns the number of available drivers
func (r *DriverRegistry) GetDriverCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return len(r.drivers)
}