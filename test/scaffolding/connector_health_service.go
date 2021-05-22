package scaffolding

import (
	"context"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	ptc "github.com/torchcc/crank4go/protocol"
	"github.com/torchcc/crank4go/router/api"
	"github.com/torchcc/crank4go/router/handler"
	"github.com/torchcc/crank4go/util"
)

type HealthService interface {
	CreateHealthReport() map[string]interface{}
	GetVersion() string
	GetAvailable() bool
}

type HealthServiceResource struct {
	basePath      string
	healthService HealthService
}

func NewHealthServiceResource(healthService HealthService) *HealthServiceResource {
	return &HealthServiceResource{
		healthService: healthService,
		basePath:      "/health",
	}
}

// @Path("/health")
func (h *HealthServiceResource) GetHealthInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	api.RespJsonOk(w, h.healthService.CreateHealthReport())
	return true
}

func (h *HealthServiceResource) RegisterResourceToHttpRouter(server *httprouter.Router, rootPath string) {
	basePath := rootPath + h.basePath
	server.GET(basePath, handler.NewXHttpHandler(handler.XHandlerFunc(h.GetHealthInfo)).ServeXHTTP)
}

type ConnectorHealthService struct {
	version                      string
	cancelHealthCheck            context.CancelFunc
	connectorToRouterConnMonitor *util.ConnectionMonitor
	isAvailable                  bool
}

func NewConnectorHealthService(connectorToRouterConnMonitor *util.ConnectionMonitor) *ConnectorHealthService {
	return &ConnectorHealthService{
		version:                      "N/A",
		connectorToRouterConnMonitor: connectorToRouterConnMonitor,
		isAvailable:                  false,
	}
}

func (c *ConnectorHealthService) CreateHealthReport() map[string]interface{} {
	return map[string]interface{}{
		"component":              "crank4go-connector",
		"description":            "open connections to crank4go-router, and pass tunneled requests to target service",
		"version":                c.version,
		"git-url":                "https://github.com",
		"isAvailable":            c.isAvailable,
		"CrankerProtocolVersion": ptc.CrankerProtocolVersion10,
		"activeConnections":      c.connectorToRouterConnMonitor.ConnectionCount(),
		"openFiles":              c.connectorToRouterConnMonitor.OpenFiles(),
	}
}

func (c *ConnectorHealthService) GetVersion() string {
	return c.version
}

func (c *ConnectorHealthService) GetAvailable() bool {
	return c.isAvailable
}

func (c *ConnectorHealthService) ScheduleHealthCheck() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelHealthCheck = cancel
	util.LOG.Infof("Health Check scheduler started with 1 minute period")
	go func(ctx context.Context) {
	LOOP:
		for {
			c.updateHealth()
			select {
			case <-ctx.Done():
				break LOOP
			case <-time.After(time.Minute):
			}
		}
	}(ctx)
}

func (c *ConnectorHealthService) updateHealth() {
	c.isAvailable = true
}
