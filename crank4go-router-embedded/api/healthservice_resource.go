package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/service"
)

const healthResourceBasePath string = "/health"

type HealthResource struct {
	basePath      string
	healthService service.HealthService
	*Filter
}

func NewHealthResource(healthService service.HealthService) *HealthResource {
	return &HealthResource{
		healthService: healthService,
		basePath:      healthResourceBasePath,
		Filter:        &Filter{},
	}
}

// basePath
func (h *HealthResource) GetHealthInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	RespJsonOk(w, h.healthService.CreateHealthReport())
	return true
}

// @Path("/connectors")
func (h *HealthResource) GetConnectorsInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	RespJsonOk(w, h.healthService.CreateConnectorsReport())
	return true
}

// @Path("/categorizedConnectors")
func (h *HealthResource) GetCategorizedConnectorsInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	RespJsonOk(w, h.healthService.CreateCategorizedConnectorsReport())
	return true
}

func (h *HealthResource) RegisterResourceToHttpServer(server *httprouter.Router, rootPath string) {
	basePath := rootPath + h.basePath
	server.GET(basePath, h.convertToHttpRouterHandlerWithFilters(h.GetHealthInfo))
	server.GET(basePath+"/connectors", h.convertToHttpRouterHandlerWithFilters(h.GetConnectorsInfo))
	server.GET(basePath+"/categorizedConnectors", h.convertToHttpRouterHandlerWithFilters(h.GetCategorizedConnectorsInfo))
}

type HealthServiceResource2 struct {
	basePath      string
	connectorPath string
	healthService service.HealthService
	*Filter
}

func NewHealthServiceResource2(healthService service.HealthService) *HealthServiceResource2 {
	return &HealthServiceResource2{
		healthService: healthService,
		basePath:      healthResourceBasePath,
		connectorPath: "/connectors",
		Filter:        &Filter{},
	}
}

func (h *HealthServiceResource2) GetHealthInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	RespJsonOk(w, h.healthService.CreateHealthReport())
	return true
}

// @Path("/connectors")
func (h *HealthServiceResource2) GetConnectorsInfo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	RespJsonOk(w, h.healthService.CreateConnectorsReport())
	return true
}

func (h *HealthServiceResource2) RegisterResourceToHttpRouter(server *httprouter.Router, rootPath string) {
	basePath := rootPath + h.basePath
	server.GET(basePath, h.convertToHttpRouterHandlerWithFilters(h.GetHealthInfo))
	server.GET(basePath+h.connectorPath, h.convertToHttpRouterHandlerWithFilters(h.GetConnectorsInfo))
}
