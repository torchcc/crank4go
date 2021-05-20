package scaffolding

import (
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/torchcc/crank4go-core/test"
	"github.com/torchcc/crank4go-core/util"
)

func GetTestTLSConfig() *tls.Config {
	// return nil
	cert, err := tls.LoadX509KeyPair(path.Join(test.TestDir, "static/cert/server.pem"), path.Join(test.TestDir, "static/cert/server.key"))
	if err != nil {
		util.LOG.Errorf("failed to load ssl certificate: err: %s", err)
		return nil
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func HelloHtmlContents() string {

	fullPath := test.TestDir + "/static/hello.html"
	return FindFileAsString(fullPath)
}

func FindFileAsString(path string) string {
	if file, e := os.Open(path); e != nil {
		fmt.Printf("failed to open path: %s, error: %s", path, e.Error())
		return ""
	} else {
		if bytes, e := io.ReadAll(file); e != nil {
			fmt.Printf("failed to read file: %s, error: %s", path, e.Error())
			return ""
		} else {
			return string(bytes)
		}
	}
}
