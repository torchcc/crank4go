package test

import (
	"errors"
	"fmt"
	"path"
	"runtime"
)

var TestDir = ""

func getTestDir() {
	TestDir = path.Dir(getCurrentFile())

	if TestDir == "" {
		panic(errors.New("can not get current file info"))
	} else {
		fmt.Println("test dir is " + TestDir)

	}
}

func getCurrentFile() string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic(errors.New("can not get current file info"))
	}
	return file
}

func init() {
	getTestDir()
}
