package connector

import (
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	. "github.com/torchcc/crank4go-core/util"
)

type WebsocketClientFarm struct {
	maxSlidingWindowSize       int
	websocketClientFarmInfoMap map[string]int
	connectorSockets           *sync.Map
}

// Set by service. maxSlidingWindowSize = 2 * slidingWindowSize because connector add and remove websocket in different goroutines,
// which may result in adding websocket goes before removing websocket.
func NewWebsocketClientFarm(slidingWindowSize int) *WebsocketClientFarm {
	return &WebsocketClientFarm{
		maxSlidingWindowSize:       slidingWindowSize * 2,
		websocketClientFarmInfoMap: nil,
		connectorSockets:           &sync.Map{},
	}
}

func (f *WebsocketClientFarm) addWebsocket(registerUrl string) {
	var atomicInt int32
	atomicInt = 0
	addr, _ := f.connectorSockets.LoadOrStore(registerUrl, &atomicInt)
	atomic.AddInt32(addr.(*int32), 1)
	LOG.Debugf("add websocket for registerUrl=%s, current websocketClientFarm=%#v", registerUrl, f)
}

func (f *WebsocketClientFarm) removeWebsocket(registerUrl string) {
	if addr, ok := f.connectorSockets.Load(registerUrl); ok {
		atomic.AddInt32(addr.(*int32), -1)
		LOG.Debugf("remove websocket for registerUrl=%s, current websocketClientFarm=%#v", registerUrl, f)
	}
}

func (f *WebsocketClientFarm) isSafeToAddWebsocket(registerUrl *url.URL) bool {
	isNotDeregisterPath := !strings.HasPrefix(registerUrl.Path, "/deregister")
	var idleSocketNum int32
	if addr, ok := f.connectorSockets.Load(registerUrl.String()); ok {
		idleSocketNum = *(addr.(*int32))
	}
	return isNotDeregisterPath && f.maxSlidingWindowSize > int(idleSocketNum)
}

func (f *WebsocketClientFarm) ToMap() map[string]int {
	f.websocketClientFarmInfoMap = make(map[string]int)
	f.connectorSockets.Range(func(key, value interface{}) bool {
		f.websocketClientFarmInfoMap[key.(string)] = int(*(value.(*int32)))
		return true
	})
	return f.websocketClientFarmInfoMap
}

type ConnectionInfo struct {
	routerURI       *url.URL
	connIndex       int
	curConnAttempts *int64
}

func NewConnectionInfo(routerURI *url.URL, connIndex int) *ConnectionInfo {
	return &ConnectionInfo{
		routerURI:       routerURI,
		connIndex:       connIndex,
		curConnAttempts: new(int64),
	}
}

func (ci *ConnectionInfo) OnConnectedSuccessfully() {
	atomic.StoreInt64(ci.curConnAttempts, 0)
}

func (ci *ConnectionInfo) OnConnectionStarting() {
	atomic.AddInt64(ci.curConnAttempts, 1)
}

func (ci *ConnectionInfo) RetryAfterMillis() int64 {
	return int64(500 * math.Min(10000, math.Pow(2, float64(atomic.LoadInt64(ci.curConnAttempts)))))
}

func (ci *ConnectionInfo) String() string {
	return "ConnectionInfo{" +
		"routerURI=" + ci.routerURI.String() +
		", connIndex=" + strconv.Itoa(ci.connIndex) +
		fmt.Sprintf(", curConnAttempts=%d", atomic.LoadInt64(ci.curConnAttempts)) +
		"}"
}
