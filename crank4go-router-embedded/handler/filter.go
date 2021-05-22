package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/torchcc/crank4go-core/util"
)

// PreLoggingFilter 1. applied on crankerRouter's websocketServer's http request, not websocket request.
// 2. applied on crankerRouter's httpServer
// log before the request is handled by target handler
func PreLoggingFilter(_ http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	util.LOG.Infof("started - API {%s} being called by client {%s} through {%s}", r.URL.String(), r.RemoteAddr, r.Method)
	return false
}

// PostLoggingFilter a log filter apply on crankerRouter's registerHandler's http request, not websocket request
// log after the request is handled by target handler
func PostLoggingFilter(_ http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	util.LOG.Infof("finished - API {%s} being called by client {%s} through {%s}", r.URL.String(), r.RemoteAddr, r.Method)
	return false
}

// ReqValidatorFilter applied on router's httpServer, it aborts requests with invalid HTTPMethod
func ReqValidatorFilter(w http.ResponseWriter, r *http.Request, _ httprouter.Params) bool {
	var (
		contentLen int
		err        error
	)
	if s := r.Header.Get("Content-Length"); s == "" {
		contentLen = -1
	} else if contentLen, err = strconv.Atoi(s); err != nil ||
		("chunked" == strings.ToLower(r.Header.Get("Transfer-Encoding")) && contentLen > 0) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("Invalid request: chunked request with Content-Length"))
		return true
	}

	if "trace" == strings.ToLower(r.Method) {
		w.WriteHeader(405)
		_, _ = w.Write([]byte("Method Not Allowed"))
		return true
	}

	if "options" == strings.ToLower(r.Method) && util.IsNotLocalEnv() {
		w.WriteHeader(405)
		_, _ = w.Write([]byte("Method Not Allowed"))
		return true
	}
	return false
}
