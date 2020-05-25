package log

import (
	"io"
	"os"
	"time"
)

var (
	LOG_PATH string
	// DEBUG ...
	DEBUG LoggerInterface
	// INFO ...
	INFO LoggerInterface
	// WARNING ...
	WARNING LoggerInterface
	// ERROR ...
	ERROR LoggerInterface
	// FATAL ...
	FATAL LoggerInterface
)

func newLogFile(file string, path string) *os.File {
	if path == "" {
		return nil
	}
	filename := path + time.Now().Format("2006-01-02") + "-" + file + ".log"
	// open an output file, this will append to the today's file if server restarted.
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	return f
}

func InitLog(logPath string) {
	var (
		logger Logger
	)
	loggerFile := newLogFile("system", logPath)
	interpreterFile := newLogFile("interpreters", logPath)
	if loggerFile == nil && interpreterFile == nil {
		logger = New(io.Writer(os.Stdout), io.Writer(os.Stdout), new(ColouredFormatter))
	} else {
		logger = New(io.MultiWriter(loggerFile, os.Stdout), io.MultiWriter(loggerFile, os.Stdout),
			new(ColouredFormatter))
	}

	// DEBUG ...
	DEBUG = logger[LDEBUG]
	// INFO ...
	INFO = logger[LINFO]
	// WARNING ...
	WARNING = logger[LWARNING]
	// ERROR ...
	ERROR = logger[LERROR]
	// FATAL ...
	FATAL = logger[LFATAL]
}
