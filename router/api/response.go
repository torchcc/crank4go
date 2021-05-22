package api

import (
	"encoding/json"
	"net/http"

	MediaType "github.com/torchcc/crank4go/router/api/media_type"
)

func RespTextPlainOk(w http.ResponseWriter, text string) {
	RespTextPlainWithStatus(w, text, http.StatusOK)
}

func RespTextPlainWithStatus(w http.ResponseWriter, text string, status int) {
	w.Header().Add("Content-Type", MediaType.TextPlain)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(text))
}

func RespJsonOk(w http.ResponseWriter, jsonObj map[string]interface{}) {
	w.Header().Add("Content-Type", MediaType.ApplicationJson)
	w.WriteHeader(http.StatusOK)
	bytes, _ := json.Marshal(jsonObj)
	_, _ = w.Write(bytes)
}
