package scaffolding

import (
	"crypto/tls"
	"fmt"
	"net/url"

	router "github.com/torchcc/crank4go/router"
	"github.com/torchcc/crank4go/router/api"
	"github.com/torchcc/crank4go/router/darklaunch_manager"
	"github.com/torchcc/crank4go/router/router_socket"
	"github.com/torchcc/crank4go/util"
)

type RouterApp struct {
	healthURI           *url.URL
	router              *router.Router
	routerHealthService *RouterHealthService
	healthServer        *RestfulServer
	darkLaunchManager   *darklaunch_manager.DarkLaunchManager
}

func NewRouterApp2(routerConfig *router.RouterConfig, healthPort int) *RouterApp {
	routerConfig.SetupDarkLaunchManagerWithPath("/fake-path") // TODO to configure properly
	startedRouter := router.CreateAndStartRouter(routerConfig)
	routerHealthService := NewRouterHealthService(startedRouter.RouterAvailability())
	healthServer := NewRestfulServer(healthPort, api.NewHealthServiceResource2(routerHealthService))
	healthURI, _ := url.Parse(fmt.Sprintf("http://%s:%d/health", routerConfig.WebserverInterface(), healthPort))

	return &RouterApp{
		healthURI:           healthURI,
		router:              startedRouter,
		routerHealthService: routerHealthService,
		healthServer:        healthServer,
		darkLaunchManager:   routerConfig.DarkLaunchManager(),
	}
}

func NewRouterApp(httpPort, registerPort, healthPort int,
	webserverTLSConfig, websocketTLSConfig *tls.Config,
	webserverInterface, websocketInterface string,
	dataPublishHandlers []util.DataPublishHandler) *RouterApp {

	connMonitor := util.NewConnectionMonitor(dataPublishHandlers)

	routerConfig := router.NewRouterConfig(websocketInterface, webserverInterface, registerPort, httpPort, websocketTLSConfig, webserverTLSConfig)
	routerConfig.SetReqComponentHeader("amazon-AI-component").
		SetCheckOrigin(func(s string) bool {
			return true
		}).
		SetIsShutDownHookAdded(true).
		SetConnMonitor(connMonitor).
		ConfigDarkLaunch("aaa", "dark-mode-service"). // TODO to config properly
		SetupDarkLaunchManagerWithPath("/fake-path")

	healthURI, _ := url.Parse(fmt.Sprintf("http://%s:%d/health", webserverInterface, healthPort))
	startedRouter := router.CreateAndStartRouter(routerConfig)
	routerHealthService := NewRouterHealthService(startedRouter.RouterAvailability())
	healthServer := NewRestfulServer(healthPort, api.NewHealthServiceResource2(routerHealthService))

	return &RouterApp{
		healthURI:           healthURI,
		router:              startedRouter,
		routerHealthService: routerHealthService,
		darkLaunchManager:   routerConfig.DarkLaunchManager(),
		healthServer:        healthServer,
	}
}

func (r *RouterApp) RegisterURI() *url.URL {
	return r.router.RegisterURI
}

func (r *RouterApp) WebsocketFarm() *router_socket.WebsocketFarm {
	return r.router.WebsocketFarm()
}

func (r *RouterApp) HttpURI() *url.URL {
	return r.router.HttpURI
}

func (r *RouterApp) RouterHealthService() *RouterHealthService {
	return r.routerHealthService
}

func (r *RouterApp) DarkLaunchManager() *darklaunch_manager.DarkLaunchManager {
	return r.darkLaunchManager
}

func (r *RouterApp) Start() {
	r.healthServer.Start()
	r.routerHealthService.ScheduleHealthCheck()
}
func (r *RouterApp) Shutdown() {
	r.healthServer.ShutDown()
	r.router.Shutdown()
}
