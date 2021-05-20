package scaffolding

import (
	"context"
	"time"

	router "github.com/torchcc/crank4go-core/crank4go-router-embedded"
	"github.com/torchcc/crank4go-core/util"
)

type RouterHealthService struct {
	version            string
	available          string
	isAvailable        bool
	cancelHealthCheck  context.CancelFunc
	routerAvailability *router.RouterAvailability
}

func NewRouterHealthService(routerAvailability *router.RouterAvailability) *RouterHealthService {
	return &RouterHealthService{
		version:            "N/A",
		available:          "isAvailable",
		routerAvailability: routerAvailability,
	}
}

func (r *RouterHealthService) CreateHealthReport() map[string]interface{} {
	embeddedRouterStatus := make(map[string]interface{})
	embeddedRouterStatus["crank4go-router-embedded"] = r.routerAvailability.Status()
	dependencies := make([]interface{}, 0, 8)
	dependencies = append(dependencies, embeddedRouterStatus)

	return map[string]interface{}{
		"component":    "crank4go-router",
		"description":  "Proxy end uses https calls and websocket calls",
		"version":      r.version,
		"service":      "/health/connectors",
		"git-url":      "https://github.com/",
		r.available:    r.isAvailable,
		"dependencies": dependencies,
	}
}

func (r *RouterHealthService) GetVersion() string {
	return r.version
}

func (r *RouterHealthService) GetAvailable() bool {
	return r.isAvailable
}

func (r *RouterHealthService) CreateConnectorsReport() map[string]interface{} {
	return map[string]interface{}{
		"services": r.routerAvailability.Services(),
	}
}

func (r *RouterHealthService) CreateCategorizedConnectorsReport() map[string]interface{} {
	m := make(map[string]interface{})
	m["services"] = r.routerAvailability.ServicesCategorizedDetail()
	return m
}

func (r *RouterHealthService) ScheduleHealthCheck() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancelHealthCheck = cancel
	util.LOG.Infof("Health Check scheduler started with 1 minute period")
	go func(ctx context.Context) {
	LOOP:
		for {
			r.updateHealth()
			select {
			case <-ctx.Done():
				break LOOP
			case <-time.After(time.Minute):
			}
		}
	}(ctx)
}

func (r *RouterHealthService) updateHealth() {
	embeddedRouterStatus := r.routerAvailability.Status()
	r.isAvailable = embeddedRouterStatus[r.available].(bool)
}
