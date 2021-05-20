package router

import (
	"context"
	"strings"
	"time"

	"github.com/torchcc/crank4go-core/crank4go-router-embedded/darklaunch_manager"
	"github.com/torchcc/crank4go-core/crank4go-router-embedded/router_socket"
	protocol "github.com/torchcc/crank4go-core/cranker-protocol"
	"github.com/torchcc/crank4go-core/util"
)

type RouterAvailability struct {
	connMonitor       *util.ConnectionMonitor
	websocketFarm     *router_socket.WebsocketFarm
	darkLaunchManager *darklaunch_manager.DarkLaunchManager
	cancelPing        context.CancelFunc // it's used to cancelPing pingTask
	cancelCtx         context.Context
}

func NewRouterAvailability(connMonitor *util.ConnectionMonitor, websocketFarm *router_socket.WebsocketFarm, darkLaunchManager *darklaunch_manager.DarkLaunchManager) *RouterAvailability {
	a := &RouterAvailability{connMonitor: connMonitor, websocketFarm: websocketFarm, darkLaunchManager: darkLaunchManager}
	a.cancelCtx, a.cancelPing = context.WithCancel(context.Background())
	return a
}

func NewRouterAvailability2(connMonitor *util.ConnectionMonitor, websocketFarm *router_socket.WebsocketFarm, darkLaunchManager *darklaunch_manager.DarkLaunchManager, isShutDownHookAdded bool) *RouterAvailability {
	a := &RouterAvailability{
		connMonitor:       connMonitor,
		websocketFarm:     websocketFarm,
		darkLaunchManager: darkLaunchManager,
	}
	a.cancelCtx, a.cancelPing = context.WithCancel(context.Background())
	if isShutDownHookAdded {
		// shutdown ping.
	}
	return a

}

func (a *RouterAvailability) Status() map[string]interface{} {
	serviceRegisterMapping := make(map[string]interface{})
	sockets := a.websocketFarm.AllSockets()

	for serviceName, socketQueue := range sockets {
		remoteAddr := make(map[string]int)
		for _, routerSocket := range socketQueue {
			getRemoteAddrMapping(remoteAddr, routerSocket)
		}
		serviceRegisterMapping[serviceName] = remoteAddr
	}

	catchallRemoteAddr := make(map[string]int)
	allCatchall := a.websocketFarm.AllCatchall()
	for _, routerSocket := range allCatchall {
		getRemoteAddrMapping(catchallRemoteAddr, routerSocket)
	}
	serviceRegisterMapping["default"] = catchallRemoteAddr

	status := make(map[string]interface{})
	status["CrankerProtocol"] = protocol.CrankerProtocolVersion10
	status["activeConnections"] = a.connMonitor.ConnectionCount()
	status["openFiles"] = a.connMonitor.OpenFiles()
	status["Services Register Map"] = serviceRegisterMapping
	status["services"] = "/health/connectors"
	status["darkMode"] = a.darkLaunchManager
	status["isAvailable"] = true
	return status
}

func (a *RouterAvailability) ServicesCategorizedDetail() map[string]interface{} {
	servicesConnState := make(map[string]interface{})
	catchall := a.websocketFarm.CategorizedAllSockets()
	for category, routerSocketMap := range catchall {
		state := make(map[string]interface{})
		servicesConnState[category] = state
		for serviceName, routerSocketQueue := range routerSocketMap {
			serviceConnectors := make([]interface{}, 0, 8)
			state[serviceName] = map[string]interface{}{"name": serviceName, "connectors": serviceConnectors}
			remoteAddr := make(map[string]struct{})
			for _, routerSocket := range routerSocketQueue {
				getServicesMap(remoteAddr, routerSocket, &serviceConnectors)
			}
		}
	}

	serviceConnectorsMap := make(map[string]interface{})
	servicesConnState["default"] = map[string]interface{}{"name": "default", "connectors": serviceConnectorsMap}
	for category, routerSocketQueue := range a.websocketFarm.CategorizedAllCatchall() {
		serviceConnectors := make([]interface{}, 0, 8)
		catchallRemoteAddr := make(map[string]struct{})
		for _, routerSocket := range routerSocketQueue {
			getServicesMap(catchallRemoteAddr, routerSocket, &serviceConnectors)
		}
		serviceConnectorsMap[category] = serviceConnectors
	}

	return servicesConnState
}

func (a *RouterAvailability) Services() map[string]interface{} {
	servicesConnState := make(map[string]interface{})
	for serviceName, routerSocketQueue := range a.websocketFarm.AllSockets() {
		serviceConnectors := make([]interface{}, 0, 8)
		remoteAddr := make(map[string]struct{})
		for _, routerSocket := range routerSocketQueue {
			getServicesMap(remoteAddr, routerSocket, &serviceConnectors)
		}
		servicesConnState[serviceName] = map[string]interface{}{"name": serviceName, "connectors": serviceConnectors}
	}

	serviceConnectors := make([]interface{}, 0, 8)
	catchallRemoteAddr := make(map[string]struct{})
	for _, routerSocket := range a.websocketFarm.AllCatchall() {
		getServicesMap(catchallRemoteAddr, routerSocket, &serviceConnectors)
	}
	servicesConnState["default"] = map[string]interface{}{"name": "default", "connectors": serviceConnectors}
	return servicesConnState
}

func (a *RouterAvailability) scheduleSendPingToConnector() {

	go func(ctx context.Context) {
	LOOP:
		for {
			a.websocketFarm.DarkSockets().Range(func(key, value interface{}) bool {
				route := key.(string)
				routerSocketQueue := value.(*router_socket.IterableChan)
				a.sendPingToConnector(route, routerSocketQueue)
				return true
			})
			a.websocketFarm.Sockets().Range(func(key, value interface{}) bool {
				route := key.(string)
				routerSocketQueue := value.(*router_socket.IterableChan)
				a.sendPingToConnector(route, routerSocketQueue)
				return true
			})
			a.sendPingToConnector("default", a.websocketFarm.Catchall())
			a.sendPingToConnector("default", a.websocketFarm.DarkCatchall())

			select {
			case <-ctx.Done():
				break LOOP
			default:
			}
			time.Sleep(5 * time.Second)
		}

	}(a.cancelCtx)
}

func (a *RouterAvailability) sendPingToConnector(route string, queue *router_socket.IterableChan) {
	queue.Range(func(value interface{}) bool {
		routerSocket := value.(*router_socket.RouterSocket)
		util.LOG.Debugf("inside router availability: going to send ping, routerName: %s, connectorInstanceID: %S", route, routerSocket.ConnectorInstanceID())
		routerSocket.SendPingToConnector()
		return true
	})
}

func (a *RouterAvailability) shutdownPing() {
	util.LOG.Infof("going to shutdown ping")
	a.cancelPing()
}

func (a *RouterAvailability) Shutdown() {
	a.shutdownPing()
}

func getServicesMap(remoteAddr map[string]struct{}, routerSocket *router_socket.RouterSocket, serviceConnectors *[]interface{}) {
	curRemoteAddr := strings.Split(routerSocket.RemoteAddr(), ":")[0]
	if _, ok := remoteAddr[curRemoteAddr]; ok {
		for _, a := range *serviceConnectors {
			serviceConnector := a.(map[string]interface{})
			if ip, exist := serviceConnector["ip"]; exist && ip.(string) == curRemoteAddr {
				connArray := serviceConnector["connections"].([]interface{})
				connArray = append(connArray, addRouterSocketIdByConnector(routerSocket, routerSocket.LastPingTime().String()))
				break
			}
		}
	} else {
		connPerIp := addRouterSocketIdByConnector(routerSocket, routerSocket.LastPingTime().String())
		conns := make([]interface{}, 0, 8)
		conns = append(conns, connPerIp)
		connInfo := addConnectorByIp(routerSocket, curRemoteAddr, conns)
		*serviceConnectors = append(*serviceConnectors, connInfo)
	}
}

func addConnectorByIp(routerSocket *router_socket.RouterSocket, curRemoteAddr string, conns []interface{}) map[string]interface{} {
	return map[string]interface{}{
		"connectorInstanceID": routerSocket.ConnectorInstanceID(),
		"ip":                  curRemoteAddr,
		"route":               routerSocket.Route,
		"component":           routerSocket.ReqComponentName(),
		"connections":         conns,
	}
}

func addRouterSocketIdByConnector(routerSocket *router_socket.RouterSocket, lastPingTime string) map[string]interface{} {
	return map[string]interface{}{"socketID": routerSocket.RouterSocketID, "lastPingTime": lastPingTime}
}

func getRemoteAddrMapping(remoteAddr map[string]int, routerSocket *router_socket.RouterSocket) {
	curRemoteAddr := strings.Split(routerSocket.RemoteAddr(), ":")[0]
	if _, ok := remoteAddr[curRemoteAddr]; ok {
		remoteAddr[curRemoteAddr] += 1
	} else {
		remoteAddr[curRemoteAddr] = 1
	}
}
