package e2etest

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	. "github.com/torchcc/crank4go-core/crank4go-connector-embedded"
	. "github.com/torchcc/crank4go-core/test/scaffolding"
)

// upload img done, get done, post done , all method done, connector is ok.
func TestDryRunConnector(t *testing.T) {
	targetURI, _ := url.Parse("http://localhost:5000")
	routerURI, _ := url.Parse("wss://localhost:16488")

	connectorConfig := NewConnectorConfig2(targetURI, "z", []*url.URL{routerURI}, "z-service", nil).
		SetSlidingWindowSize(100).
		SetIsShutDownHookAdded(false)

	connector := CreateAndStartConnector(connectorConfig)
	fmt.Println(connector)

	serviceA := NewContextualizedWebserver(5000, "/z")
	serviceA.Start()
	time.Sleep(1000 * time.Minute)

}

// upload img done, get done, post done , all method done, connector is ok.
func TestDryRunConnector2(t *testing.T) {
	targetURI, _ := url.Parse("http://localhost:5000")
	routerURI, _ := url.Parse("wss://localhost:9070")

	connectorConfig := NewConnectorConfig2(targetURI, "z", []*url.URL{routerURI}, "z-service", nil).
		SetSlidingWindowSize(300).
		SetIsShutDownHookAdded(false)

	connector := CreateAndStartConnector(connectorConfig)
	fmt.Println(connector)

	serviceA := NewContextualizedWebserver(5000, "/z")
	serviceA.Start()
	time.Sleep(1000 * time.Minute)
}
