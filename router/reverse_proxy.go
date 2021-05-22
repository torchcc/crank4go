package router

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/julienschmidt/httprouter"
	ptc "github.com/torchcc/crank4go/protocol"
	itc "github.com/torchcc/crank4go/router/interceptor"
	"github.com/torchcc/crank4go/router/router_socket"
	"github.com/torchcc/crank4go/util"
)

var (
	ip      string
	bufPool *sync.Pool
)

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		util.LOG.Infof("cannot get local ip.")
		return "unknown"
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	util.LOG.Infof("cannot get local ip.")
	return "unknown"
}

func init() {
	ip = GetLocalIP()
}

type ReverseProxy struct {
	// see https://tools.ietf.org/html/draft-ietf-httpbis-p1-messaging-14#section-7.1.3
	HopByHopHeadersFields map[string]struct{}
	websocketFarm         *router_socket.WebsocketFarm
	reqComponentHeader    string
	interceptors          []itc.ProxyInterceptor
}

func NewReverseProxy(farm *router_socket.WebsocketFarm, reqComponentHeader string, interceptors []itc.ProxyInterceptor) *ReverseProxy {
	proxy := &ReverseProxy{
		HopByHopHeadersFields: map[string]struct{}{
			"Connection": {}, "Keep-Alive": {}, "Proxy-Authenticate": {}, "Proxy-Authorization": {},
			"TE": {}, "Trailer": {}, "Transfer-Encoding": {}, "Upgrade": {}},
		websocketFarm:      farm,
		reqComponentHeader: reqComponentHeader,
		interceptors:       interceptors,
	}
	if interceptors == nil {
		proxy.interceptors = make([]itc.ProxyInterceptor, 0, 8)
	}
	util.LOG.Debugf("created reverse proxy: %s", proxy)
	return proxy
}

func (p *ReverseProxy) String() string {
	return fmt.Sprintf("ReverseProxy{websocketFarm=%v, requestComponentHeader=%s, interceptors=%s}",
		p.websocketFarm, p.reqComponentHeader, p.interceptors)
}

func (p *ReverseProxy) Handle(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool {
	var (
		crankedSocket *router_socket.RouterSocket
		target        = r.URL.Path
		componentName = p.componentNameFromHeader(r)
		err           error
	)
	util.LOG.Debugf("websocketFarm target: %s, component: %s", target, componentName)
	if crankedSocket, err = p.websocketFarm.AcquireSocket(target, componentName); err != nil {
		if routeErr, ok := err.(util.NoRouteErr); ok {
			util.LOG.Errorf("failed to forward target %s, NoRouteErr occurs, err: %s", target, routeErr)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Service Not Found"))
			return true
		} else { // TODO should assert if err is timeoutErr or not ?
			util.LOG.Errorf("failed to forward target %s, after timeout, err: %s", target, routeErr)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("No crankers Available"))
			return true
		}
	}
	util.LOG.Infof("proxying to target service, forwarding %s from %s to %s, "+
		"connectorID=%s, requestComponentName=%s", target, r.RemoteAddr, crankedSocket.RemoteAddr(),
		crankedSocket.ConnectorInstanceID(), componentName)
	handleDone := &sync.WaitGroup{} // its Done method must be called only after the writing to respWriter is finished.
	handleDone.Add(1)
	p.sendRequestOverWebsocket(r, w, crankedSocket, handleDone)
	handleDone.Wait()
	return true

}

func (p *ReverseProxy) componentNameFromHeader(r *http.Request) (name string) {
	if strings.TrimSpace(p.reqComponentHeader) != "" {
		name = r.Header.Get(strings.TrimSpace(p.reqComponentHeader))
	}
	return
}

func (p *ReverseProxy) sendRequestOverWebsocket(cliReq *http.Request, respWriter http.ResponseWriter, socket *router_socket.RouterSocket, handleDone *sync.WaitGroup) {
	socket.SetResponse(respWriter, cliReq, handleDone)

	ptcReqBuilder := new(ptc.CrankerProtocolRequestBuilder)
	ptcReqBuilder.WithReqLine(createReqLine(cliReq))

	headers := new(ptc.HeadersBuilder)
	hasBody := p.setTargetReqHeaders(cliReq, headers)
	ptcReqBuilder.WithReqHeaders(headers)

	var (
		carriers = itc.NewInterceptorStatCarriers()
		ptcMsg   string
		err      error
		buf      []byte
		n        int // offset, use to mark how many bytes have been read from request body
	)
	defer func() {
		if e := recover(); e != nil {
			p.onProxyingError(cliReq, socket, e.(error))
		}
		if buf != nil {
			bufPool.Put(buf)
		}
		_ = carriers.Close()
		util.LOG.Infof("finish - API %s being called by client %s through %s", cliReq.URL.String(), cliReq.RemoteAddr, cliReq.Method)
	}()

	if hasBody {
		// stream the body
		ptcMsg = ptcReqBuilder.WithReqBodyPending().Build()
		if err = socket.SendText(ptcMsg); err != nil {
			panic(err)
		}
		// read http request Body and send to connector through websocket
		buf = bufPool.Get().([]byte)
		for {
			n, err = cliReq.Body.Read(buf)
			if n > 0 {
				util.LOG.Debugf("about to send %s bytes to connector", n)
				if e := socket.SendData(buf[:n]); e != nil {
					panic(err)
				}
			}
			if err == io.EOF {
				err = nil
				break
			} else if err != nil {
				panic(err)
			}
		}
		if err = socket.SendText(new(ptc.CrankerProtocolRequestBuilder).WithReqBodyEnded().Build()); err != nil {
			panic(err)
		}
	} else {
		ptcMsg = ptcReqBuilder.WithReqHasNoBody().Build()
		if err := socket.SendText(ptcMsg); err != nil {
			panic(err)
		}
	}
	ptcReq := ptc.NewCrankerProtocolRequest(ptcMsg)
	if err = p.handleReqWithInterceptors(cliReq, carriers, ptcReq); err != nil {
		util.LOG.Errorf("failed to handleReqWithInterceptors, err: %s", err.Error())
	}
	if ptc.IsDebugReq(ptcReq) {
		util.LOG.Infof("handle -> before proxy forwarding, request is %s", ptcReq.ToProtocolMessage())
	}
}

func (p *ReverseProxy) setTargetReqHeaders(req *http.Request, headersBuilder *ptc.HeadersBuilder) bool {
	connHeaders := req.Header.Values("Connection")
	var (
		hasContentLength,
		hasTransferEncodingHeader bool
	)
	for headerName, headerValues := range req.Header {
		hasContentLength = hasContentLength || "content-length" == strings.ToLower(headerName)
		hasTransferEncodingHeader = hasContentLength || "transfer-encoding" == strings.ToLower(headerName)
		if !p.shouldSendHeaderFromClientToTarget(headerName, connHeaders) {
			continue
		}
		for _, value := range headerValues {
			headersBuilder.AppendHeader(headerName, value)
		}
	}
	addProxyForwardingHeaders(headersBuilder, req)
	return hasContentLength || hasTransferEncodingHeader

}

func addProxyForwardingHeaders(headers *ptc.HeadersBuilder, req *http.Request) {
	xfor := req.RemoteAddr
	proto := req.URL.Scheme
	host := req.Host
	by := ip
	headers.AppendHeader("Forwarded", fmt.Sprintf("for=%s;proto=%s;host=%s;by=%s", xfor, proto, host, by))
	if req.Header.Get("X-Forwarded-For") == "" {
		headers.AppendHeader("X-Forwarded-For", xfor)
	}
	if req.Header.Get("X-Forwarded-Proto") == "" {
		headers.AppendHeader("X-Forwarded-Proto", proto)
	}
	if req.Header.Get("X-Forwarded-Host") == "" {
		headers.AppendHeader("X-Forwarded-Host", host)
	}
	if req.Header.Get("X-Forwarded-Server") == "" {
		headers.AppendHeader("X-Forwarded-Server", by)
	}
}

func (p *ReverseProxy) shouldSendHeaderFromClientToTarget(headerName string, connHeaders []string) bool {
	if util.UUI31223NotSendingHostHeader && "Host" == headerName {
		return false
	}
	if _, ok := p.HopByHopHeadersFields[headerName]; ok {
		return false
	}
	for _, v := range connHeaders {
		if v == headerName {
			return false
		}
	}
	return true
}

func (p *ReverseProxy) handleReqWithInterceptors(request *http.Request, carriers *itc.InterceptorStatCarriers, ptcReq *ptc.CrankerProtocolRequest) error {
	for _, interceptor := range p.interceptors {
		util.LOG.Debugf("applying interceptor %#v on request %s", interceptor, ptcReq)
		if carrier, err := interceptor.ApplyOnReq(request, ptcReq); err != nil {
			util.LOG.Errorf("failed to apply the plugin [%#v] on request %s, err: %s", interceptor, ptcReq.ToProtocolMessage(), err.Error())
			return err
		} else {
			carriers.Add(interceptor, carrier)
		}
	}
	return nil
}

func (p *ReverseProxy) onProxyingError(req *http.Request, crankedSocket *router_socket.RouterSocket, err error) {
	errMsg := fmt.Sprintf("failed to proxy API %s being called by client %s through %s, err: %s",
		req.URL.String(), req.RemoteAddr, req.Method, err.Error())
	util.LOG.Error(errMsg)
	crankedSocket.OnSendOrReceiveDataError(err)
}

func createReqLine(req *http.Request) string {
	// Request-Line Method SP Request-HttpURI SP HTTP-Version CRLF
	uri := req.URL.Path
	qs := req.URL.RawQuery
	if qs != "" {
		qs = "?" + qs
	}
	return req.Method + " " + uri + qs + " HTTP/1.1"
}

func init() {
	bufPool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, 4096) // java use 2048
		},
	}
}
