package scaffolding

import (
	"fmt"
	"net/url"
	"time"

	ct "github.com/torchcc/crank4go/connector"
	"github.com/torchcc/crank4go/connector/plugin"
	"github.com/torchcc/crank4go/util"
)

type ConnectorApp struct {
	healthURI              *url.URL
	healthServer           *RestfulServer
	connector              *ct.Connector
	connectorHealthService *ConnectorHealthService
}

func NewConnectorApp(routerURIs []*url.URL, targetURI *url.URL, targetServiceName string, healthPort, websocketPoolSize int, dataPublishHandlers []util.DataPublishHandler) *ConnectorApp {
	return NewConnectorApp2(routerURIs, targetURI, targetServiceName, "testService", healthPort,
		websocketPoolSize, dataPublishHandlers, nil)
}

func NewConnectorApp2(routerURIs []*url.URL, targetURI *url.URL, targetServiceName, componentName string, healthPort, websocketPoolSize int, dataPublishHandlers []util.DataPublishHandler, connectorPlugins []plugin.ConnectorPlugin) *ConnectorApp {
	connectorConfig := ct.NewConnectorConfig2(targetURI, targetServiceName, routerURIs, componentName, connectorPlugins).
		SetSlidingWindowSize(websocketPoolSize).
		SetDataPublishHandlers(dataPublishHandlers).
		SetIsShutDownHookAdded(false)

	connector := ct.CreateAndStartConnector(connectorConfig)
	connectorHealthService := NewConnectorHealthService(connector.ConnMonitor())
	healthServer := NewRestfulServer(healthPort, NewHealthServiceResource(connectorHealthService))
	healthURI, _ := url.Parse(fmt.Sprintf("http://localhost:%d/health", healthPort))
	return &ConnectorApp{
		healthURI:              healthURI,
		healthServer:           healthServer,
		connector:              connector,
		connectorHealthService: connectorHealthService,
	}
}

func (c *ConnectorApp) Start() {
	c.healthServer.Start()
	c.connectorHealthService.ScheduleHealthCheck()
}

func (c *ConnectorApp) ShutDownSuccessfully(timeout time.Duration) (succeeded bool) {
	succeeded = c.connector.ShutDownAfterTimeout(timeout)
	c.healthServer.ShutDown()
	return
}

func (c *ConnectorApp) ShutDown() {
	c.ShutDownSuccessfully(20 * time.Microsecond)
}

func (c *ConnectorApp) IdleWebsocketFarmInfoMap() map[string]int {
	return c.connector.IdleWebsocketFarmInfo()
}
