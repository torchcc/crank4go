package scaffolding

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/torchcc/crank4go/router/api"
	"github.com/torchcc/crank4go/util"
)

type RestfulServer struct {
	server    *http.Server
	port      int
	resources []api.Resource
}

func NewRestfulServer(port int, resources ...api.Resource) *RestfulServer {
	return &RestfulServer{port: port, resources: resources}
}

func (s *RestfulServer) Port() int {
	return s.port
}

func (s *RestfulServer) Start() {
	s.createHttpServer()
	go func() {
		util.LOG.Infof("going to start RESTFul health server at http://0.0.0.0:%d/health", s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("failed to start httpServer, err: " + err.Error())
		}
	}()
}

func (s *RestfulServer) createHttpServer() {
	httpRouter := httprouter.New()
	for _, resource := range s.resources {
		resource.RegisterResourceToHttpRouter(httpRouter, "")
	}

	s.server = &http.Server{
		Addr:    ":" + strconv.Itoa(s.port),
		Handler: httpRouter,
	}
}

func (s *RestfulServer) ShutDown() {
	timeout, cancelFunc := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelFunc()
	_ = s.server.Shutdown(timeout)
}
