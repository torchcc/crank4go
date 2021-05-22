package connector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"
	. "github.com/torchcc/crank4go/connector/plugin"
	. "github.com/torchcc/crank4go/protocol"
	. "github.com/torchcc/crank4go/util"
)

// if the msg are not written back within this time span, the writing will be aborted
const writeWait = time.Second

type ConnectorSocket struct {
	SockId                       uuid.UUID
	httpClient                   *http.Client
	targetURI                    *url.URL
	registerURI                  *url.URL
	connMonitor                  *ConnectionMonitor
	connInfo                     *ConnectionInfo
	createTime                   int64
	session                      *ws.Conn
	requestToTarget              *IntermediateRequest
	whenConsumedAction           func()
	websocketClientFarm          *WebsocketClientFarm
	componentName                string
	targetRequestContentProvider *io.PipeReader
	targetRequestContentWriter   *io.PipeWriter
	cancelPing                   context.CancelFunc // it's used to cancelPing pingTask when a socket dies naturally. it is idempotent
	parentCtx                    context.Context    // comes from the connector, use to stop pingTask and prevent error-reconnect-mechanism
	requestComplete              bool               // does it need to guarantee atomicity ?
	newSocketAdded               bool
	hadError                     bool
	plugins                      []ConnectorPlugin
}

func NewConnectorSocket(sourceURI *url.URL, targetURI *url.URL, connMonitor *ConnectionMonitor,
	connInfo *ConnectionInfo, plugins []ConnectorPlugin, websocketClientFarm *WebsocketClientFarm,
	componentName string, parentCtx context.Context) *ConnectorSocket {
	s := &ConnectorSocket{
		SockId:              uuid.New(),
		httpClient:          GetHttpClient(),
		targetURI:           targetURI,
		registerURI:         sourceURI,
		connMonitor:         connMonitor,
		connInfo:            connInfo,
		createTime:          time.Now().Unix(),
		websocketClientFarm: websocketClientFarm,
		componentName:       componentName,
		parentCtx:           parentCtx,
	}
	if plugins != nil {
		s.plugins = plugins
	} else {
		s.plugins = make([]ConnectorPlugin, 0, 0)
	}
	return s
}

func (s *ConnectorSocket) WhenConsumed(runnable func()) {
	s.whenConsumedAction = runnable
}

func (s *ConnectorSocket) RegisterURI() *url.URL {
	return s.registerURI
}

// OnWebsocketConnect A Websocket Session has connected successfully and is ready to be used.
func (s *ConnectorSocket) OnWebsocketConnect(conn *ws.Conn) {
	s.session = conn
	LOG.Debugf("connected to %s, sockId= %s", conn.RemoteAddr().String(), s.SockId.String())

	var count int64
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelPing = cancel
	s.session.SetReadLimit(maxWebsocketMsgSize)
	go func(ctx, parentCtx context.Context) {
	LOOP:
		for {
			if s.session == nil {
				LOG.Infof("cancel ping task because the websocket is closed, sockID is :%s", s.SockId.String())
				return
			}
			select {
			case <-ctx.Done():
				LOG.Infof("the socket itself dies , cancelling its pingTask, sockID is: %s", s.SockId.String())
				break LOOP
			case <-parentCtx.Done():
				LOG.Infof("the connectorApp is shutting down, cancelling its pingTask, sockID is: %s", s.SockId.String())
				break LOOP
			case <-time.After(5 * time.Second):
			}
			s.session.WriteControl(ws.PingMessage, []byte(
				fmt.Sprintf("send the %dth ping to %s for sockId= %s",
					atomic.AddInt64(&count, 1), s.session.LocalAddr().String(), s.SockId.String())), time.Now().Add(writeWait))
		}

	}(ctx, s.parentCtx)
	s.runForever(conn)
}

func (s *ConnectorSocket) OnWebsocketBinary(payload []byte) {
	if n, err := s.targetRequestContentWriter.Write(payload); err != nil {
		LOG.Warningf("failed to feed request content to target, sockId: %s, request: %s, err: %s", s.SockId, s.requestToTarget.url, err.Error())
	} else {
		LOG.Debugf("content added: %s", payload[:n])
	}
}

func (s *ConnectorSocket) OnWebsocketText(msg string) {
	ptcReq := NewCrankerProtocolRequest(msg)

	LOG.Debugf("connectorSocket %s receive msg from routerSocket, request is %s", s.SockId, ptcReq.ToProtocolMessage())
	if IsDebugReq(ptcReq) {
		LOG.Infof("onWebsocketText -> connectorSocket %s receive msg from routerSocket, request is %s", s.SockId, ptcReq.ToProtocolMessage())
	}

	// fire the req to target only when endMarker is RequestHasNoBodyMarker: str = "_2" or RequestBodyEndedMarker: str = "_1"
	if s.requestToTarget == nil {
		s.onRequestReceived()
		s.newRequestToTarget(ptcReq)
		s.sendRequestToTarget(ptcReq)
	} else if ptcReq.RequestBodyEnded() {
		LOG.Debugf("there will be no more request body coming. sockId: %s", s.SockId)
		_ = s.targetRequestContentWriter.Close()
	}
}

func (s *ConnectorSocket) onRequestReceived() {
	LOG.Debugf("connectorSocket %s connected with connectionInfo %s", s.SockId, s.connInfo)
	s.websocketClientFarm.removeWebsocket(s.RegisterURI().String())
	s.connInfo.OnConnectedSuccessfully()
	s.whenConsumedAction()
	s.newSocketAdded = true
}

func (s *ConnectorSocket) newRequestToTarget(ptcReq *CrankerProtocolRequest) {
	if IsDebugReq(ptcReq) {
		LOG.Infof("newRequestToTarget -> connector receive msg from router socket, request is %s", ptcReq.ToProtocolMessage())
	}
	ptcResp := new(CrankerProtocolResponseBuilder).WithSourceUrl(ptcReq.Dest).WithHttpMethod(ptcReq.HttpMethod)
	reqDest, _ := url.Parse(ptcReq.Dest)
	dest := s.targetURI.ResolveReference(reqDest)
	LOG.Infof("going to send %s to %s and component is %s", ptcReq, dest, s.componentName)
	carriers := NewConnectorPluginStatCarriers()
	_ = s.handlePluginsBeforeRequestSent(ptcReq, carriers)
	s.requestToTarget = NewIntermediateRequest(dest.String()).Method(ptcReq.HttpMethod).Agent("").WithWebsocketSession(s.session) // use the client's agent rather than golang agent

	putHeadersTo(s.requestToTarget, ptcReq)

	onResponseBegin := func(resp *http.Response) {
		// handle response line
		ptcResp.WithRespStatus(resp.StatusCode).WithRespReason(resp.Status)
		// handler response Headers
		LOG.Debugf("golang's httpClient finished its job and here is response. request: %s, method: %s", dest, ptcReq.HttpMethod)
		ptcResp.WithRespHeaders(parseHeaders(resp.Header))
		ptcRespMsg := NewCrankerProtocolResponse(ptcResp.Build())
		LOG.Debugf("going to send response to cranker router. response: %s, request: %s, method: %s", ptcRespMsg.ToProtocolMessage(), dest, ptcReq.HttpMethod)
		if IsDebugResp(ptcRespMsg) {
			LOG.Infof("onResponseHeaders -> connector receive msg from target service, response is %s", ptcRespMsg.ToProtocolMessage())
		}
		_ = s.handlePluginsAfterResponseReceived(ptcRespMsg, carriers)
		if err := s.session.WriteMessage(ws.TextMessage, []byte(ptcRespMsg.ToProtocolMessage())); err != nil {
			LOG.Errorf("failed to send response header back to router through websocket, request: %s, err: %s", dest, err.Error())
		}
	}
	s.requestToTarget.WithResponseBeginHandler(onResponseBegin)
}

func (s *ConnectorSocket) sendRequestToTarget(ptcReq *CrankerProtocolRequest) {
	if !ptcReq.RequestBodyPending() && !ptcReq.RequestHasNoBody() {
		return
	}

	if ptcReq.RequestBodyPending() {
		s.targetRequestContentProvider, s.targetRequestContentWriter = io.Pipe()
		s.requestToTarget.Content(s.targetRequestContentProvider)
		LOG.Debugf("request body pending, sockId=%s", s.SockId)
	}

	LOG.Debug("request Headers are received")
	s.connMonitor.OnConnectionStarted()
	callback := func(result *result) {
		LOG.Debugf("connectorSocket got response from target service and finish sending back to routerSocket,sockId: %s, request: %s", s.SockId, s.requestToTarget.url)
		s.connMonitor.OnConnectionEnded()
		// cancelPing first
		if s.cancelPing != nil {
			LOG.Debugf("target service response finished, socket's cancelPing is not nil, so it is going to be cancelled, sockID is %s", s.SockId.String())
			s.cancelPing()
		}
		if result.isSucceeded {
			s.requestComplete = true
			LOG.Debugf("closing websocket because response is fully processed, sockId: %s, request: %s", s.SockId, s.requestToTarget.url)
			if s.session != nil {
				if err := s.session.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseNormalClosure, "Proxy complete"), time.Now().Add(writeWait)); err != nil {
					LOG.Errorf("failed to close websocket connection normally from ws client side, err: %s", err.Error())
				}
			}
		} else {
			s.requestComplete = false
			errorID := uuid.New().String()
			if _, ok := result.failure.(CancelErr); !ok {
				LOG.Warningf("failed for %s, ErrorID: %s, err: %s", result.response, errorID, result.failure.Error())
			}
			// nil means onWebsocketClose has been invoked, clientServer close the conn actively,cancelError, not nil means connector side error
			if s.session != nil {
				if err := s.session.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(ws.CloseInternalServerErr, "ErrorID: "+errorID), time.Now().Add(writeWait)); err != nil {
					LOG.Errorf("failed to close websocket connection normally from ws client side, err: %s", err.Error())
				}
			}
		}
	}
	go s.requestToTarget.FireRequestFromConnectorToTargetService(callback)
	LOG.Debug("request body is fully sent")
}

func putHeadersTo(reqToTarget *IntermediateRequest, ptcReq *CrankerProtocolRequest) {
	if IsDebugReq(ptcReq) {
		LOG.Infof("putHeadersTo -> connector receive msg from router socket, request is %s", ptcReq.ToProtocolMessage())
	}
	for _, line := range ptcReq.Headers {
		if pos := strings.Index(line, ":"); pos > 0 {
			header := line[0:pos]
			value := line[pos+1:]
			var h string
			if header == "Authorization" {
				h = strings.Split(value, ":")[0]
			} else {
				h = value
			}
			LOG.Debugf("target request header %s = %s", header, h)
			reqToTarget.Headers.Add(header, value)
		}
	}
	reqToTarget.Headers.Add("Via", "1.1 crnk")
}

func parseHeaders(h http.Header) *HeadersBuilder {
	hb := new(HeadersBuilder)
	for k, vs := range h {
		for _, v := range vs {
			hb.AppendHeader(k, v)
		}
	}
	return hb
}

type StatCarrier struct {
}

func (c *StatCarrier) Close() error {
	return nil
}

func (c *StatCarrier) GetStat() interface{} {
	return 0
}

func (s *ConnectorSocket) handlePluginsBeforeRequestSent(req *CrankerProtocolRequest, carriers *ConnectorPluginStatCarriers) (err error) {
	for _, p := range s.plugins {
		if carrier, err := p.HandleBeforeRequestSent(req, &StatCarrier{}); err != nil {
			LOG.Errorf("failed to apply handleBeforeRequestSent for plugin [%s] with protocol request %s, err: %s", p, req, err.Error())
			return err
		} else {
			carriers.Put(p, carrier)
		}
	}
	return
}

func (s *ConnectorSocket) handlePluginsAfterResponseReceived(resp *CrankerProtocolResponse, carriers *ConnectorPluginStatCarriers) (err error) {
	defer carriers.Close()
	for _, p := range s.plugins {
		carrier := carriers.Get(p)
		if carrierAfterResponse, err := p.HandleAfterResponseReceived(resp, carrier); err != nil {
			LOG.Errorf("failed to apply handleAfterResponseReceived for plugin [%s] with protocol response %s, err: %s", p, resp, err.Error())
			carriers.CleanCarrier(p)
			return err
		} else if err = carrierAfterResponse.Close(); err != nil {
			LOG.Errorf("failed to close carrierAfterResponse %s for plugin [%s] with protocol response %s, err: %s", p, resp, err.Error())
			carriers.CleanCarrier(p)
			return err
		}
	}
	return
}

// A Close Event was received. if we set this hook, we need to response a CloseMessage back to websocket client.
func (s *ConnectorSocket) OnWebsocketClose(statusCode int, reason string) error {
	LOG.Debugf("connection with sockId %s, closed, statusCode: %s, reason: %s", s.SockId, statusCode, reason)
	s.clean()
	if s.cancelPing != nil {
		LOG.Debugf("OnWebsocketClose, socket's cancelPing is not nil, so it is going to be cancelled, sockID is %s", s.SockId.String())
		s.cancelPing()
		s.cancelPing = nil
	}
	if !s.newSocketAdded {
		LOG.Debugf("going to reconnect to router, the dying conn's sockId: %s, close code: %d", s.SockId, statusCode)
		s.websocketClientFarm.removeWebsocket(s.RegisterURI().String())
		s.whenConsumedAction()
		s.newSocketAdded = true
	}
	if !s.requestComplete && s.requestToTarget != nil {
		if statusCode != ws.CloseInternalServerErr {
			LOG.Infof("the websocket closed before the target response was processed, this may be because the user closed their browser."+
				"going to cancel request to target %s", s.requestToTarget.url)
			s.requestToTarget.Abort(CancelErr{Msg: "the websocket session to router is close"})
		}
	}
	if s.session != nil {
		LOG.Debugf("OnWebsocketClose. Replying CloseMessage from client side.. ")
		return s.session.WriteControl(ws.CloseMessage, ws.FormatCloseMessage(statusCode, ""), time.Now().Add(writeWait))
	}
	return nil
}

// this func will be invoked under 2 circumstances:
// 1. websocket connection has been built but error happened when reading msg from conn
// 2. failed to dial a connection.
func (s *ConnectorSocket) OnWebsocketError(cause error) {
	if s.hadError {
		LOG.Infof("received error, connectionInfo=%s, but it was already handed, so ignoring it.", s.connInfo.String())
		return
	}
	LOG.Debug("going to remove websocket as websocket error")
	s.hadError = true
	LOG.Warningf("websocket error, %s to %s - Error: %s", s.connInfo, s.targetURI, cause.Error())
	if s.cancelPing != nil {
		LOG.Debugf("OnWebsocketError, socket's cancelPing is not nil, so it is going to be cancelled, sockID is %s", s.SockId.String())
		s.cancelPing()
		s.cancelPing = nil
	}
	if !s.newSocketAdded {
		s.websocketClientFarm.removeWebsocket(s.RegisterURI().String())
		delay := s.connInfo.RetryAfterMillis()
		LOG.Infof("going to reconnect to router after %d ms", delay)
		go func() {
			select {
			case <-s.parentCtx.Done(): // if parent from the connector says, shutdown, new connectorSocket are not allow to be added
				return
			default:
			}
			time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
				s.whenConsumedAction()
				s.newSocketAdded = true
			})
		}()
		s.clean()
	}
}

func (s *ConnectorSocket) clean() {
	sessionToClean := s.session
	if sessionToClean != nil {
		if err := sessionToClean.Close(); err != nil { // close the underlying conn without sending or waiting for a close msg.
			LOG.Warningf("failed to close the underlying net.conn connection, sockId: %s, err: %s", s.SockId, err.Error())
		}
	}
	s.session = nil
}

func (s *ConnectorSocket) runForever(conn *ws.Conn) {
	for {
		// msgType must be either TextMessage or BinaryMessage
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			if closeErr, ok := err.(*ws.CloseError); !ok {
				s.OnWebsocketError(err)
			} else {
				s.OnWebsocketClose(closeErr.Code, closeErr.Text)
			}
			return
		}
		if msgType == ws.TextMessage {
			s.OnWebsocketText(string(msg))
		} else if msgType == ws.BinaryMessage {
			s.OnWebsocketBinary(msg)
		} else {
			LOG.Errorf("unexpected msgType got: %d", msgType)
		}
	}
}
