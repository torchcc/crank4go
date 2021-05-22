package router

import (
	"crypto/tls"
	"time"

	"github.com/torchcc/crank4go/router/darklaunch_manager"
	"github.com/torchcc/crank4go/router/handler"
	"github.com/torchcc/crank4go/router/interceptor"
	"github.com/torchcc/crank4go/router/plugin"
	"github.com/torchcc/crank4go/util"
)

type RouterConfig struct {
	websocketInterface        string
	webserverInterface        string
	registrationWebsocketPort int
	websocketTLSConfig        *tls.Config
	webserverTLSConfig        *tls.Config
	httpPort                  int
	connMonitor               *util.ConnectionMonitor
	handlerList               []handler.XHandler
	ipValidator               Validator
	darkLaunchManager         *darklaunch_manager.DarkLaunchManager
	darkLaunchPublicKey       string
	darkLaunchServiceName     string
	// indicates which key to found requestComponentName from the Header obj of *http.Request
	reqComponentHeader   string
	checkOrigin          func(string) bool
	proxyInterceptors    []interceptor.ProxyInterceptor
	routerSocketPlugins  []plugin.RouterSocketPlugin
	isShutDownHookAdded  bool
	idleTimeout          time.Duration
	pingScheduleInterval time.Duration
}

func (r *RouterConfig) PingScheduleInterval() time.Duration {
	return r.pingScheduleInterval
}

func (r *RouterConfig) IdleTimeout() time.Duration {
	return r.idleTimeout
}

func (r *RouterConfig) DarkLaunchManager() *darklaunch_manager.DarkLaunchManager {
	return r.darkLaunchManager
}

func (r *RouterConfig) DarkLaunchPublicKey() string {
	return r.darkLaunchPublicKey
}

func (r *RouterConfig) IpValidator() Validator {
	return r.ipValidator
}

// NewRouterConfig  Mandatory values are set in constructor; optional values are set by setters.
// @param websocketInterface e.g. 0.0.0.0
// @param webserverInterface e.g. 0.0.0.0
// @param registrationWebsocketPort the port that crank which connector make websocket connect to
// @param websocketTLSConfig
// @param webserverTLSConfig
// @param httpPorts the port to point your browser at. https://hostname:port
// @return *RouterConfig
func NewRouterConfig(websocketInterface, webserverInterface string, registrationWebsocketPort, httpPort int, websocketTLSConfig, webserverTLSConfig *tls.Config) *RouterConfig {
	return NewRouterConfig2(websocketInterface, webserverInterface, registrationWebsocketPort, httpPort, websocketTLSConfig, webserverTLSConfig, nil, nil)
}

func NewRouterConfig2(websocketInterface, webserverInterface string, registrationWebsocketPort, httpPort int,
	websocketTLSConfig, webserverTLSConfig *tls.Config,
	routerSocketPlugins []plugin.RouterSocketPlugin, proxyInterceptors []interceptor.ProxyInterceptor) *RouterConfig {
	config := &RouterConfig{
		websocketInterface:        websocketInterface,
		webserverInterface:        webserverInterface,
		registrationWebsocketPort: registrationWebsocketPort,
		websocketTLSConfig:        websocketTLSConfig,
		webserverTLSConfig:        webserverTLSConfig,
		handlerList:               make([]handler.XHandler, 0, 8),
		httpPort:                  httpPort,
		proxyInterceptors:         proxyInterceptors,
		routerSocketPlugins:       routerSocketPlugins,
		isShutDownHookAdded:       true,
		ipValidator:               &IpValidator{},
	}

	if config.proxyInterceptors == nil {
		config.proxyInterceptors = make([]interceptor.ProxyInterceptor, 0, 0)
	}
	if config.routerSocketPlugins == nil {
		config.routerSocketPlugins = make([]plugin.RouterSocketPlugin, 0, 0)
	}
	return config
}

func (r *RouterConfig) RouterSocketPlugins() []plugin.RouterSocketPlugin {
	return r.routerSocketPlugins
}

func (r *RouterConfig) ProxyInterceptors() []interceptor.ProxyInterceptor {
	return r.proxyInterceptors
}

func (r *RouterConfig) CheckOrigin() func(string) bool {
	return r.checkOrigin
}

func (r *RouterConfig) SetCheckOrigin(checkOrigin func(string) bool) *RouterConfig {
	r.checkOrigin = checkOrigin
	return r
}

func (r *RouterConfig) SetReqComponentHeader(reqComponentHeader string) *RouterConfig {
	r.reqComponentHeader = reqComponentHeader
	return r
}

/*
use to log request component name
*/
func (r *RouterConfig) ReqComponentHeader() string {
	return r.reqComponentHeader
}

func (r *RouterConfig) DarkLaunchServiceName() string {
	return r.darkLaunchServiceName
}

/*
path is where to store the dark ip config in disk
*/
func (r *RouterConfig) SetupDarkLaunchManagerWithPath(path string) {
	r.darkLaunchManager = darklaunch_manager.NewDarkLaunchManager2(path)
}

/*
darkLaunchPublicKey is used to authorize for traffic control in DarkLaunchHandler
without darkLaunchPublicKey DarkLaunchHandler will return 500
darkLaunchServiceName is used for authorization. if input header decode name is not equal to darkLaunchServiceName, then DarkLaunchHandler will return 500
*/
func (r *RouterConfig) ConfigDarkLaunch(darkLaunchPublicKey, darkLaunchServiceName string) *RouterConfig {
	r.darkLaunchPublicKey = darkLaunchPublicKey
	r.darkLaunchServiceName = darkLaunchServiceName
	return r
}

func (r *RouterConfig) ConnMonitor() *util.ConnectionMonitor {
	return r.connMonitor
}

/**
 * @Description: send metrics to various data publishers
 * @receiver r
 * @param connMonitor
 */
func (r *RouterConfig) SetConnMonitor(connMonitor *util.ConnectionMonitor) *RouterConfig {
	r.connMonitor = connMonitor
	return r
}

func (r *RouterConfig) HttpPort() int {
	return r.httpPort
}

func (r *RouterConfig) RegistrationWebsocketPort() int {
	return r.registrationWebsocketPort
}

func (r *RouterConfig) IsShutDownHookAdded() bool {
	return r.isShutDownHookAdded
}

/**
 * @Description: a parameter that determines if router shutdown Hook will be called before shutdown.
 * @receiver r
 * @param addShutDownHook
 */
func (r *RouterConfig) SetIsShutDownHookAdded(addShutDownHook bool) *RouterConfig {
	r.isShutDownHookAdded = addShutDownHook
	return r
}

func (r *RouterConfig) HandlerList() []handler.XHandler {
	return r.handlerList
}

/**
 * @Description: a list of request handlers that will be called before the request bis passed to cranker.
 * this lets you observe, change, or filter requests. to stop a request from sending to cranker, return true is your handler
 * @receiver r
 * @param handlerList
 */
func (r *RouterConfig) SetHandlerList(handlerList []handler.XHandler) {
	r.handlerList = handlerList
}

func (r *RouterConfig) WebserverInterface() string {
	return r.webserverInterface
}

func (r *RouterConfig) WebsocketInterface() string {
	return r.websocketInterface
}

func (r *RouterConfig) WebsocketTLSConfig() *tls.Config {
	return r.websocketTLSConfig
}

func (r *RouterConfig) WebserverTLSConfig() *tls.Config {
	return r.webserverTLSConfig
}
