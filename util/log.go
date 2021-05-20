package util

import (
	"os"

	"github.com/op/go-logging"
)

var LOG = logging.MustGetLogger("crank4go-core")

// Example format string. Everything except the message has a custom color
// which is dependent on the log level. Many fields have a custom output
// formatting too, eg. the time returns the hour down to the milli second.
var format = logging.MustStringFormatter(
	`%{color}%{shortfile} %{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

// Password is just an example type implementing the Redactor interface. Any
// time this is logged, the Redacted() function will be called.
type Password string

func (p Password) Redacted() interface{} {
	return logging.Redact(string(p))
}

func init() {
	// For demo purposes, create two backend for os.Stderr.
	backend1 := logging.NewLogBackend(os.Stderr, "", 0)
	backend2 := logging.NewLogBackend(os.Stderr, "", 0)

	// For messages written to backend2 we want to add some additional
	// information to the output, including the used log level and the name of
	// the function.
	backend2Formatter := logging.NewBackendFormatter(backend2, format)

	// Only errors and more severe messages should be sent to backend1
	backend1Leveled := logging.AddModuleLevel(backend1)
	backend1Leveled.SetLevel(logging.ERROR, "")

	// Set the backends to be used.
	logging.SetBackend(backend1Leveled, backend2Formatter)
}

// const (
// 	logDir      = "log"
// 	logSoftLink = "latest_log"
// 	module      = "crank4go-core"
// )
//
// var (
// 	defaultFormatter = `%{time:2006/01/02 - 15:04:05.000} %{longfile} %{color:bold}▶ [%{level:.6s}] %{message}%{color:reset}`
// )
//
// func init() {
// 	c := global.GVA_CONFIG.Log
// 	if c.Prefix == "" {
// 		_ = fmt.Errorf("logger prefix not found")
// 	}
// 	logger := oplogging.MustGetLogger(module)
// 	var backends []oplogging.Backend
// 	registerStdout(c, &backends)
// 	if fileWriter := registerFile(c, &backends); fileWriter != nil {
// 		gin.DefaultWriter = io.MultiWriter(fileWriter, os.Stdout)
// 	}
// 	oplogging.SetBackend(backends...)
// 	global.GVA_LOG = logger
// }
//
// func registerStdout(c config.Log, backends *[]oplogging.Backend) {
// 	if c.Stdout != "" {
// 		level, err := oplogging.LogLevel(c.Stdout)
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 		*backends = append(*backends, createBackend(os.Stdout, c, level))
// 	}
// }
//
// func registerFile(c config.Log, backends *[]oplogging.Backend) io.Writer {
// 	if c.File != "" {
// 		if ok, _ := utils.PathExists(logDir); !ok {
// 			// directory not exist
// 			fmt.Println("create log directory")
// 			_ = os.Mkdir(logDir, os.ModePerm)
// 		}
// 		fileWriter, err := rotatelogs.New(
// 			logDir+string(os.PathSeparator)+"%Y-%m-%d-%H-%M.log",
// 			// generate soft link, point to latest log file
// 			rotatelogs.WithLinkName(logSoftLink),
// 			// maximum time to save log files
// 			rotatelogs.WithMaxAge(7*24*time.Hour),
// 			// time period of log file switching
// 			rotatelogs.WithRotationTime(24*time.Hour),
// 		)
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 		level, err := oplogging.LogLevel(c.File)
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 		*backends = append(*backends, createBackend(fileWriter, c, level))
//
// 		return fileWriter
// 	}
// 	return nil
// }
//
// func createBackend(w io.Writer, c config.Log, level oplogging.Level) oplogging.Backend {
// 	backend := oplogging.NewLogBackend(w, c.Prefix, 0)
// 	stdoutWriter := false
// 	if w == os.Stdout {
// 		stdoutWriter = true
// 	}
// 	format := getLogFormatter(c, stdoutWriter)
// 	backendLeveled := oplogging.AddModuleLevel(oplogging.NewBackendFormatter(backend, format))
// 	backendLeveled.SetLevel(level, module)
// 	return backendLeveled
// }
//
// func getLogFormatter(c config.Log, stdoutWriter bool) oplogging.Formatter {
// 	pattern := defaultFormatter
// 	if !stdoutWriter {
// 		// Color is only required for console output
// 		// Other writers don't need %{color} tag
// 		pattern = strings.Replace(pattern, "%{color:bold}", "", -1)
// 		pattern = strings.Replace(pattern, "%{color:reset}", "", -1)
// 	}
// 	if !c.LogFile {
// 		// Remove %{logfile} tag
// 		pattern = strings.Replace(pattern, "%{longfile}", "", -1)
// 	}
// 	return oplogging.MustStringFormatter(pattern)
// }
