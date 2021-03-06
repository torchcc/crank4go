package handler

import (
	"net/http"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/torchcc/crank4go/router/router_socket"
)

var upgrader = &ws.Upgrader{
	HandshakeTimeout:  45 * time.Second,
	ReadBufferSize:    4096, // default value
	WriteBufferSize:   4096, // default value
	WriteBufferPool:   &sync.Pool{},
	EnableCompression: false,
}

type WebsocketHandler struct {
	factory  func(w http.ResponseWriter, r *http.Request) *router_socket.RouterSocket
	upgrader *ws.Upgrader
}

func NewWebsocketHandler() *WebsocketHandler {
	return NewWebsocketHandler2(nil)
}

func NewWebsocketHandler2(factory func(w http.ResponseWriter, r *http.Request) *router_socket.RouterSocket) *WebsocketHandler {
	return &WebsocketHandler{factory: factory, upgrader: upgrader}
}

func (w *WebsocketHandler) WithWebsocketFactory(factory func(w http.ResponseWriter, r *http.Request) *router_socket.RouterSocket) *WebsocketHandler {
	w.factory = factory
	return w
}

func (w *WebsocketHandler) Handle(respWriter http.ResponseWriter, req *http.Request, params httprouter.Params) bool {
	if socket := w.factory(respWriter, req); socket == nil {
		return true
	}
	return false
}
