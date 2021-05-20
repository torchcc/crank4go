package util

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// a helper function to execute some hooks before exiting the program.
// usage:
// 1. wrapper all things that you want to do in a func for example `exitFunc ()`
// 2. call `ExitProgram(ExitFunc)` inside a `init` func of your program
func ExitProgram(exitFunc func()) {
	c := make(chan os.Signal)
	// listen to specified signals: ctrl+c kill
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				fmt.Println("executing shutdownHooks before exit...")
				exitFunc()
				fmt.Println("all hooks are executed. shutting down the program...")
				os.Exit(0)
			case syscall.SIGUSR1:
				fmt.Println("usr1", s)
			case syscall.SIGUSR2:
				fmt.Println("usr2", s)
			default:
				fmt.Println("other", s)
			}
		}
	}()
}
