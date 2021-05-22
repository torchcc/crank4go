package router_socket

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/torchcc/crank4go/router/darklaunch_manager"
	"github.com/torchcc/crank4go/util"
)

const blockingQueueCapacity = 0 // use default value

type WebsocketFarm struct {
	sockets           *sync.Map // in format of map[string]*IterableChan this Map is like java's ConcurrentHashMap
	darkSockets       *sync.Map // in format of map[string]*IterableChan
	catchall          *IterableChan
	darkCatchall      *IterableChan
	connMonitor       *util.ConnectionMonitor
	timeSpent         int64
	socketAcquireTime time.Duration
	darkLaunchManager *darklaunch_manager.DarkLaunchManager
}

func NewWebsocketFarm(connMonitor *util.ConnectionMonitor, darkLaunchManager *darklaunch_manager.DarkLaunchManager) *WebsocketFarm {
	f := &WebsocketFarm{
		sockets:           &sync.Map{},
		darkSockets:       &sync.Map{},
		catchall:          NewIterableChan(blockingQueueCapacity),
		darkCatchall:      NewIterableChan(blockingQueueCapacity),
		connMonitor:       connMonitor,
		timeSpent:         0,
		socketAcquireTime: time.Second * 15,
		darkLaunchManager: darkLaunchManager,
	}
	listener := &darkListener{
		sockets:      f.sockets,
		darkSockets:  f.darkSockets,
		catchall:     f.catchall,
		darkCatchAll: f.darkCatchall,
	}
	darkLaunchManager.SetIpListener(listener).SetServiceListener(listener)
	return f
}

func (f *WebsocketFarm) DarkCatchall() *IterableChan {
	return f.darkCatchall
}

func (f *WebsocketFarm) Catchall() *IterableChan {
	return f.catchall
}

func (f *WebsocketFarm) SetSocketAcquireTime(socketAcquireTime time.Duration) {
	f.socketAcquireTime = socketAcquireTime
	util.LOG.Infof("socketAcquireTime is %v", socketAcquireTime)
}

func (f *WebsocketFarm) Stop() {
	for !f.catchall.IsEmpty() {
		if socket := f.catchall.Poll(); socket == nil {
			util.LOG.Info("failed to pop socket from catchall linkedBlockingQueue, the queue is empty")
			break
		} else {
			socket.CloseSocketSession()
		}
	}
	f.sockets.Range(func(_, queue interface{}) bool {
		socketQueue := queue.(*IterableChan)
		for !socketQueue.IsEmpty() {
			if sock := socketQueue.Poll(); sock == nil {
				util.LOG.Info("failed to pop socket from sockets linkedBlockingQueue, the queue is empty")
				break
			} else {
				sock.CloseSocketSession()
			}
		}
		return true
	})
}

func (f *WebsocketFarm) RemoveWebsocket(route string, socket *RouterSocket) {
	util.LOG.Debugf("removing websocket {%s}, its connectorInstanceID is %s", route, socket.ConnectorInstanceID())
	if f.darkLaunchManager.IsDarkModeOn() && (f.darkLaunchManager.ContainsIp(socket.Ip()) || f.darkLaunchManager.ContainsService(socket.Route)) {
		util.LOG.Debugf("dark mode on and current socket %v contains ip %v, contains service %s", socket.String(),
			f.darkLaunchManager.ContainsIp(socket.Ip()), f.darkLaunchManager.ContainsService(socket.Route))
		if socket.IsCatchAll() {
			f.darkCatchall.Remove(socket)
		} else {
			if queue, ok := f.darkSockets.Load(route); ok {
				queue.(*IterableChan).Remove(socket)
			}
		}
	} else {
		util.LOG.Debugf("removing socket %s of route %s", socket.String(), route)
		if socket.IsCatchAll() {
			f.catchall.Remove(socket)
		} else {
			if queue, ok := f.sockets.Load(route); ok {
				queue.(*IterableChan).Remove(socket)
			}
		}
	}
}

func (f *WebsocketFarm) AddWebsocket(route string, socket *RouterSocket) {
	if route == "" {
		route = "*"
	}
	var queue *IterableChan
	if f.darkLaunchManager.IsDarkModeOn() && (f.darkLaunchManager.ContainsIp(socket.Ip()) || f.darkLaunchManager.ContainsService(socket.Route)) {
		util.LOG.Debugf("addWebsocket, dork mode is on and current socket is %s, contains ip is %v, "+
			"contains service is %v", socket.String(), f.darkLaunchManager.ContainsIp(socket.Ip()), f.darkLaunchManager.ContainsService(socket.Route))
		if route == "*" {
			queue = f.darkCatchall
		} else {
			if queueInterface, ok := f.darkSockets.Load(route); !ok {
				queue = NewIterableChan(blockingQueueCapacity)
				f.darkSockets.Store(route, queue)
			} else {
				queue = queueInterface.(*IterableChan)
			}
		}
	} else {
		util.LOG.Debugf("add socket %s of route %s", socket.String(), route)
		if route == "*" {
			queue = f.catchall
		} else {
			if queueInterface, ok := f.sockets.Load(route); !ok {
				queue = NewIterableChan(blockingQueueCapacity)
				f.sockets.Store(route, queue)
			} else {
				queue = queueInterface.(*IterableChan)
			}
		}
	}

	queue.Offer(socket)
	util.LOG.Debugf("websocket added: route=%s, connectorInstanceID=%s, routeSocketID=%s", route, socket.ConnectorInstanceID(), socket.RouterSocketID)
}

func (f *WebsocketFarm) AcquireSocket(target string, componentName string) (socket *RouterSocket, err error) {
	if socket = f.getRouterSocket(target); socket == nil {
		util.LOG.Warningf("failed to wait socket for %s, requestComponentName: %s, queue is empty", target, componentName)
		return nil, util.TimeoutErr{Msg: fmt.Sprintf("failed to proxy %s, requestComponentName: %s", target, componentName)}
	} else {
		util.LOG.Infof("socket acquired, target: %s, socket: %s, requestComponentName: %s", target, socket.RouterSocketID, componentName)
		socket.SetReqComponentName(componentName)
		return socket, nil
	}
}

func (f *WebsocketFarm) getRouterSocket(target string) *RouterSocket {
	var (
		sockets          *sync.Map     = f.sockets
		catchAll         *IterableChan = f.catchall
		allRouterSockets *IterableChan
	)
	if f.darkLaunchManager.IsDarkModeOn() && darklaunch_manager.IsGrayTestingOn() {
		util.LOG.Infof("gray testing on.")
		sockets = f.darkSockets
		catchAll = f.darkCatchall
	}
	route := resolveRoute(target)
	util.LOG.Debugf("handling target %s and getting router socket for route %s", target, route)

	if allRouterSocketsInterface, ok := sockets.Load(route); ok {
		allRouterSockets = allRouterSocketsInterface.(*IterableChan)
	} else {
		allRouterSockets = catchAll
	}
	f.connMonitor.ReportWebsocketPoolSize(allRouterSockets.LenAlive())
	return allRouterSockets.PollTimeout(f.socketAcquireTime)
}

func resolveRoute(target string) string {
	if len(strings.Split(target, "/")) >= 2 {
		return strings.Split(target, "/")[1]
	} else {
		// it's either root target or blank target
		return ""
	}
}

func (f *WebsocketFarm) DeregisterSocket(target, remoteAddr, connectorInstanceID string) {
	util.LOG.Infof("going to deregister target %s, targetAddr: %s, connectorInstanceID: %s", target, remoteAddr, connectorInstanceID)
	var allRouterSockets *IterableChan
	if allRouterSocketsInterface, ok := f.sockets.Load(target); ok {
		allRouterSockets = allRouterSocketsInterface.(*IterableChan)
	} else {
		allRouterSockets = f.catchall
	}
	for !allRouterSockets.IsAliveSocketSetEmpty() {
		if socket := allRouterSockets.Poll(); socket != nil {
			f.removeWebsockets(remoteAddr, connectorInstanceID, socket)
		}
	}
}

func (f *WebsocketFarm) removeWebsockets(remoteAddr, connectorInstanceID string, routerSocket *RouterSocket) {
	curConnectorInstanceID := routerSocket.ConnectorInstanceID()
	curRemoteAddr := strings.Split(strings.Split(routerSocket.RemoteAddr(), "/")[1], ":")[0]
	if connectorInstanceID == curConnectorInstanceID {
		util.LOG.Infof("currentRemoteAddr: %s, remoteAddr: %s, connectorInstanceID: %s, routerSocketID: %s", curRemoteAddr, remoteAddr, curConnectorInstanceID, routerSocket.RouterSocketID)
		f.RemoveWebsocket(routerSocket.Route, routerSocket)
	} else {
		util.LOG.Infof("connectorInstanceID discrepancy: currentRemoteAddr: %s, remoteAddr: %s, connectorInstanceID: %s, routerSocketID: %s", curRemoteAddr, remoteAddr, curConnectorInstanceID, routerSocket.RouterSocketID)
	}
}

func (f *WebsocketFarm) AllCatchall() []*RouterSocket {
	return combineCatchall(f.catchall, f.darkCatchall)
}

func combineCatchall(catchall *IterableChan, darkCatchall *IterableChan) []*RouterSocket {
	all := catchall.AliveSocketSlice()
	all = append(all, darkCatchall.AliveSocketSlice()...)
	return all
}

func (f *WebsocketFarm) CategorizedAllCatchall() map[string][]*RouterSocket {
	categorized := make(map[string][]*RouterSocket)
	if !f.darkCatchall.IsAliveSocketSetEmpty() {
		categorized["dark"] = f.darkCatchall.AliveSocketSlice()
	}
	if !f.catchall.IsAliveSocketSetEmpty() {
		categorized["normal"] = f.catchall.AliveSocketSlice()
	}
	return categorized
}

func (f *WebsocketFarm) CategorizedAllSockets() map[string]map[string][]*RouterSocket {
	categorized := make(map[string]map[string][]*RouterSocket)
	detail := make(map[string][]*RouterSocket)
	f.darkSockets.Range(func(key, value interface{}) bool {
		route := key.(string)
		routerSocketQueue := value.(*IterableChan)
		detail[route] = routerSocketQueue.AliveSocketSlice()
		return true
	})
	if len(detail) != 0 {
		categorized["dark"] = detail
	}

	detail2 := make(map[string][]*RouterSocket)
	f.sockets.Range(func(key, value interface{}) bool {
		route := key.(string)
		routerSocketQueue := value.(*IterableChan)
		detail2[route] = routerSocketQueue.AliveSocketSlice()
		return true
	})
	if len(detail2) != 0 {
		categorized["normal"] = detail2
	}
	return categorized
}

func (f *WebsocketFarm) DarkSockets() *sync.Map {
	return f.darkSockets
}

func (f *WebsocketFarm) Sockets() *sync.Map {
	return f.sockets
}

func (f *WebsocketFarm) AllSockets() map[string][]*RouterSocket {
	return combineSyncMap(f.sockets, f.darkSockets)
}

func combineSyncMap(sockets *sync.Map, darkSockets *sync.Map) map[string][]*RouterSocket {
	m := make(map[string][]*RouterSocket)

	darkSockets.Range(func(key, value interface{}) bool {
		q := value.(*IterableChan)
		m[key.(string)] = q.AliveSocketSlice()
		return true
	})

	sockets.Range(func(key, value interface{}) bool {
		route := key.(string)
		socketBlockingQu := value.(*IterableChan)
		if q, ok := m[route]; ok {
			q = append(q, socketBlockingQu.AliveSocketSlice()...)
		} else {
			m[route] = socketBlockingQu.AliveSocketSlice()
		}
		return true
	})
	return m
}

type darkListener struct {
	sockets      *sync.Map // in format of map[string]*BlockingQueue   this Map is like java's ConcurrentHashMap
	darkSockets  *sync.Map // in format of map[string]*BlockingQueue
	catchall     *IterableChan
	darkCatchAll *IterableChan
}

func (l *darkListener) AfterDarkServiceAdded(addedService string) {
	l.sockets.Range(func(_, queueInterface interface{}) bool {
		queue := queueInterface.(*IterableChan)
		toBeRemoved := make([]*RouterSocket, 0, 8)
		queue.Range(func(socketInterface interface{}) bool {
			socket := socketInterface.(*RouterSocket)
			if socket.Ip() == addedService {
				var darkQu *IterableChan
				if darkQuInterface, ok := l.darkSockets.Load(socket.Route); !ok {
					darkQu = NewIterableChan(blockingQueueCapacity)
					l.darkSockets.Store(socket.Route, darkQu)
				} else {
					darkQu = darkQuInterface.(*IterableChan)
				}
				darkQu.Offer(socket)
				toBeRemoved = append(toBeRemoved, socket)
			}
			return true
		})
		for _, socket := range toBeRemoved {
			queue.Remove(socket)
		}
		return true
	})

	toBeRemoved := make([]*RouterSocket, 0, 8)
	l.catchall.Range(func(socketInterface interface{}) bool {
		socket := socketInterface.(*RouterSocket)
		if socket.Route == addedService {
			l.darkCatchAll.Offer(socket)
			toBeRemoved = append(toBeRemoved, socket)
		}
		return true
	})
	for _, socket := range toBeRemoved {
		l.catchall.Remove(socket)
	}
}

func (l *darkListener) AfterDarkServiceRevoked(revokedService string) {
	darklaunch_manager.TurnGrayTestingOff("turn off gray testing after service revoked")
	l.darkSockets.Range(func(_, queueInterface interface{}) bool {
		darkQueue := queueInterface.(*IterableChan)
		toBeRemoved := make([]*RouterSocket, 0, 8)
		darkQueue.Range(func(socketInterface interface{}) bool {
			socket := socketInterface.(*RouterSocket)
			if socket.Route == revokedService {
				var qu *IterableChan
				if darkQuInterface, ok := l.darkSockets.Load(socket.Route); !ok {
					qu = NewIterableChan(blockingQueueCapacity)
					l.sockets.Store(socket.Route, qu)
				} else {
					qu = darkQuInterface.(*IterableChan)
				}
				qu.Offer(socket)
				toBeRemoved = append(toBeRemoved, socket)
			}
			return true
		})
		for _, socket := range toBeRemoved {
			darkQueue.Remove(socket)
		}
		return true
	})

	toBeRemoved := make([]*RouterSocket, 0, 8)
	l.darkCatchAll.Range(func(socketInterface interface{}) bool {
		socket := socketInterface.(*RouterSocket)
		if socket.Route == revokedService {
			l.catchall.Offer(socket)
			toBeRemoved = append(toBeRemoved, socket)
		}
		return true
	})
	for _, socket := range toBeRemoved {
		l.darkCatchAll.Remove(socket)
	}
}

func (l *darkListener) AfterDarkIpAdded(addedIp string) {
	l.sockets.Range(func(_, queueInterface interface{}) bool {
		queue := queueInterface.(*IterableChan)
		toBeRemoved := make([]*RouterSocket, 0, 8)
		queue.Range(func(socketInterface interface{}) bool {
			socket := socketInterface.(*RouterSocket)
			if socket.Ip() == addedIp {
				var darkQu *IterableChan
				if darkQuInterface, ok := l.darkSockets.Load(socket.Route); !ok {
					darkQu = NewIterableChan(blockingQueueCapacity)
					l.darkSockets.Store(socket.Route, darkQu)
				} else {
					darkQu = darkQuInterface.(*IterableChan)
				}
				darkQu.Offer(socket)
				toBeRemoved = append(toBeRemoved, socket)
			}
			return true
		})
		for _, socket := range toBeRemoved {
			queue.Remove(socket)
		}
		return true
	})

	toBeRemoved := make([]*RouterSocket, 0, 8)
	l.catchall.Range(func(socketInterface interface{}) bool {
		socket := socketInterface.(*RouterSocket)
		if socket.Ip() == addedIp {
			l.darkCatchAll.Offer(socket)
			toBeRemoved = append(toBeRemoved, socket)
		}
		return true
	})
	for _, socket := range toBeRemoved {
		l.catchall.Remove(socket)
	}
}

func (l *darkListener) AfterDarkIpRevoked(revokedIp string) {
	darklaunch_manager.TurnGrayTestingOff("turn off gray testing after ip revoked")
	l.darkSockets.Range(func(_, queueInterface interface{}) bool {
		darkQueue := queueInterface.(*IterableChan)
		toBeRemoved := make([]*RouterSocket, 0, 8)
		darkQueue.Range(func(socketInterface interface{}) bool {
			socket := socketInterface.(*RouterSocket)
			if socket.Ip() == revokedIp {
				var qu *IterableChan
				if darkQuInterface, ok := l.darkSockets.Load(socket.Route); !ok {
					qu = NewIterableChan(blockingQueueCapacity)
					l.sockets.Store(socket.Route, qu)
				} else {
					qu = darkQuInterface.(*IterableChan)
				}
				qu.Offer(socket)
				toBeRemoved = append(toBeRemoved, socket)
			}
			return true
		})
		for _, socket := range toBeRemoved {
			darkQueue.Remove(socket)
		}
		return true
	})

	toBeRemoved := make([]*RouterSocket, 0, 8)
	l.darkCatchAll.Range(func(socketInterface interface{}) bool {
		socket := socketInterface.(*RouterSocket)
		if socket.Ip() == revokedIp {
			l.catchall.Offer(socket)
			toBeRemoved = append(toBeRemoved, socket)
		}
		return true
	})
	for _, socket := range toBeRemoved {
		l.darkCatchAll.Remove(socket)
	}
}
