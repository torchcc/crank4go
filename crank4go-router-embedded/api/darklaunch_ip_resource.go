package api

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	MediaType "github.com/torchcc/crank4go-core/crank4go-router-embedded/api/media_type"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/darklaunch_manager"
	"github.com/torchcc/crank4go-core/util"
)

const ipResourceBasePath string = "/dark-launch/ip"

type DarkLaunchIpResource struct {
	basePath          string
	darkLaunchManager *darklaunch_manager.DarkLaunchManager
	*Filter
}

func NewDarkLaunchIpResource(manager *darklaunch_manager.DarkLaunchManager) *DarkLaunchIpResource {
	return &DarkLaunchIpResource{
		basePath:          ipResourceBasePath,
		darkLaunchManager: manager,
		Filter:            &Filter{},
	}
}

func (d *DarkLaunchIpResource) GetDarkIps(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	RespTextPlainOk(w, fmt.Sprintf("DarkMode IPs = %v", d.darkLaunchManager.IpList()))
	return true
}

// @Path("/{ip}")
func (d *DarkLaunchIpResource) GetDarkModeByHost(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool {
	ip := params.ByName("ip")
	RespTextPlainOk(w, fmt.Sprintf("DarkMode = %v for ip = %s", d.darkLaunchManager.ContainsIp(ip), ip))
	return true
}

// @Path("/{ip}")
func (d *DarkLaunchIpResource) PutEnableDarkModeByIp(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool {
	ip := params.ByName("ip")
	if err := d.darkLaunchManager.AddIp(ip); err != nil {
		d.errorHandle(w, r, ip, "Add IP")
	} else {
		util.LOG.Infof("darkLaunch update: true, action: Add IP, ip: %s", ip)
		RespTextPlainOk(w, "update dark launch manager successfully")
	}
	return true
}

// @Path("/{ip}")
func (d *DarkLaunchIpResource) DeleteDarkModeByIp(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool {
	ip := params.ByName("ip")
	if err := d.darkLaunchManager.RemoveIp(ip); err != nil {
		d.errorHandle(w, r, ip, "Remove IP")
	} else {
		util.LOG.Infof("darkLaunch update: true, action: Remove IP, ip: %s", ip)
		RespTextPlainOk(w, fmt.Sprintf("ip: %s was deleted successfully from dark launch manager", ip))
	}
	return true
}

func (d *DarkLaunchIpResource) errorHandle(respWriter http.ResponseWriter, req *http.Request, ip, action string) {
	errorID := uuid.New().String()
	util.LOG.Warningf("Receive invalid ip: %s, action: %s, errorID: %s", ip, action, errorID)
	respWriter.Header().Add("Content-Type", MediaType.TextPlain)
	respWriter.WriteHeader(http.StatusBadRequest)
	_, _ = respWriter.Write([]byte(fmt.Sprintf("Invalid request, invalid ip: %s, action: %s, ErrorID: %s", ip, action, errorID)))
}

func (d *DarkLaunchIpResource) RegisterResourceToHttpRouter(httpRouter *httprouter.Router, rootPath string) {
	basePath := rootPath + d.basePath
	httpRouter.GET(basePath, d.convertToHttpRouterHandlerWithFilters(d.GetDarkIps))
	httpRouter.GET(basePath+"/:ip", d.convertToHttpRouterHandlerWithFilters(d.GetDarkModeByHost))
	httpRouter.PUT(basePath+"/:ip", d.convertToHttpRouterHandlerWithFilters(d.PutEnableDarkModeByIp))
	httpRouter.DELETE(basePath+"/:ip", d.convertToHttpRouterHandlerWithFilters(d.DeleteDarkModeByIp))
}
