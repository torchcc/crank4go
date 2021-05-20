package e2etest

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	connector "github.com/torchcc/crank4go-core/crank4go-connector-embedded"
	"github.com/torchcc/crank4go-core/crank4go-connector-embedded/plugin"
	router "github.com/torchcc/crank4go-core/crank4go-router-embedded"
	protocol "github.com/torchcc/crank4go-core/cranker-protocol"
	. "github.com/torchcc/crank4go-core/test/scaffolding"
	"github.com/torchcc/crank4go-core/util"
)

func TestCrankerWithAllExtensionSingleService(t *testing.T) {
	s := NewCrankerWithAllExtensionSingleService()
	s.start()
	defer s.stop()

	subUri, _ := url.Parse("/a/static/hello.html")
	resp, err := s.httpClient.Get(s.routerApp.HttpURI().ResolveReference(subUri).String())
	if err != nil {
		t.Errorf("failed to send request, err: %s", err.Error())
		return
	}
	if resp.StatusCode != 200 {
		t.Errorf("statusCode err: %s", err.Error())
		return
	}
	defer resp.Body.Close()
	contentBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed, err: %s", err.Error())
		return
	}
	if string(contentBytes) != HelloHtmlContents() {
		t.Errorf("failed, err: %s", err.Error())
		return
	}

	// test to access service which is not registered on router:
	subUri, _ = url.Parse("/b/static/hello.html")
	resp, err = s.httpClient.Get(s.routerApp.HttpURI().ResolveReference(subUri).String())
	if err != nil {
		t.Errorf("failed to send request, err: %s", err.Error())
		return
	}
	if resp.StatusCode != 503 {
		t.Errorf("statusCode err, expected 503, got: %d", resp.StatusCode)
		return
	}

}

type CrankerWithAllExtensionSingleService struct {
	serviceA                 *ContextualizedWebserver
	testProxyInterceptorStat *testStat
	testSocketStat           *testStat
	testConnectorPluginStat  *testStat
	routerHttpPort           int
	connectorHealthPort      int
	routerApp                *RouterApp
	connectorA               *ConnectorApp
	httpClient               *http.Client
}

func NewCrankerWithAllExtensionSingleService() *CrankerWithAllExtensionSingleService {
	testConnectorPluginStat := newTestStat()
	dataPublishHandlers := []util.DataPublishHandler{
		util.DataPublishHandlerFunc(func(key string, value int) { fmt.Printf("mykey: %s, myvalue: %d", key, value) }),
	}
	connectorHealthPort, _ := util.GetFreePort()
	routerHealthPort, _ := util.GetFreePort()
	serviceAHttpPort, _ := util.GetFreePort()
	routerHttpPort := 9000
	serviceA := NewContextualizedWebserver(serviceAHttpPort, "/a")
	routerApp := CreateRouterApp(routerHttpPort, 9070, routerHealthPort)
	connectorApp := NewConnectorApp2([]*url.URL{routerApp.RegisterURI(), routerApp.RegisterURI()}, serviceA.Uri,
		"a", "service-a", connectorHealthPort, 3, dataPublishHandlers, []plugin.ConnectorPlugin{newTestConnectorPlugin(testConnectorPluginStat)})

	t := &CrankerWithAllExtensionSingleService{
		serviceA:                 serviceA,
		testProxyInterceptorStat: newTestStat(),
		testSocketStat:           newTestStat(),
		testConnectorPluginStat:  testConnectorPluginStat,
		routerHttpPort:           routerHttpPort,
		connectorHealthPort:      connectorHealthPort,
		routerApp:                routerApp,
		connectorA:               connectorApp,
		httpClient:               connector.GetHttpClient(),
	}
	return t
}

func (s *CrankerWithAllExtensionSingleService) start() {
	util.LOG.Infof("service start is starting...")
	s.routerApp.Start()
	s.serviceA.Start()
	s.connectorA.Start()
	util.LOG.Infof("service start done")
}

func (s *CrankerWithAllExtensionSingleService) stop() {
	s.serviceA.ShutDown()
	s.connectorA.ShutDown()
	s.routerApp.Shutdown()
}

func CreateRouterApp(httpPort, registerPort, healthPort int) *RouterApp {
	connMonitor := util.NewConnectionMonitor([]util.DataPublishHandler{
		util.DataPublishHandlerFunc(func(key string, value int) { fmt.Printf("key: %s, value: %d", key, value) }),
	})

	routerConfig := router.NewRouterConfig("localhost", "localhost",
		registerPort, httpPort, GetTestTLSConfig(), GetTestTLSConfig())

	routerConfig.SetIsShutDownHookAdded(false).
		SetConnMonitor(connMonitor).
		ConfigDarkLaunch("abc", "dark-mode-service")
	return NewRouterApp2(routerConfig, healthPort)
}

type testStat struct {
	states []string
}

func newTestStat() *testStat {
	return &testStat{states: make([]string, 0, 8)}
}
func (s *testStat) UpdateState(state string) {
	s.states = append(s.states, state)
}
func (s *testStat) CurrentState() string {
	if l := len(s.states); l > 0 {
		return s.states[l-1]
	} else {
		return ""
	}
}

type testConnectorPlugin struct {
	testStat *testStat
}

func newTestConnectorPlugin(testStat *testStat) *testConnectorPlugin {
	return &testConnectorPlugin{testStat: testStat}
}

func (t *testConnectorPlugin) HandleBeforeRequestSent(req *protocol.CrankerProtocolRequest, carrier plugin.ConnectorPluginStatCarrier) (plugin.ConnectorPluginStatCarrier, error) {
	util.LOG.Infof("handleBeforeRequestSent %s", req.String())
	t.testStat.UpdateState(req.ToProtocolMessage())
	return carrier, nil
}

func (t *testConnectorPlugin) HandleAfterResponseReceived(resp *protocol.CrankerProtocolResponse, carrier plugin.ConnectorPluginStatCarrier) (plugin.ConnectorPluginStatCarrier, error) {
	util.LOG.Infof("HandleAfterResponseReceived %s", resp.String())
	t.testStat.UpdateState(resp.ToProtocolMessage())
	return carrier, nil
}
