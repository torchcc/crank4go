package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/torchcc/crank4go/router/router_socket"
	"github.com/torchcc/crank4go/util"
)

const registrationsResourceBasePath string = "/registrations"

type RegistrationsResource struct {
	basePath      string
	websocketFarm *router_socket.WebsocketFarm
	*Filter
}

func NewRegistrationsResource(websocketFarm *router_socket.WebsocketFarm) *RegistrationsResource {
	return &RegistrationsResource{
		websocketFarm: websocketFarm,
		basePath:      registrationsResourceBasePath,
		Filter:        &Filter{},
	}
}

func (r *RegistrationsResource) GetRegisterInfo(w http.ResponseWriter, req *http.Request, _ httprouter.Params) bool {
	begin := time.Now()
	servicesRegisterMap := make(map[string]interface{})

	for serviceName, routerSocketQueue := range r.websocketFarm.AllSockets() {
		remoteAddrs := make(map[string]struct{})
		for _, routerSocket := range routerSocketQueue {
			remoteAddrs[strings.Split(routerSocket.RemoteAddr(), ":")[0]] = struct{}{}
			servicesRegisterMap[serviceName] = remoteAddrs
		}
	}

	remoteAddrs := make(map[string]struct{})
	for _, routerSocket := range r.websocketFarm.AllCatchall() {
		remoteAddrs[strings.Split(routerSocket.RemoteAddr(), ":")[0]] = struct{}{}
	}
	servicesRegisterMap["default"] = remoteAddrs
	util.LOG.Debugf("getRegisterInfo spent %v time, the request is from %s", time.Now().Sub(begin), req.RemoteAddr)
	RespJsonOk(w, servicesRegisterMap)
	return true
}

func (r *RegistrationsResource) RegisterResourceToHttpRouter(httpRouter *httprouter.Router, rootPath string) {
	basePath := rootPath + r.basePath
	httpRouter.GET(basePath, r.convertToHttpRouterHandlerWithFilters(r.GetRegisterInfo))
}
