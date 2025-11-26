// internal/app/system/handler/handler.go
package handler

import (
	"github.com/dalemusser/stratasave/internal/app/system/config"
	"go.mongodb.org/mongo-driver/mongo"
)

// Handler holds references to config, DB clients, etc.
type Handler struct {
	Cfg    *config.Config
	Client *mongo.Client
}

// NewHandler returns a new routes Handler with config and db client
func NewHandler(cfg *config.Config, client *mongo.Client) *Handler {
	return &Handler{Cfg: cfg, Client: client}
}
