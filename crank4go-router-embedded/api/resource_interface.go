package api

import "github.com/julienschmidt/httprouter"

type Resource interface {
	RegisterResourceToHttpRouter(server *httprouter.Router, rootPath string)
}
