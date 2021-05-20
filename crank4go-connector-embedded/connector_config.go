package connector

import (
	"net/url"

	"github.com/google/uuid"
	"github.com/torchcc/crank4go-core/crank4go-connector-embedded/plugin"
	"github.com/torchcc/crank4go-core/util"
)

type ConnectorConfig struct {
	slidingWindowSize   int
	isShutDownHookAdded bool
	targetServiceName   string
	instanceID          string
	componentName       string
	plugins             []plugin.ConnectorPlugin
	routerURIs          []*url.URL
	dataPublishHandlers []util.DataPublishHandler
	targetURI           *url.URL
}

func NewConnectorConfig(targetURI *url.URL, targetServiceName string, routerURIs []*url.URL, componentName string) *ConnectorConfig {
	return NewConnectorConfig2(targetURI, targetServiceName, routerURIs, componentName, make([]plugin.ConnectorPlugin, 0, 0))
}

func NewConnectorConfig2(targetURI *url.URL, targetServiceName string, routerURIs []*url.URL, componentName string, plugins []plugin.ConnectorPlugin) *ConnectorConfig {
	return &ConnectorConfig{
		slidingWindowSize:   2,
		isShutDownHookAdded: false,
		targetServiceName:   targetServiceName,
		instanceID:          uuid.New().String(),
		componentName:       componentName,
		plugins:             plugins,
		routerURIs:          routerURIs,
		dataPublishHandlers: make([]util.DataPublishHandler, 0, 0),
		targetURI:           targetURI,
	}

}

func (c *ConnectorConfig) SetIsShutDownHookAdded(isAddShutDownHook bool) *ConnectorConfig {
	c.isShutDownHookAdded = isAddShutDownHook
	return c
}

func (c *ConnectorConfig) IsShutDownHookAdded() bool {
	return c.isShutDownHookAdded
}

func (c *ConnectorConfig) DataPublishHandlers() []util.DataPublishHandler {
	return c.dataPublishHandlers
}

// you can subscribe to this to see various metrics
func (c *ConnectorConfig) SetDataPublishHandlers(dataPublishHandlers []util.DataPublishHandler) *ConnectorConfig {
	if dataPublishHandlers != nil {
		c.dataPublishHandlers = dataPublishHandlers
	}
	return c
}

func (c *ConnectorConfig) Plugins() []plugin.ConnectorPlugin {
	return c.plugins
}

func (c *ConnectorConfig) TargetURI() *url.URL {
	return c.targetURI
}

func (c *ConnectorConfig) TargetServiceName() string {
	return c.targetServiceName
}

func (c *ConnectorConfig) RouterURIs() []*url.URL {
	return c.routerURIs
}

func (c *ConnectorConfig) SlidingWindowSize() int {
	return c.slidingWindowSize
}

// controls the idle socket windows of the pool size. please do not set this parameter unless you understand you need more
// max_value is 100
func (c *ConnectorConfig) SetSlidingWindowSize(slidingWindowSize int) *ConnectorConfig {
	c.slidingWindowSize = 2
	if slidingWindowSize > 0 && slidingWindowSize <= 1000 {
		c.slidingWindowSize = slidingWindowSize
	}
	return c
}

func (c *ConnectorConfig) ComponentName() string {
	return c.componentName
}

// returns a unique ID for this instance of the connector, This is visible to the router
func (c *ConnectorConfig) InstanceID() string {
	return c.instanceID
}
