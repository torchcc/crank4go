package connector

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
)

var (
	once, once2     sync.Once
	httpClient      *http.Client
	websocketDialer *ws.Dialer
)

const (
	// for websocket conn to read msg
	maxWebsocketMsgSize = 16384
	// for httpClient to write msg, use jetty's reverseProxy default value
	WriteBufferSize = 4 * 8192
)

func GetHttpClient() *http.Client {
	once.Do(func() {
		tr := &http.Transport{
			TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
			IdleConnTimeout:    2 * time.Hour,
			MaxConnsPerHost:    32768,
			WriteBufferSize:    WriteBufferSize,
			DisableCompression: false,
		}
		httpClient = &http.Client{
			Transport: tr,
			Timeout:   0,
		}
	})
	return httpClient
}

func GetWebsocketDialer() *ws.Dialer {
	once2.Do(func() {
		websocketDialer = &ws.Dialer{
			Proxy:             http.ProxyFromEnvironment,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			HandshakeTimeout:  45 * time.Second,
			ReadBufferSize:    0,            // default is 4096
			WriteBufferSize:   0,            // default is 4096
			WriteBufferPool:   &sync.Pool{}, // java version do not use pool
			EnableCompression: false,
		}
	})
	return websocketDialer
}
