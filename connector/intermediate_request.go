package connector

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	ws "github.com/gorilla/websocket"
	. "github.com/torchcc/crank4go/util"
)

type result struct {
	isSucceeded bool
	failure     error
	response    *http.Response
}

type IntermediateRequest struct {
	method           string
	Headers          http.Header
	url              string
	client           *http.Client
	contentProvider  *io.PipeReader
	onResponseBegin  func(resp *http.Response)
	websocketSession *ws.Conn
	result           *result
}

func NewIntermediateRequest(url string) *IntermediateRequest {
	return &IntermediateRequest{
		url:     url,
		client:  GetHttpClient(),
		Headers: http.Header{},
		result:  &result{},
	}
}

// Agent set user agent. we need to set user agent to "" so that golang's http client will not be used and the origin user-agent will be use
func (r *IntermediateRequest) Agent(agent string) *IntermediateRequest {
	r.Headers.Add("User-Agent", agent)
	return r
}

// Method set http method
func (r *IntermediateRequest) Method(method string) *IntermediateRequest {
	r.method = method
	return r
}

func (r *IntermediateRequest) WithResponseBeginHandler(runnable func(resp *http.Response)) *IntermediateRequest {
	r.onResponseBegin = runnable
	return r
}

func (r *IntermediateRequest) WithWebsocketSession(session *ws.Conn) *IntermediateRequest {
	r.websocketSession = session
	return r
}

// Abort abort sending request to target service
func (r *IntermediateRequest) Abort(err error) {
	r.result.failure = err
}

// FireRequestFromConnectorToTargetService send the composed httpRequest from connector to target service
func (r *IntermediateRequest) FireRequestFromConnectorToTargetService(callback func(result *result)) {
	defer func() {
		if e := recover(); e != nil {
			LOG.Errorf("failed to fire http request, %s, err: %s", r.String(), e.(error).Error())
		}
		callback(r.result)
	}()

	var (
		request  *http.Request
		response *http.Response
		err      error
	)
	// please don't change the if else clause
	if r.contentProvider == nil {
		request, err = http.NewRequest(r.method, r.url, nil)
	} else {
		request, err = http.NewRequest(r.method, r.url, r.contentProvider)
	}
	if err != nil {
		r.result.failure = err
		LOG.Errorf("fail to compose a request from intermediate request, request: %s", r.String())
		panic(errors.New("fail to compose a request from intermediate request"))
	}
	request.Header = r.Headers
	response, err = r.client.Do(request)
	if err != nil {
		e := HttpClientPolicyErr{Msg: "failed to send request, err detail: " + err.Error()}
		r.result.failure = e
		LOG.Errorf("failed to send request from connector to target-service, err: %s", err.Error())
		panic(e)
	}
	// check is the request is aborted
	if r.result.failure != nil {
		panic(r.result.failure)
	}
	// set resp Headers
	r.onResponseBegin(response)
	// read resp body
	defer response.Body.Close()
	buf := make([]byte, WriteBufferSize)
	n := 0
	for {
		n, err = response.Body.Read(buf)
		if n > 0 {
			if err = r.websocketSession.WriteMessage(ws.BinaryMessage, buf[:n]); err != nil {
				r.result.failure = err
				LOG.Errorf("got response from target-service, but failed to write binary back to router side websocket server, server address: %s, err: %s", r.websocketSession.RemoteAddr(), err.Error())
				panic(err)
			}
		}
		if err == io.EOF {
			r.result.isSucceeded = true
			r.result.response = response
			break
		} else if err != nil {
			err = errors.New("failed to read response body from service response, err detail: " + err.Error())
			LOG.Error(err.Error())
			r.result.failure = err
			panic(err)
		}
	}
}

func (r *IntermediateRequest) Content(reader *io.PipeReader) {
	r.contentProvider = reader
}

func (r *IntermediateRequest) String() string {
	return fmt.Sprintf("IntermediateRequest{url: %s, method: %s}", r.url, r.method)
}
