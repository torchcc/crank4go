package api

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	MediaType "github.com/torchcc/crank4go/router/api/media_type"
	"github.com/torchcc/crank4go/router/darklaunch_manager"
	"github.com/torchcc/crank4go/router/handler"
	"github.com/torchcc/crank4go/util"
)

const serviceResourceBasePath string = "/dark-launch/service"

type DarkLaunchServiceResource struct {
	basePath          string
	darkLaunchManager *darklaunch_manager.DarkLaunchManager
	*Filter
}

func NewDarkLaunchServiceResource(manager *darklaunch_manager.DarkLaunchManager) *DarkLaunchServiceResource {
	return &DarkLaunchServiceResource{
		basePath:          serviceResourceBasePath,
		darkLaunchManager: manager,
		Filter:            &Filter{},
	}
}

func (d *DarkLaunchServiceResource) GetDarkServices(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	RespTextPlainOk(w, fmt.Sprintf("DarkMode Services = %v", d.darkLaunchManager.ServiceList()))
	return true
}

// @Path("/{service}")
func (d *DarkLaunchServiceResource) GetDarkModeByHost(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool {
	service := params.ByName("service")
	RespTextPlainOk(w, fmt.Sprintf("DarkMode = %v for service = %s", d.darkLaunchManager.ContainsService(service), service))
	return true
}

// @Path("/{service}")
func (d *DarkLaunchServiceResource) PutEnableDarkModeByService(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool {
	service := params.ByName("service")
	if err := d.darkLaunchManager.AddService(service); err != nil {
		d.errorHandle(w, r, service, "Add service")
	} else {
		util.LOG.Infof("darkLaunch update: true, action: Add service, service: %s", service)
		RespTextPlainOk(w, "update dark launch manager successfully")
	}
	return true
}

// @Path("/{service}")
func (d *DarkLaunchServiceResource) DeleteDarkModeByService(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool {
	service := params.ByName("service")
	if err := d.darkLaunchManager.RemoveService(service); err != nil {
		d.errorHandle(w, r, service, "Remove service")
	} else {
		util.LOG.Infof("darkLaunch update: true, action: Remove service, service: %s", service)
		RespTextPlainOk(w, fmt.Sprintf("service: %s was deleted successfully from dark launch manager", service))
	}
	return true
}

func (d *DarkLaunchServiceResource) errorHandle(respWriter http.ResponseWriter, req *http.Request, service, action string) {
	errorID := uuid.New().String()
	util.LOG.Warningf("Receive invalid service: %s, action: %s, errorID: %s", service, action, errorID)
	respWriter.Header().Add("Content-Type", MediaType.TextPlain)
	respWriter.WriteHeader(http.StatusBadRequest)
	_, _ = respWriter.Write([]byte(fmt.Sprintf("Invalid request, invalid service: %s, action: %s, ErrorID: %s", service, action, errorID)))
}

func (d *DarkLaunchServiceResource) RegisterResourceToHttpRouter(httpRouter *httprouter.Router, rootPath string) {
	basePath := rootPath + d.basePath
	httpRouter.GET(basePath, d.convertToHttpRouterHandlerWithFilters(d.GetDarkServices))
	httpRouter.GET(basePath+"/:service", d.convertToHttpRouterHandlerWithFilters(d.GetDarkModeByHost))
	httpRouter.PUT(basePath+"/:service", d.convertToHttpRouterHandlerWithFilters(d.PutEnableDarkModeByService))
	httpRouter.DELETE(basePath+"/:service", d.convertToHttpRouterHandlerWithFilters(d.DeleteDarkModeByService))
}

type Filter struct {
	reqFilters  []handler.XHandler
	respFilters []handler.XHandler
}

func (f *Filter) RespFilters() []handler.XHandler {
	return f.respFilters
}

func (f *Filter) ReqFilters() []handler.XHandler {
	return f.reqFilters
}

func (f *Filter) AddReqFilters(handlers ...handler.XHandler) *Filter {
	if f.reqFilters == nil {
		f.reqFilters = make([]handler.XHandler, 0, 8)
	}
	for _, h := range handlers {
		f.reqFilters = append(f.reqFilters, h)
	}
	return f
}

func (f *Filter) AddRespFilters(handlers ...handler.XHandler) *Filter {
	if f.respFilters == nil {
		f.respFilters = make([]handler.XHandler, 0, 8)
	}
	for _, h := range handlers {
		f.respFilters = append(f.respFilters, h)
	}
	return f
}

func (f *Filter) convertToHttpRouterHandlerWithFilters(function func(http.ResponseWriter, *http.Request, httprouter.Params) bool) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return handler.NewXHttpHandler(handler.XHandlerFunc(function)).
		AddReqHandlers(f.ReqFilters()...).
		AddRespHandlers(f.RespFilters()...).
		ServeXHTTP
}
