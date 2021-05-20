package router

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/api"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/corsheader_processor"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/darklaunch_manager"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/handler"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/router_socket"
	ptc "github.com/torchcc/crank4go-core/cranker-protocol"
	"github.com/torchcc/crank4go-core/util"
)

var upgrader = &ws.Upgrader{
	HandshakeTimeout:  45 * time.Second,
	ReadBufferSize:    4096, // default value
	WriteBufferSize:   4096, // default value
	WriteBufferPool:   &sync.Pool{},
	EnableCompression: false,
}

type Router struct {
	routerConfig        *RouterConfig
	ipValidator         Validator
	darkLaunchManager   *darklaunch_manager.DarkLaunchManager
	authPublicKey       string
	authServiceName     string
	reqComponentHeader  string
	websocketFarm       *router_socket.WebsocketFarm
	RegisterURI         *url.URL
	HttpURI             *url.URL
	websocketTLSConfig  *tls.Config
	webserverTLSConfig  *tls.Config
	connMonitor         *util.ConnectionMonitor
	httpServer          *http.Server
	registrationServer  *http.Server
	idleTimeout         time.Duration
	pingInterval        time.Duration
	routerAvailability  *RouterAvailability
	corsHeaderProcessor *corsheader_processor.CorsHeaderProcessor
}

func (r *Router) WebsocketFarm() *router_socket.WebsocketFarm {
	return r.websocketFarm
}

func (r *Router) RouterAvailability() *RouterAvailability {
	return r.routerAvailability
}

// validate register and deregister request
func ValidateReq(ipValidator Validator, req *http.Request) error {
	if err := validateIpAddr(ipValidator, req); err != nil {
		return err
	} else {
		return validateCrankerProtocolVersion(req)
	}
}

func validateCrankerProtocolVersion(req *http.Request) error {
	version := req.Header.Get("CrankerProtocol")
	if flag := ptc.ValidateCrankerProtocolVersion(version); !flag {
		msg := fmt.Sprintf("failed to establish websocket connection to cranker connector for nonsupport cranker version: %s, routerName is: %s", version, req.Header.Get("Route"))
		util.LOG.Warningf(msg)
		return &util.CrankerErr{
			Msg:  msg,
			Code: http.StatusInternalServerError,
		}
	} else {
		return nil
	}
}

func validateIpAddr(validator Validator, req *http.Request) error {
	if !validator.IsValid(req.RemoteAddr) {
		msg := fmt.Sprintf("failed to establish websocket connection to cranker connector for invalid ip: %s, routerName is: %s", ip, req.Header.Get("Route"))
		util.LOG.Warning(msg)
		return &util.CrankerErr{
			Msg:  msg,
			Code: http.StatusForbidden,
		}
	}
	return nil
}

func NewRouter(routerConfig *RouterConfig) *Router {
	r := &Router{
		routerConfig:        routerConfig,
		HttpURI:             nil,
		ipValidator:         routerConfig.IpValidator(),
		authPublicKey:       routerConfig.DarkLaunchPublicKey(),
		authServiceName:     routerConfig.DarkLaunchServiceName(),
		reqComponentHeader:  routerConfig.ReqComponentHeader(),
		websocketTLSConfig:  routerConfig.WebsocketTLSConfig(),
		webserverTLSConfig:  routerConfig.WebserverTLSConfig(),
		darkLaunchManager:   routerConfig.darkLaunchManager,
		websocketFarm:       router_socket.NewWebsocketFarm(routerConfig.ConnMonitor(), routerConfig.DarkLaunchManager()),
		connMonitor:         routerConfig.ConnMonitor(),
		idleTimeout:         routerConfig.IdleTimeout(),
		pingInterval:        routerConfig.PingScheduleInterval(),
		corsHeaderProcessor: corsheader_processor.NewCorsHeaderProcessor(routerConfig.CheckOrigin()),
	}

	r.routerAvailability = NewRouterAvailability2(r.connMonitor, r.websocketFarm, r.darkLaunchManager, routerConfig.IsShutDownHookAdded())
	theSecureS := ""
	if r.webserverTLSConfig != nil {
		theSecureS = "s"
	}
	port := routerConfig.HttpPort()
	r.HttpURI, _ = url.Parse("http" + theSecureS + "://" + routerConfig.WebserverInterface() + ":" + strconv.Itoa(port))
	theSecureS = ""
	if r.websocketTLSConfig != nil {
		theSecureS = "s"
	}
	r.RegisterURI, _ = url.Parse("ws" + theSecureS + "://" + routerConfig.WebsocketInterface() + ":" + strconv.Itoa(routerConfig.RegistrationWebsocketPort()))
	return r
}

func (r *Router) connectorRegisterToRouter(respWriter http.ResponseWriter, req *http.Request) *router_socket.RouterSocket {
	route := getRoute(req)
	connectorInstanceID := req.URL.Query().Get("connectorInstanceID")
	if connectorInstanceID == "" {
		connectorInstanceID = "unknown-" + req.RemoteAddr
	}
	util.LOG.Info("the register request connectorInstanceID is %s", connectorInstanceID)
	routerSocket := router_socket.NewRouterSocket2(route, r.connMonitor, r.websocketFarm, connectorInstanceID, true, req.RemoteAddr, r.corsHeaderProcessor, r.routerConfig.RouterSocketPlugins())
	util.LOG.Infof("got routerSocket %s", routerSocket.String())
	routerSocket.SetOnReadyToAct(func() {
		r.websocketFarm.AddWebsocket(route, routerSocket)
	})

	// validation pass can upgrade to websocket now
	header := http.Header{}
	header.Set("CrankerProtocol", ptc.CrankerProtocolVersion10)
	if conn, err := upgrader.Upgrade(respWriter, req, header); err != nil {
		util.LOG.Errorf("upgrade error: %s, the requestURI is: %s, remoteAddr is %s", err.Error(), req.URL.String(), req.RemoteAddr)
		return nil
	} else if conn == nil {
		util.LOG.Errorf("upgrade error: got nil conn after upgrade, the requestURI is: %s, remoteAddr is %s", req.URL.String(), req.RemoteAddr)
		return nil
	} else {
		util.LOG.Infof("registered successfully, the connecting remote address is %s", req.RemoteAddr)
		go routerSocket.OnWebsocketConnect(conn)
		return routerSocket
	}
}

func (r *Router) connectorDeregisterFromRouter(respWriter http.ResponseWriter, req *http.Request) *router_socket.RouterSocket {
	route := getRoute(req)
	connectorInstanceID := req.URL.Query().Get("connectorInstanceID")
	if connectorInstanceID == "" {
		connectorInstanceID = "no connector instance id exist"
	}
	componentName := req.URL.Query().Get("componentName")
	if componentName == "" {
		componentName = "no component name provided"
	}
	if connectorInstanceID == "no connector instance id exist" {
		util.LOG.Infof("targetName {%s} is using unsupported zero down time connector, socket will not be deregistered")
	} else {
		util.LOG.Infof("going to deregister... remoteAddr: %s, targetName: %s, componentName: %s, connectorInstanceID: %s", req.RemoteAddr, route, componentName, connectorInstanceID)
		r.websocketFarm.DeregisterSocket(route, req.RemoteAddr, connectorInstanceID)
	}
	routerSocket := router_socket.NewRouterSocket(route, r.connMonitor, r.websocketFarm, connectorInstanceID, false, req.RemoteAddr, r.corsHeaderProcessor)
	header := http.Header{}
	header.Set("CrankerProtocol", ptc.CrankerProtocolVersion10)
	respWriter.Header().Set("CrankerProtocol", ptc.CrankerProtocolVersion10)

	if conn, err := upgrader.Upgrade(respWriter, req, header); err != nil {
		util.LOG.Errorf("upgrade error: %s, the requestURI is: %s, remoteAddr is %s", err.Error(), req.URL.String(), req.RemoteAddr)
		return nil
	} else {
		util.LOG.Infof("registered successfully, the connecting remote address is %s", req.RemoteAddr)
		go routerSocket.OnWebsocketConnect(conn)
		return routerSocket
	}
}

func (r *Router) CreateRegisterHandler() *httprouter.Router {
	httpRouter := httprouter.New()

	// use to register
	registerWsHandler := handler.NewWebsocketHandler().WithWebsocketFactory(r.registerWebsocketFactory)
	registerWsXHandler := handler.NewXHttpHandler(registerWsHandler)
	httpRouter.GET("/register", registerWsXHandler.ServeXHTTP)
	httpRouter.GET("/register/", registerWsXHandler.ServeXHTTP)

	deregisterWsHandler := handler.NewWebsocketHandler().WithWebsocketFactory(r.deregisterWebsocketFactory)
	deregisterWsXHandler := handler.NewXHttpHandler(deregisterWsHandler)
	httpRouter.GET("/deregister", deregisterWsXHandler.ServeXHTTP)
	httpRouter.GET("/deregister/", deregisterWsXHandler.ServeXHTTP)

	// TODO should add authentication filter to restraint access
	darkLaunchServiceResource := api.NewDarkLaunchServiceResource(r.darkLaunchManager)
	darkLaunchServiceResource.
		AddReqFilters(handler.XHandlerFunc(handler.PreLoggingFilter)).
		AddRespFilters(handler.XHandlerFunc(handler.PostLoggingFilter))
	darkLaunchServiceResource.RegisterResourceToHttpRouter(httpRouter, "/api")

	darkLaunchIpResource := api.NewDarkLaunchIpResource(r.darkLaunchManager)
	darkLaunchIpResource.
		AddReqFilters(handler.XHandlerFunc(handler.PreLoggingFilter)).
		AddRespFilters(handler.XHandlerFunc(handler.PostLoggingFilter))
	darkLaunchIpResource.RegisterResourceToHttpRouter(httpRouter, "/api")

	launchGrayToggleResource := api.NewDarkLaunchGrayToggleResource(r.darkLaunchManager)
	launchGrayToggleResource.
		AddReqFilters(handler.XHandlerFunc(handler.PreLoggingFilter)).
		AddRespFilters(handler.XHandlerFunc(handler.PostLoggingFilter))
	launchGrayToggleResource.RegisterResourceToHttpRouter(httpRouter, "/api")

	registrationsResource := api.NewRegistrationsResource(r.websocketFarm)
	registrationsResource.
		AddReqFilters(handler.XHandlerFunc(handler.PreLoggingFilter)).
		AddRespFilters(handler.XHandlerFunc(handler.PostLoggingFilter))
	registrationsResource.RegisterResourceToHttpRouter(httpRouter, "/api")

	return httpRouter
}

// can configure rate limit here
func (r *Router) CreateHttpHandler() *handler.XHTTPHandler {
	return handler.NewXHttpHandler(NewReverseProxy(r.websocketFarm, r.reqComponentHeader, r.routerConfig.ProxyInterceptors())).
		AddReqHandlers(handler.XHandlerFunc(handler.PreLoggingFilter)).
		AddReqHandlers(r.routerConfig.HandlerList()...).
		AddReqHandlers(handler.XHandlerFunc(handler.ReqValidatorFilter))
}

func (r *Router) Start() *Router {
	serveMux := http.NewServeMux()
	serveMux.Handle("/", r.CreateHttpHandler())
	r.httpServer = &http.Server{
		Addr:           r.HttpURI.Host,
		Handler:        serveMux,
		TLSConfig:      r.webserverTLSConfig,
		MaxHeaderBytes: 8192 * 4,
	}
	util.LOG.Infof("starting router httpServer on %v", r.HttpURI)
	go func() {
		if r.HttpURI.Scheme == "https" {
			util.LOG.Infof("httpServer uses TLS")
			if err := r.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				util.LOG.Warningf("crankerRouter's httpServer failed to start, err: %s", err.Error())
				panic(err)
			}
		} else {
			if err := r.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				util.LOG.Warningf("crankerRouter's httpServer failed to start, err: %s", err.Error())
				panic(err)
			}
		}
	}()

	r.registrationServer = &http.Server{
		Addr:           fmt.Sprintf("%s", r.RegisterURI.Host),
		Handler:        r.CreateRegisterHandler(),
		TLSConfig:      r.websocketTLSConfig,
		MaxHeaderBytes: 8192 * 4,
	}
	util.LOG.Infof("starting router registrationServer on %v", r.RegisterURI)
	go func() {
		if r.RegisterURI.Scheme == "wss" {
			util.LOG.Infof("registerServer uses TLS")
			if err := r.registrationServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				util.LOG.Warningf("crankerRouter's registrationServer failed to start, err: %s", err.Error())
				panic(err)
			}
		} else {
			if err := r.registrationServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				util.LOG.Warningf("crankerRouter's registrationServer failed to start, err: %s", err.Error())
				panic(err)
			}
		}
	}()
	return r
}

func (r *Router) Shutdown() {
	var wg sync.WaitGroup

	go func() {
		wg.Add(1)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := r.httpServer.Shutdown(ctx); err != nil {
			util.LOG.Warningf("crankerRouter's httpServer shutdown gracefully returned, err: %s", err.Error())
		}
		wg.Done()
	}()

	go func() {
		wg.Add(1)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := r.registrationServer.Shutdown(ctx); err != nil {
			util.LOG.Warningf("crankerRouter's registrationServer shutdown gracefully returned, err: %s", err.Error())
		}
		wg.Done()

	}()
	wg.Wait()
}

func getRoute(req *http.Request) string {
	return req.Header.Get("Route")
}

func (r *Router) registerWebsocketFactory(respWriter http.ResponseWriter, req *http.Request) *router_socket.RouterSocket {
	util.LOG.Infof("got register request: %s, remote addr is %s, routeName is %s",
		req.RequestURI, req.RemoteAddr, req.Header.Get("Route"))
	if err := ValidateReq(r.ipValidator, req); err != nil {
		e := err.(*util.CrankerErr)
		api.RespTextPlainWithStatus(respWriter, e.Error(), e.Code)
		return nil
	}
	return r.connectorRegisterToRouter(respWriter, req)
}

func (r *Router) deregisterWebsocketFactory(respWriter http.ResponseWriter, req *http.Request) *router_socket.RouterSocket {
	if err := ValidateReq(r.ipValidator, req); err != nil {
		e := err.(*util.CrankerErr)
		api.RespTextPlainWithStatus(respWriter, e.Error(), e.Code)
		return nil
	}
	return r.connectorDeregisterFromRouter(respWriter, req)
}

// program shutdown hooks
var shutDownHooks []func()

func addShutDownHook(f func()) {
	if shutDownHooks == nil {
		shutDownHooks = make([]func(), 0, 8)
	}
	shutDownHooks = append(shutDownHooks, f)
}

func exitFunc() {
	for _, f := range shutDownHooks {
		f()
	}
}

func init() {
	util.ExitProgram(exitFunc)
}

func CreateAndStartRouter(config *RouterConfig) *Router {
	router := NewRouter(config).Start()
	if config.isShutDownHookAdded {
		addShutDownHook(router.Shutdown)
	}
	return router
}
