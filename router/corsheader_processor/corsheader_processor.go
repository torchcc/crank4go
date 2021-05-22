package corsheader_processor

import "net/http"

type CorsHeaderProcessor struct {
	checkOrigin func(string) bool
}

func NewCorsHeaderProcessor(checkOrigin func(string) bool) *CorsHeaderProcessor {
	return &CorsHeaderProcessor{checkOrigin: checkOrigin}
}

func (p *CorsHeaderProcessor) Process(req *http.Request, respWriter http.ResponseWriter) {
	origin := req.Header.Get("Origin")
	if origin != "" && p.checkOrigin(origin) { // is CORS and is a good onw
		respWriter.Header().Set("Access-Control-Allow-Origin", origin)
		if varyValue := respWriter.Header().Get("Vary"); varyValue != "" {
			respWriter.Header().Set("Vary", varyValue+", Origin")
		} else {
			respWriter.Header().Add("Vary", "Origin")
		}
	}
}
