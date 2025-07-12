package op

import (
	"context"
	"reflect"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/driver_manager"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/pkg/utils"
	"github.com/pkg/errors"
)

type DriverConstructor func() driver.Driver

var driverMap = map[string]DriverConstructor{}
var driverInfoMap = map[string]driver.Info{}
var remoteDriverFactory *driver_manager.RemoteDriverFactory

func RegisterDriver(driver DriverConstructor) {
	// log.Infof("register driver: [%s]", config.Name)
	tempDriver := driver()
	tempConfig := tempDriver.Config()
	registerDriverItems(tempConfig, tempDriver.GetAddition())
	driverMap[tempConfig.Name] = driver
}

func GetDriver(name string) (DriverConstructor, error) {
	n, ok := driverMap[name]
	if !ok {
		return nil, errors.Errorf("no driver named: %s", name)
	}
	return n, nil
}

func GetDriverNames() []string {
	var driverNames []string
	for k := range driverInfoMap {
		driverNames = append(driverNames, k)
	}
	return driverNames
}

func GetDriverInfoMap() map[string]driver.Info {
	return driverInfoMap
}

func registerDriverItems(config driver.Config, addition driver.Additional) {
	// log.Debugf("addition of %s: %+v", config.Name, addition)
	tAddition := reflect.TypeOf(addition)
	for tAddition.Kind() == reflect.Pointer {
		tAddition = tAddition.Elem()
	}
	mainItems := getMainItems(config)
	additionalItems := getAdditionalItems(tAddition, config.DefaultRoot)
	driverInfoMap[config.Name] = driver.Info{
		Common:     mainItems,
		Additional: additionalItems,
		Config:     config,
	}
}

func getMainItems(config driver.Config) []driver.Item {
	items := []driver.Item{{
		Name:     "mount_path",
		Type:     conf.TypeString,
		Required: true,
		Help:     "The path you want to mount to, it is unique and cannot be repeated",
	}, {
		Name: "order",
		Type: conf.TypeNumber,
		Help: "use to sort",
	}, {
		Name: "remark",
		Type: conf.TypeText,
	}}
	if !config.NoCache {
		items = append(items, driver.Item{
			Name:     "cache_expiration",
			Type:     conf.TypeNumber,
			Default:  "30",
			Required: true,
			Help:     "The cache expiration time for this storage",
		})
	}
	if !config.OnlyProxy && !config.OnlyLocal {
		items = append(items, []driver.Item{{
			Name: "web_proxy",
			Type: conf.TypeBool,
		}, {
			Name:     "webdav_policy",
			Type:     conf.TypeSelect,
			Options:  "302_redirect,use_proxy_url,native_proxy",
			Default:  "302_redirect",
			Required: true,
		},
		}...)
		if config.ProxyRangeOption {
			item := driver.Item{
				Name: "proxy_range",
				Type: conf.TypeBool,
				Help: "Need to enable proxy",
			}
			if config.Name == "139Yun" {
				item.Default = "true"
			}
			items = append(items, item)
		}
	} else {
		items = append(items, driver.Item{
			Name:     "webdav_policy",
			Type:     conf.TypeSelect,
			Default:  "native_proxy",
			Options:  "use_proxy_url,native_proxy",
			Required: true,
		})
	}
	items = append(items, driver.Item{
		Name: "down_proxy_url",
		Type: conf.TypeText,
	})
	if config.LocalSort {
		items = append(items, []driver.Item{{
			Name:    "order_by",
			Type:    conf.TypeSelect,
			Options: "name,size,modified",
		}, {
			Name:    "order_direction",
			Type:    conf.TypeSelect,
			Options: "asc,desc",
		}}...)
	}
	items = append(items, driver.Item{
		Name:    "extract_folder",
		Type:    conf.TypeSelect,
		Options: "front,back",
	})
	items = append(items, driver.Item{
		Name:     "disable_index",
		Type:     conf.TypeBool,
		Default:  "false",
		Required: true,
	})
	items = append(items, driver.Item{
		Name:     "enable_sign",
		Type:     conf.TypeBool,
		Default:  "false",
		Required: true,
	})
	return items
}
func getAdditionalItems(t reflect.Type, defaultRoot string) []driver.Item {
	var items []driver.Item
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() == reflect.Struct {
			items = append(items, getAdditionalItems(field.Type, defaultRoot)...)
			continue
		}
		tag := field.Tag
		ignore, ok1 := tag.Lookup("ignore")
		name, ok2 := tag.Lookup("json")
		if (ok1 && ignore == "true") || !ok2 {
			continue
		}
		item := driver.Item{
			Name:     name,
			Type:     strings.ToLower(field.Type.Name()),
			Default:  tag.Get("default"),
			Options:  tag.Get("options"),
			Required: tag.Get("required") == "true",
			Help:     tag.Get("help"),
		}
		if tag.Get("type") != "" {
			item.Type = tag.Get("type")
		}
		if item.Name == "root_folder_id" || item.Name == "root_folder_path" {
			if item.Default == "" {
				item.Default = defaultRoot
			}
			item.Required = item.Default != ""
		}
		// set default type to string
		if item.Type == "" {
			item.Type = "string"
		}
		items = append(items, item)
	}
	return items
}

// InitDriverManagerServer initializes the driver manager server
func InitDriverManagerServer() {
	server := driver_manager.GetDriverManagerServer()
	
	// Start the server on port 5245 (OpenList port + 1)
	address := "localhost:5245"
	if err := server.Start(address); err != nil {
		utils.Log.Errorf("Failed to start driver manager server: %v", err)
		return
	}
	
	// Initialize remote driver factory
	remoteDriverFactory = driver_manager.NewRemoteDriverServerFactory(server)
	
	utils.Log.Infof("Driver Manager Server started on %s", address)
}

// GetAllDriversFromManagers gets all drivers from connected driver managers
func GetAllDriversFromManagers(ctx context.Context) (map[string]interface{}, error) {
	if remoteDriverFactory == nil {
		return make(map[string]interface{}), nil
	}
	
	pool := driver_manager.GetDriverManagerPool()
	return pool.GetAllDrivers(ctx)
}

// GetDriverInfoFromManagers gets driver info from connected driver managers
func GetDriverInfoFromManagers(ctx context.Context, driverName string) (map[string]interface{}, error) {
	if remoteDriverFactory == nil {
		return nil, errors.Errorf("driver manager not initialized")
	}
	
	pool := driver_manager.GetDriverManagerPool()
	return pool.GetDriverInfo(ctx, driverName)
}

// CreateDriverFromStorage creates a driver instance from storage configuration
// This function will try local drivers first, then remote drivers
func CreateDriverFromStorage(storage *model.Storage) (driver.Driver, error) {
	// Try local drivers first
	constructor, err := GetDriver(storage.Driver)
	if err == nil {
		return constructor(), nil
	}

	// Try remote drivers if local driver not found
	if remoteDriverFactory != nil {
		return remoteDriverFactory.CreateDriver(storage)
	}

	return nil, errors.Errorf("driver %s not found in local or remote drivers", storage.Driver)
}

// GetCombinedDriverNames returns driver names from both local and remote sources
func GetCombinedDriverNames(ctx context.Context) []string {
	// Get local driver names
	localNames := GetDriverNames()
	
	// Get remote driver names
	var remoteNames []string
	if remoteDriverFactory != nil {
		pool := driver_manager.GetDriverManagerPool()
		if drivers, err := pool.GetAllDrivers(ctx); err == nil {
			for name := range drivers {
				remoteNames = append(remoteNames, name)
			}
		}
	}
	
	// Combine and deduplicate
	nameSet := make(map[string]bool)
	var allNames []string
	
	for _, name := range localNames {
		if !nameSet[name] {
			nameSet[name] = true
			allNames = append(allNames, name)
		}
	}
	
	for _, name := range remoteNames {
		if !nameSet[name] {
			nameSet[name] = true
			allNames = append(allNames, name)
		}
	}
	
	return allNames
}

// GetCombinedDriverInfoMap returns driver info from both local and remote sources
func GetCombinedDriverInfoMap(ctx context.Context) map[string]driver.Info {
	// Start with local drivers
	combined := make(map[string]driver.Info)
	for name, info := range driverInfoMap {
		combined[name] = info
	}
	
	// Add remote drivers (they won't override local ones due to the check above)
	if remoteDriverFactory != nil {
		pool := driver_manager.GetDriverManagerPool()
		if drivers, err := pool.GetAllDrivers(ctx); err == nil {
			for name, driverData := range drivers {
				if _, exists := combined[name]; !exists {
					// Convert remote driver data to driver.Info
					// This is a simplified conversion - you might need to adjust based on actual data structure
					if info, ok := convertRemoteDriverInfo(driverData); ok {
						combined[name] = info
					}
				}
			}
		}
	}
	
	return combined
}

// convertRemoteDriverInfo converts remote driver data to driver.Info
func convertRemoteDriverInfo(data interface{}) (driver.Info, bool) {
	// This is a placeholder implementation
	// You'll need to implement the actual conversion based on the remote driver data structure
	return driver.Info{}, false
}
