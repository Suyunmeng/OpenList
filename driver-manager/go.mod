module github.com/OpenListTeam/OpenList/v4/driver-manager

go 1.21

replace github.com/OpenListTeam/OpenList/v4 => ../

require (
	github.com/OpenListTeam/OpenList/v4 v4.0.0-00010101000000-000000000000
)

// Include all the same dependencies as the main module
require (
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3
)