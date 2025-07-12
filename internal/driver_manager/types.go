package driver_manager

import (
	"encoding/json"

	"github.com/OpenListTeam/OpenList/v4/internal/driver"
)

// Additional represents driver additional configuration
type Additional interface{}

// GenericAddition is a generic structure that can hold any driver configuration
// It implements the driver.Additional interface and can be used for remote drivers
type GenericAddition map[string]interface{}

// UnmarshalJSON implements json.Unmarshaler to handle database configuration
func (ga *GenericAddition) UnmarshalJSON(data []byte) error {
	if *ga == nil {
		*ga = make(map[string]interface{})
	}
	return json.Unmarshal(data, (*map[string]interface{})(ga))
}

// MarshalJSON implements json.Marshaler
func (ga GenericAddition) MarshalJSON() ([]byte, error) {
	if ga == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]interface{}(ga))
}

// GetConfig returns the configuration map
func (ga GenericAddition) GetConfig() map[string]interface{} {
	if ga == nil {
		return make(map[string]interface{})
	}
	return map[string]interface{}(ga)
}

// SetConfig sets the configuration map
func (ga *GenericAddition) SetConfig(config map[string]interface{}) {
	if *ga == nil {
		*ga = make(map[string]interface{})
	}
	for k, v := range config {
		(*ga)[k] = v
	}
}

// Ensure RemoteDriverAdapter implements all necessary interfaces
var _ driver.Driver = (*RemoteDriverAdapter)(nil)

// Optional interfaces that may be implemented by remote drivers
var _ driver.Getter = (*RemoteDriverAdapter)(nil)
var _ driver.Other = (*RemoteDriverAdapter)(nil)

// Helper function to check if a driver supports specific interfaces
func SupportsGetter(d driver.Driver) bool {
	_, ok := d.(driver.Getter)
	return ok
}

func SupportsOther(d driver.Driver) bool {
	_, ok := d.(driver.Other)
	return ok
}

func SupportsGetRooter(d driver.Driver) bool {
	_, ok := d.(driver.GetRooter)
	return ok
}

// DriverCapabilities represents the capabilities of a driver
type DriverCapabilities struct {
	SupportsGet   bool `json:"supports_get"`
	SupportsOther bool `json:"supports_other"`
	SupportsRoot  bool `json:"supports_root"`
}

// GetDriverCapabilities returns the capabilities of a driver
func GetDriverCapabilities(d driver.Driver) DriverCapabilities {
	return DriverCapabilities{
		SupportsGet:   SupportsGetter(d),
		SupportsOther: SupportsOther(d),
		SupportsRoot:  SupportsGetRooter(d),
	}
}