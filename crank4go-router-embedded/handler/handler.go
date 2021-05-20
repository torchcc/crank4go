package handler

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type XHandler interface {
	// if this method return true, it prevents pending reqHandlers from being called
	Handle(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool
}

// The XHandlerFunc type is an adapter to allow the use of
// ordinary functions as XHTTPHandlers. If f is a function
// with the appropriate signature, XHandlerFunc(f) is a
// Handler that calls f.
type XHandlerFunc func(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool

func (f XHandlerFunc) Handle(w http.ResponseWriter, r *http.Request, params httprouter.Params) bool {
	return f(w, r, params)
}

// an implement of net/http Handler, to implement pipeline/filter pattern easily
type XHTTPHandler struct {
	// they are executed before request is handled by target handler
	reqHandlers []XHandler
	// they are executed after request is handled by target handler
	respHandlers []XHandler

	targetHandler XHandler
}

// To register on origin http ServeMux
func (h *XHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.ServeXHTTP(w, r, nil)
}

func NewXHttpHandler(targetHandler XHandler) *XHTTPHandler {
	return &XHTTPHandler{targetHandler: targetHandler}
}

// To register handler on httprouter
func (h *XHTTPHandler) ServeXHTTP(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	stop := false
	for _, handler := range h.reqHandlers {
		stop = handler.Handle(w, r, params)
		if stop {
			return
		}
	}

	h.targetHandler.Handle(w, r, params)

	for _, handler := range h.respHandlers {
		handler.Handle(w, r, params)
	}
}

func (h *XHTTPHandler) AddReqHandlers(handlers ...XHandler) *XHTTPHandler {
	if h.reqHandlers == nil {
		h.respHandlers = make([]XHandler, 0, 8)
	}
	for _, handler := range handlers {
		h.reqHandlers = append(h.reqHandlers, handler)
	}
	return h
}

func (h *XHTTPHandler) AddRespHandlers(handlers ...XHandler) *XHTTPHandler {
	if h.respHandlers == nil {
		h.respHandlers = make([]XHandler, 0, 8)
	}
	for _, handler := range handlers {
		h.respHandlers = append(h.respHandlers, handler)
	}
	return h
}
