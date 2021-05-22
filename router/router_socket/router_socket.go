package router_socket

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"
	ptc "github.com/torchcc/crank4go/protocol"
	"github.com/torchcc/crank4go/router/corsheader_processor"
	"github.com/torchcc/crank4go/router/plugin"
	"github.com/torchcc/crank4go/util"
)

const Authorization = "authorization"
const writeWait = time.Second

// RouterSocket It is a socket on router side for connector to send response content
type RouterSocket struct {
	RespHeadersNotSendBack map[string]struct{}
	Route                  string
	RouterSocketID         string
	connectorInstanceID    string
	connMonitor            *util.ConnectionMonitor
	websocketFarm          *WebsocketFarm
	isRegister             bool // true if the socket is from a registration to server HTTP request, false if it's a deRegistration of a connector
	corsHeaderProcessor    *corsheader_processor.CorsHeaderProcessor
	session                *ws.Conn
	respWriter             http.ResponseWriter
	req                    *http.Request
	handleDone             *sync.WaitGroup
	onReadyToAct           func()
	remoteAddr             string
	isRemoved              bool
	hasResp                bool
	lastPingTime           time.Time
	reqStartTime           time.Time
	bytesReceived          int64
	bytesSent              int64
	ip                     string
	reqComponentName       string
	routerSocketPlugins    []plugin.RouterSocketPlugin
}

func NewRouterSocket(route string, connMonitor *util.ConnectionMonitor, websocketFarm *WebsocketFarm,
	connectorInstanceID string, isRegister bool, ip string, corsHeaderProcessor *corsheader_processor.CorsHeaderProcessor) *RouterSocket {
	return NewRouterSocket2(route, connMonitor, websocketFarm, connectorInstanceID, isRegister, ip, corsHeaderProcessor, nil)
}

func NewRouterSocket2(route string, connMonitor *util.ConnectionMonitor, websocketFarm *WebsocketFarm,
	connectorInstanceID string, isRegister bool, ip string, corsHeaderProcessor *corsheader_processor.CorsHeaderProcessor, routerSocketPlugins []plugin.RouterSocketPlugin) *RouterSocket {
	s := &RouterSocket{
		RespHeadersNotSendBack: map[string]struct{}{"server": {}},
		Route:                  route,
		RouterSocketID:         uuid.New().String(),
		connectorInstanceID:    connectorInstanceID,
		connMonitor:            connMonitor,
		websocketFarm:          websocketFarm,
		isRegister:             isRegister,
		corsHeaderProcessor:    corsHeaderProcessor,
		ip:                     ip,
	}
	if !util.UUI51288AllowFixedLengthResponses {
		s.RespHeadersNotSendBack["content-length"] = struct{}{}
	}
	if routerSocketPlugins != nil {
		s.routerSocketPlugins = routerSocketPlugins
	} else {
		s.routerSocketPlugins = make([]plugin.RouterSocketPlugin, 0, 0)
	}
	util.LOG.Debugf("an adding websocket request received, routerName=%s, routerSocketID=%s", route, s.RouterSocketID)
	return s
}

func (s *RouterSocket) String() string {
	return fmt.Sprintf("RouterSocket{route=%s, routerSocketID=%s, connectorInstanceID=%s, isRegister=%v, ip=%s, requestComponentName=%s}",
		s.Route, s.RouterSocketID, s.connectorInstanceID, s.isRegister, s.ip, s.reqComponentName)
}

func (s *RouterSocket) IsCatchAll() bool {
	return s.Route == ""
}

// RemoteAddr the websocket connection's client peer's host and port
func (s *RouterSocket) RemoteAddr() string {
	return s.remoteAddr
}

func (s *RouterSocket) LastPingTime() time.Time {
	return s.lastPingTime
}

func (s *RouterSocket) ConnectorInstanceID() string {
	return s.connectorInstanceID
}

func (s *RouterSocket) ReqComponentName() string {
	return s.reqComponentName
}

func (s *RouterSocket) SetReqComponentName(reqComponentName string) {
	s.reqComponentName = reqComponentName
}

func (s *RouterSocket) Ip() string {
	return s.ip
}

func (s *RouterSocket) SetResponse(respWriter http.ResponseWriter, req *http.Request, handleDone *sync.WaitGroup) {
	s.connMonitor.OnConnectionStarted2(s.Route, s.RouterSocketID)
	s.req = req
	s.respWriter = respWriter
	s.handleDone = handleDone
	s.reqStartTime = time.Now()
}

func (s *RouterSocket) SetOnReadyToAct(action func()) {
	s.onReadyToAct = action
}

func (s *RouterSocket) SendPingToConnector() {
	if s.session != nil {
		util.LOG.Debugf("sending ping message... routerName=%s, routerSocketID=%s, lastPingTime=%#v, connectorInstanceID=%s",
			s.Route, s.RouterSocketID, s.lastPingTime, s.connectorInstanceID)
		if err := s.session.WriteControl(ws.PingMessage, []byte("*ping*_%"), time.Now().Add(time.Second)); err != nil {
			util.LOG.Debugf("failed to send ping, killing bad websocket... routerSocketID: %s, err: %s", s.RouterSocketID, err.Error())
			s.removeBadWebsocket()
		}
		s.lastPingTime = time.Now()
	}
}

// OnWebsocketClose A Close Event was received. The underlying Connection will be considered closed at this point.
func (s *RouterSocket) OnWebsocketClose(statusCode int, reason string) error {
	util.LOG.Debugf("router side got closeMessage, statusCode=%d, reason=%s, routerName=%s, routerSocketID=%s",
		statusCode, reason, s.Route, s.RouterSocketID)
	s.session = nil
	// status code: https://tools.ietf.org/html/rfc6455#section-7.4.1
	if s.respWriter != nil {
		s.connMonitor.OnConnectionEnded3(s.RouterSocketID, s.Route, s.reqComponentName, 200,
			time.Now().Sub(s.reqStartTime).Milliseconds(), s.bytesSent, s.bytesReceived)
		if statusCode == ws.CloseInternalServerErr {
			s.respWriter.WriteHeader(http.StatusBadGateway)
			util.LOG.Debugf("client response is %#v, routerName=%s, routerSocketID=%s", s.respWriter, s.Route, s.RouterSocketID)
		} else if statusCode == ws.ClosePolicyViolation {
			s.respWriter.WriteHeader(http.StatusBadRequest)
			util.LOG.Debugf("client response is %#v, routerName=%s, routerSocketID=%s", s.respWriter, s.Route, s.RouterSocketID)
		}
	}
	if s.handleDone != nil {
		s.handleDone.Done()
		s.handleDone = nil
	}
	if s.isRegister && !s.isRemoved {
		util.LOG.Debugf("going to remove socket, statusCode=%d, reason=%s, routerName=%s, routerSocketID=%s",
			statusCode, reason, s.Route, s.RouterSocketID)
		s.websocketFarm.RemoveWebsocket(s.Route, s)
		s.isRemoved = true
	}
	return nil
}

func (s *RouterSocket) OnWebsocketConnect(session *ws.Conn) {
	s.session = session
	s.remoteAddr = session.RemoteAddr().String()
	if s.isRegister {
		s.onReadyToAct()
	}
	s.runForever(s.session)
}

func (s *RouterSocket) OnWebsocketText(msg string) {
	util.LOG.Debugf("cranker router socket received response from service connector onWebsocketText=%s", msg)
	if s.respWriter != nil {
		atomic.AddInt64(&s.bytesReceived, int64(len(msg)))
		ptcResp := ptc.NewCrankerProtocolResponse(msg)
		if ptc.IsDebugResp(ptcResp) {
			util.LOG.Infof("onWebsocketText: cranker router receive response from service connector,"+
				" routeName: %s, routerSocketID: %s, msg: %s", s.Route, s.RouterSocketID, msg)
		}
		util.LOG.Debugf("onWebsocketText: cranker router receive response from service connector,"+
			" routeName: %s, routerSocketID: %s, msg: %s, response status: %s", s.Route, s.RouterSocketID, msg, ptcResp.Status())

		for _, p := range s.routerSocketPlugins {
			if err := p.HandleAfterRespReceived(ptcResp); err != nil {
				util.LOG.Errorf("failed to apply the plugin [%#v] on response %s, err: %s", p, ptcResp.ToProtocolMessage(), err.Error())
			}
		}
		s.putHeadersTo(ptcResp)
		s.respWriter.WriteHeader(ptcResp.Status())
		// s.corsHeaderProcessor.Process(s.req, s.respWriter)
	}
}

// OnWebsocketBinary please make sure the there is available data in buf before calling this method
func (s *RouterSocket) OnWebsocketBinary(buf []byte) {
	atomic.AddInt64(&s.bytesReceived, int64(len(buf)))
	util.LOG.Debugf("router with routerName: %s, routerSocketID: %s is sending %d bytes to connector",
		s.Route, s.RouterSocketID, len(buf))

	if _, err := s.respWriter.Write(buf); err != nil {
		util.LOG.Errorf("router with routerName: %s, routerSocketID: %s cannot write to client response writer "+
			"(maybe the user closed their browser) so the request is cancelling. err: %s", s.Route, s.RouterSocketID, err.Error())
		if s.handleDone != nil {
			util.LOG.Debugf("going to done, socketID: %s", s.RouterSocketID)
			s.handleDone.Done() // just in case router side failed to get websocket closeMessage.
			s.handleDone = nil
		}
		s.CloseSocketSession()
	}
}

// OnWebsocketError A WebSocket exception has occurred.
// This is a way for the internal implementation to notify of exceptions occured during the processing of websocket.
// Usually this occurs from bad / malformed incoming packets. (example: bad UTF8 data, frames that are too big, violations of the spec)
// This will result in the {@link Session} being closed by the implementing side.
func (s *RouterSocket) OnWebsocketError(cause error) {
	util.LOG.Errorf("websocket error occurs when websocket server side receiving reading msg from session, err: %s", cause.Error())
	s.OnSendOrReceiveDataError(cause)
}

func (s *RouterSocket) SendText(msg string) error {
	atomic.AddInt64(&s.bytesSent, int64(len(msg)))
	return s.session.WriteMessage(ws.TextMessage, []byte(msg))
}

func (s *RouterSocket) SendData(buf []byte) error {
	atomic.AddInt64(&s.bytesSent, int64(len(buf)))
	return s.session.WriteMessage(ws.BinaryMessage, buf)
}

// OnSendOrReceiveDataError this will be called when reverseProxy failed too send textMessage or binaryMessage to connector
func (s *RouterSocket) OnSendOrReceiveDataError(err error) {
	s.removeBadWebsocket()
	errMsg := err.Error()
	if strings.Contains(errMsg, "timeout") {
		util.LOG.Warning("hit timeout err when sending data from router to connector, err: %s", err)
		if s.respWriter != nil {
			s.respWriter.WriteHeader(http.StatusGatewayTimeout)
			if _, e := s.respWriter.Write([]byte(fmt.Sprintf("504 Gateway Timeout, err: %s", err.Error()))); e != nil {
				util.LOG.Error("failed to write response from router reverseProxy to client, err: %s", err.Error())
			}
		}
	} else {
		if s.respWriter != nil {
			s.respWriter.WriteHeader(http.StatusBadGateway)
			if _, e := s.respWriter.Write([]byte(fmt.Sprintf("502 Bad Gateway, err: %s", err.Error()))); e != nil {
				util.LOG.Error("failed to write response from router reverseProxy to client, err: %s", err.Error())
			}
		}
	}
	if s.handleDone != nil {
		util.LOG.Debugf("OnSendOrReceiveDataError: going to done, socketID: %s", s.RouterSocketID)
		s.handleDone.Done()
		s.handleDone = nil
	}
}

func (s *RouterSocket) removeBadWebsocket() {
	if !s.isRemoved {
		s.CloseSocketSession()
		s.websocketFarm.RemoveWebsocket(s.Route, s)
		s.isRemoved = true
	}
}

func (s *RouterSocket) CloseSocketSession() {
	util.LOG.Debugf("closing socketSession %s ...", s.String())
	if s.session != nil {
		_ = s.session.WriteControl(ws.CloseGoingAway, []byte("Going away"), time.Now().Add(time.Second))
		s.session = nil
	}
}

// response statusCode must be written after this function is called, as is said by golang http package
func (s *RouterSocket) putHeadersTo(ptcResp *ptc.CrankerProtocolResponse) {
	for _, line := range ptcResp.Headers {
		if pos := strings.Index(line, ":"); pos > 0 {
			header := line[:pos]
			if _, ok := s.RespHeadersNotSendBack[strings.ToLower(header)]; !ok && s.respWriter != nil {
				value := line[pos+1:]
				v := value
				if Authorization == strings.ToLower(header) {
					v = strings.Split(value, ":")[0]
				}
				util.LOG.Debugf("sending client response header %s=%s", header, v)
				s.respWriter.Header().Add(header, value)
			}
		}
	}
	s.respWriter.Header().Add("Via", "1.1 crnk")
}

func (s *RouterSocket) CorsHeaderProcessor() *corsheader_processor.CorsHeaderProcessor {
	return s.corsHeaderProcessor
}

func (s *RouterSocket) runForever(conn *ws.Conn) {
	for {
		// msgType must be either TextMessage or BinaryMessage
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if closeErr, ok := err.(*ws.CloseError); !ok { // conn polling got CloseMessage, execute closeHandler, read again and got CloseError, so if we got CloseError, just ignore it
				s.OnWebsocketError(err)
			} else {
				_ = s.OnWebsocketClose(closeErr.Code, closeErr.Text)
			}
			return
		}
		if msgType == ws.TextMessage {
			s.OnWebsocketText(string(msg))
		} else if msgType == ws.BinaryMessage {
			s.OnWebsocketBinary(msg)
		} else {
			util.LOG.Errorf("unexpected msgType got: %d", msgType)
		}
	}
}
