package scaffolding

import (
	"testing"
)

// it blocks. open it to manually run
// func TestManualGetTestTLSConfig(t *testing.T) {
// 	config := GetTestTLSConfig()
// 	if config == nil {
// 		t.Errorf("failed to get cert, err:")
// 		return
// 	}
//
// 	mux := http.NewServeMux()
// 	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(200)
// 		w.Write([]byte("hello"))
// 	}))
// 	server := &http.Server{
// 		Addr:      ":8443",
// 		Handler:   mux,
// 		TLSConfig: config,
// 	}
// 	server.ListenAndServeTLS("", "")
// }

func TestFindFileAsString(t *testing.T) {
	contents := HelloHtmlContents()
	t.Log(contents)
}
