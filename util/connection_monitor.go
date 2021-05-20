package util

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
)

type DataPublishHandler interface {
	PublishData(key string, value int)
}

type DataPublishHandlerFunc func(key string, value int)

func (f DataPublishHandlerFunc) PublishData(key string, value int) {
	f(key, value)
}

type ConnectionMonitor struct {
	requestNum          int32
	availableConns      int32
	openFiles           int64
	dataPublishHandlers []DataPublishHandler
}

func NewConnectionMonitor(handlers []DataPublishHandler) *ConnectionMonitor {
	return &ConnectionMonitor{
		dataPublishHandlers: handlers,
	}
}

func (m *ConnectionMonitor) OnConnectionStarted() {
	m.reportConnCount(atomic.AddInt32(&m.requestNum, 1))
	m.reportOpenFilesCount()
	LOG.Infof("activeConnections=%d, openFiles=%d", m.requestNum, m.openFiles)
}

func (m *ConnectionMonitor) OnConnectionStarted2(path, routerSocketID string) {
	m.reportConnCount(atomic.AddInt32(&m.requestNum, 1))
	m.reportOpenFilesCount()
	if path == "" {
		path = "default"
	}
	LOG.Infof("activeConnections=%d, openFiles=%d, routerName=%s, routerSocketID=%s", m.requestNum, m.openFiles, path, routerSocketID)
}

// called by connector socket
func (m *ConnectionMonitor) OnConnectionEnded() {
	m.reportConnCount(atomic.AddInt32(&m.requestNum, -1))
	m.reportOpenFilesCount()
	LOG.Infof("activeConnections=%d, openFiles=%d", m.requestNum, m.openFiles)
}

func (m *ConnectionMonitor) OnConnectionEnded2(path, routerSocketID string) {
	m.reportConnCount(atomic.AddInt32(&m.requestNum, -1))
	m.reportOpenFilesCount()
	LOG.Infof("activeConnections=%d, openFiles=%d", m.requestNum, m.openFiles)
	if path == "" {
		path = "default"
	}
	LOG.Infof("activeConnections=%d, openFiles=%d, routerName=%s, routerSocketID=%s", m.requestNum, m.openFiles, path, routerSocketID)
}

// called by router socket
func (m *ConnectionMonitor) OnConnectionEnded3(routerSocketID, path, reqComponentName string, respStatus int, reqDuration, reqBytes, respBytes int64) {
	activeRequest := atomic.AddInt32(&m.requestNum, -1)
	if path == "" {
		path = "default"
	}
	for _, handler := range m.dataPublishHandlers {
		m.reportConnCount2(handler, activeRequest)
		m.reportOpenFilesCount2(handler)
		handler.PublishData("request.requestBytes,path="+path, int(reqBytes))
		handler.PublishData("request.duration,path="+path, int(reqDuration))
		handler.PublishData("request.responseStatus,path="+path, respStatus)
		handler.PublishData("request.responseBytes,path="+path, int(respBytes))
	}
	LOG.Infof("activeConnection=%d, "+
		"openFiles=%d, "+
		"routerName=%s, "+
		"routerSocketID=%s, "+
		"request_%s_requestBytes=%d, "+
		"request_%s_duration=%d, "+
		"request_%s_responseStatus=%d, "+
		"request_%s_responseBytes=%d, "+
		"request_%s_requestComponentName=%s",
		m.requestNum, m.openFiles, path, routerSocketID,
		path, reqBytes,
		path, reqDuration,
		path, respStatus,
		path, respBytes,
		path, reqComponentName)
}

func (m *ConnectionMonitor) OnConnectionAvailable() {
	m.reportActiveConnCount(atomic.AddInt32(&m.availableConns, 1))
}

// called by connector
func (m *ConnectionMonitor) OnConnectionConsumed() {
	m.reportActiveConnCount(atomic.AddInt32(&m.availableConns, -1))
}

func (m *ConnectionMonitor) ReportWebsocketPoolSize(size int) {
	for _, handler := range m.dataPublishHandlers {
		handler.PublishData("websocket.pool.size", size)
	}
}

func (m *ConnectionMonitor) ConnectionCount() int {
	return int(atomic.LoadInt32(&m.requestNum))
}

func (m *ConnectionMonitor) AvailableConns() int {
	return int(m.availableConns)
}

func (m *ConnectionMonitor) OpenFiles() int {
	return int(m.openFiles)
}

func (m *ConnectionMonitor) reportConnCount(newInflightReqCount int32) {
	for _, handler := range m.dataPublishHandlers {
		m.reportConnCount2(handler, newInflightReqCount)
		m.reportOpenFilesCount2(handler)
	}
}

func (m *ConnectionMonitor) reportConnCount2(handler DataPublishHandler, newInflightReqCount int32) {
	handler.PublishData("connections", int(newInflightReqCount))

}

func (m *ConnectionMonitor) reportOpenFilesCount() {
	for _, handler := range m.dataPublishHandlers {
		m.reportOpenFilesCount2(handler)
	}
}

func (m *ConnectionMonitor) reportOpenFilesCount2(handler DataPublishHandler) {
	return
	// if os is Unix.
	if runtime.GOOS == "linux" || runtime.GOOS == "openbsd" || runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" {
		go func() {
			out, err := exec.Command("/bin/sh", "-c", fmt.Sprintf("lsof -p %v", os.Getpid())).Output()
			if err != nil {
				fmt.Println(err.Error())
			}
			lines := strings.Split(string(out), "\n")
			handler.PublishData("openFiles", len(lines)-1)
		}()
	}

}

func (m *ConnectionMonitor) reportActiveConnCount(newActiveConnCount int32) {
	for _, handler := range m.dataPublishHandlers {
		handler.PublishData("availableConnections", int(newActiveConnCount))
	}

}
