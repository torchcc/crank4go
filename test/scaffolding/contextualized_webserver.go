package scaffolding

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/torchcc/crank4go-core/test"
	"github.com/torchcc/crank4go-core/util"
)

type ContextualizedWebserver struct {
	Uri           *url.URL
	server        *http.Server
	staticContext string
}

func NewContextualizedWebserver(port int, ctx string) *ContextualizedWebserver {
	svr := &ContextualizedWebserver{}
	svr.Uri, _ = url.Parse("http://localhost:" + strconv.Itoa(port))
	svr.staticContext = ctx + "/static/"
	util.LOG.Infof("test dir is %s", test.TestDir)
	mux := http.NewServeMux()
	mux.Handle(svr.staticContext, http.StripPrefix(ctx, http.FileServer(http.Dir(test.TestDir))))
	mux.Handle(ctx+"/say_hi", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("say hi"))
	}))
	mux.Handle(ctx+"/upload", http.HandlerFunc(UploadImage))

	svr.server = &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: mux,
	}
	return svr
}

func (s *ContextualizedWebserver) Start() {
	go func() {
		util.LOG.Infof("target server is started at %s", s.Uri.String()+s.staticContext+"hello.html")
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("failed to start httpServer, uri: " + s.Uri.String() + " err: " + err.Error())
		}
	}()
}
func (s *ContextualizedWebserver) ShutDown() {
	timeout, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	_ = s.server.Shutdown(timeout)
}

func UploadImage(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(1024 * 1024) // max size of file that allowed to upload
	if err != nil {
		util.LOG.Errorf("failed to upload: err: %s", err.Error())
		return
	}

	img := r.MultipartForm.File["image"][0]
	name := img.Filename

	fmt.Println("uploaded: ", name)

	file, err := img.Open()
	if err == nil {
		data, err := ioutil.ReadAll(file)
		if err == nil {
			filename := name
			// 创建这个文件
			newFile, err := os.Create(test.TestDir + "/static/img/" + filename)
			if err != nil {
				fmt.Errorf("failed to create, err: %s", err.Error())
			}
			defer newFile.Close()
			// 将上传文件的二进制字节信息写入新建的文件
			size, err := newFile.Write(data)
			if err == nil {
				fmt.Fprintf(w, "uploaded, img size: %d 字节\n", size/1000)
			}
		}
	}
}

func TestContextualizedWebserver(t *testing.T) {
	NewContextualizedWebserver(10086, "/a").Start()
	time.Sleep(60 * time.Second)
}
