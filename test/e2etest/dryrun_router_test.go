package e2etest

import (
	"testing"
	"time"
)

func TestDryRunRouter(t *testing.T) {
	app := CreateRouterApp(9000, 9070, 12439)
	app.Start()
	time.Sleep(60 * time.Minute)
}
