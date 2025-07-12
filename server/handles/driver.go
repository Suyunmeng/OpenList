package handles

import (
	"context"
	"fmt"

	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/OpenListTeam/OpenList/v4/server/common"
	"github.com/gin-gonic/gin"
)

func ListDriverInfo(c *gin.Context) {
	ctx := context.Background()
	
	// Get combined driver info from both local and remote sources
	combinedInfo := op.GetCombinedDriverInfoMap(ctx)
	common.SuccessResp(c, combinedInfo)
}

func ListDriverNames(c *gin.Context) {
	ctx := context.Background()
	
	// Get combined driver names from both local and remote sources
	combinedNames := op.GetCombinedDriverNames(ctx)
	common.SuccessResp(c, combinedNames)
}

func GetDriverInfo(c *gin.Context) {
	ctx := context.Background()
	driverName := c.Query("driver")
	
	// Try to get from local drivers first
	localInfoMap := op.GetDriverInfoMap()
	if items, ok := localInfoMap[driverName]; ok {
		common.SuccessResp(c, items)
		return
	}
	
	// Try to get from remote driver managers
	remoteInfo, err := op.GetDriverInfoFromManagers(ctx, driverName)
	if err == nil {
		common.SuccessResp(c, remoteInfo)
		return
	}
	
	common.ErrorStrResp(c, fmt.Sprintf("driver [%s] not found", driverName), 404)
}

// ListDriverManagers lists all connected driver managers
func ListDriverManagers(c *gin.Context) {
	// This would require additional implementation to track manager connections
	// For now, return empty list
	common.SuccessResp(c, []interface{}{})
}

// GetDriverManagerInfo gets information about driver managers
func GetDriverManagerInfo(c *gin.Context) {
	ctx := context.Background()
	
	// Get all drivers from managers
	drivers, err := op.GetAllDriversFromManagers(ctx)
	if err != nil {
		common.ErrorResp(c, err, 500)
		return
	}
	
	result := map[string]interface{}{
		"driver_count": len(drivers),
		"drivers":      drivers,
	}
	
	common.SuccessResp(c, result)
}
