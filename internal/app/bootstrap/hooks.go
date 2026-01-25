// internal/app/bootstrap/hooks.go
package bootstrap

import (
	"github.com/dalemusser/waffle/app"
)

// Hooks wires this app into the WAFFLE lifecycle.
// Each function is called in order by app.Run, from configuration
// loading through DB setup, one-time startup work, HTTP handler
// construction, and finally graceful shutdown.
//
// Only LoadConfig, ConnectDB, and BuildHandler are strictly required;
// the others are optional and may be nil if the app does not need them.
var Hooks = app.Hooks[AppConfig, DBDeps]{
	Name:           "stratasave", // used only for logging/diagnostics
	LoadConfig:     LoadConfig,    // load core + app config
	ValidateConfig: ValidateConfig, // validate MongoDB URI and other settings
	ConnectDB:      ConnectDB,     // connect to MongoDB and return DBDeps
	EnsureSchema:   EnsureSchema,  // create indexes
	Startup:        Startup,       // load shared templates, seed admin
	BuildHandler:   BuildHandler,  // build the HTTP router + middleware stack
	Shutdown:       Shutdown,      // disconnect MongoDB on shutdown
}
