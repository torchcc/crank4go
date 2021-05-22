package connector

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	ws "github.com/gorilla/websocket"
	. "github.com/torchcc/crank4go/connector/plugin"
	ptc "github.com/torchcc/crank4go/protocol"
	. "github.com/torchcc/crank4go/util"
)

type State int

const (
	NotStarted   State = iota // value --> 0
	RUNNING                   // value --> 1
	ShuttingDown              // value --> 2
	ShutDown                  // value --> 3
	// idleTimeout  = 60 * 1000        // mili second
)

type Connector struct {
	slidingWindowSize   int
	ParentCtx           context.Context
	ParentCancel        context.CancelFunc
	routerURIs          []*url.URL
	targetURI           *url.URL
	connMonitor         *ConnectionMonitor
	websocketClientFarm *WebsocketClientFarm
	websocketDialer     *ws.Dialer
	targetServiceName   string
	connectorInstanceID string
	componentName       string
	connectorPlugins    []ConnectorPlugin
	state               State
}

func NewConnector(routerURIs []*url.URL, targetURI *url.URL, targetServiceName string, slidingWindowSize int,
	connMonitor *ConnectionMonitor, connectorInstanceID string, componentName string,
	connectorPlugins []ConnectorPlugin) *Connector {

	c := &Connector{
		slidingWindowSize:   slidingWindowSize,
		routerURIs:          routerURIs,
		targetURI:           targetURI,
		connMonitor:         connMonitor,
		websocketClientFarm: NewWebsocketClientFarm(slidingWindowSize),
		websocketDialer:     GetWebsocketDialer(),
		targetServiceName:   targetServiceName,
		connectorInstanceID: connectorInstanceID,
		componentName:       componentName,
		state:               NotStarted,
	}
	if connectorPlugins != nil {
		c.connectorPlugins = connectorPlugins
	} else {
		c.connectorPlugins = make([]ConnectorPlugin, 0, 0)
	}
	c.ParentCtx, c.ParentCancel = context.WithCancel(context.Background())
	return c
}

func (c *Connector) ConnMonitor() *ConnectionMonitor {
	return c.connMonitor
}
func (c *Connector) Start() {
	for _, routerURI := range c.routerURIs {
		registerURI := routerURI.ResolveReference(&url.URL{
			Path:     "register/",
			RawQuery: fmt.Sprintf("connectorInstanceID=%s&componentName=%s", c.connectorInstanceID, c.componentName),
		})
		LOG.Infof("Connecting to %s", registerURI.String())
		for i := 0; i < c.slidingWindowSize; i++ {
			connInfo := NewConnectionInfo(registerURI, i)
			c.connectToRouter(registerURI, connInfo)
			c.websocketClientFarm.addWebsocket(registerURI.String())
			c.connMonitor.OnConnectionAvailable()
		}
	}
	LOG.Infof("connector started for component=%s for path=/%s", c.componentName, c.targetServiceName)
	c.state = RUNNING
}

func (c *Connector) connectToRouter(registerURI *url.URL, connInfo *ConnectionInfo) {
	LOG.Debugf("connecting to router, registerURI is %s", registerURI)
	socket := NewConnectorSocket(registerURI, c.targetURI, c.connMonitor, connInfo, c.connectorPlugins, c.websocketClientFarm, c.componentName, c.ParentCtx)

	// whenConsumedAction
	runnable := func() {
		c.connMonitor.OnConnectionConsumed()
		if c.state == ShuttingDown || c.state == ShutDown {
			LOG.Infof("connector {%s} will not reconnect to router as it is being shut down", c.connectorInstanceID)
		} else if c.websocketClientFarm.isSafeToAddWebsocket(registerURI) {
			LOG.Debugf("connector {%s} is adding another connectorSocket...", c.connectorInstanceID)
			c.connectToRouter(socket.RegisterURI(), connInfo)
			c.websocketClientFarm.addWebsocket(socket.RegisterURI().String())
			c.connMonitor.OnConnectionAvailable()
		} else {
			LOG.Warningf("unexpected error happened, no websocket will be added for connector {%s}, current websocketClientFarm is {%#v}", c.connectorInstanceID, c.websocketClientFarm)
		}
	}

	socket.WhenConsumed(runnable)

	if c.state != ShutDown {
		connInfo.OnConnectionStarting()
		headers := make(http.Header)
		headers.Add("CrankerProtocol", ptc.CrankerProtocolVersion10)
		headers.Add("Route", c.targetServiceName)
		go func() {
			if conn, _, err := c.websocketDialer.Dial(registerURI.String(), headers); err != nil {
				LOG.Errorf("cannot replace socket for %s, err: %s", registerURI, err.Error())
				socket.OnWebsocketError(err)
			} else {
				LOG.Debugf("connected to router, register url: %s", registerURI)
				socket.OnWebsocketConnect(conn)
			}
		}()
	}
}

// IdleWebsocketFarmInfo return websocketClientFarm info as Map: <router register url, idleConnection number>
func (c *Connector) IdleWebsocketFarmInfo() map[string]int {
	return c.websocketClientFarm.ToMap()
}

// ShutDown shutdown the connection to the router, waiting up to 20 seconds for existing requests to complete
// any requests not finished in that time will be terminated.
func (c *Connector) ShutDown() {
	c.ShutDownAfterTimeout(30 * time.Second)
}

// ShutDownAfterTimeout gracefully shuts down the connection to the router, any in-flight requests will continue to be process
// until the timeout limit. this method will block until all requests are finished or the timeout is reached.
// so after it returns you can shutdown your webServer. Note: on timeout, false will be returned as an indicator
// if there are some remaining requests terminated due to timeout
func (c *Connector) ShutDownAfterTimeout(timeout time.Duration) bool {
	endTime := time.Now().Add(timeout)
	if c.state != ShutDown {
		c.callShutDown()
	}
	for {
		if c.connMonitor.ConnectionCount() == 0 {
			return true
		}
		if time.Now().After(endTime) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (c *Connector) callShutDown() {
	c.state = ShuttingDown

	c.ParentCancel() // cancel alive pingTask, stop reconnectOnError mechanism
	for _, routerURI := range c.routerURIs {
		deRegisterURI := routerURI.ResolveReference(&url.URL{
			Path:     "deregister/",
			RawQuery: fmt.Sprintf("connectorInstanceID=%s&componentName=%s", c.connectorInstanceID, c.componentName),
		})
		LOG.Infof("disconnecting... deregister URL: %s", deRegisterURI)
		deRegisterInfo := NewConnectionInfo(deRegisterURI, 0)
		c.connectToRouter(deRegisterURI, deRegisterInfo)
	}
	LOG.Infof("After deregister to router, AvailableConnections=%d, ConnectionCount=%d, OpenFiles=%d",
		c.connMonitor.AvailableConns(), c.connMonitor.ConnectionCount(), c.connMonitor.OpenFiles())

	c.state = ShutDown
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
	ExitProgram(exitFunc)
}

func CreateAndStartConnector(c *ConnectorConfig) *Connector {
	connMonitor := NewConnectionMonitor(c.dataPublishHandlers)
	connector := NewConnector(c.RouterURIs(), c.TargetURI(), c.TargetServiceName(), c.SlidingWindowSize(),
		connMonitor, c.InstanceID(), c.ComponentName(), c.Plugins())
	connector.Start()
	if c.IsShutDownHookAdded() {
		addShutDownHook(connector.ShutDown)
	}
	return connector
}
