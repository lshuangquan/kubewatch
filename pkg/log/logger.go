package log

import (
	"io"
	"log"
	"os"
)

// Level type
type level int

const (
	// LDEBUG level
	LDEBUG level = iota
	// LINFO level
	LINFO
	// LWARNING level
	LWARNING
	// LERROR level
	LERROR
	// LFATAL level
	LFATAL

	//flag = log.Ldate | log.Ltime
	flag = 0
)

// Log level prefix map
var prefix = map[level]string{
	LDEBUG:   "DEBUG: ",
	LINFO:    "INFO: ",
	LWARNING: "WARN: ",
	LERROR:   "ERROR: ",
	LFATAL:   "FATAL: ",
}

// Logger ...
type Logger map[level]LoggerInterface

// New returns instance of Logger
func New(out, errOut io.Writer, f Formatter) Logger {
	// Fall back to stdout if out not set
	if out == nil {
		out = os.Stdout
	}

	// Fall back to stderr if errOut not set
	if errOut == nil {
		errOut = os.Stderr
	}

	// Fall back to DefaultFormatter if f not set
	if f == nil {
		f = new(DefaultFormatter)
	}

	l := make(map[level]LoggerInterface, 5)
	l[LDEBUG] = &Wrapper{lvl: LDEBUG, formatter: f, logger: log.New(out, f.GetPrefix(LDEBUG)+prefix[LDEBUG], flag)}
	l[LINFO] = &Wrapper{lvl: LINFO, formatter: f, logger: log.New(out, f.GetPrefix(LINFO)+prefix[LINFO], flag)}
	l[LWARNING] = &Wrapper{lvl: LWARNING, formatter: f, logger: log.New(out, f.GetPrefix(LWARNING)+prefix[LWARNING], flag)}
	l[LERROR] = &Wrapper{lvl: LERROR, formatter: f, logger: log.New(errOut, f.GetPrefix(LERROR)+prefix[LERROR], flag)}
	l[LFATAL] = &Wrapper{lvl: LFATAL, formatter: f, logger: log.New(errOut, f.GetPrefix(LFATAL)+prefix[LFATAL], flag)}
	return Logger(l)
}

// Wrapper ...
type Wrapper struct {
	lvl       level
	formatter Formatter
	logger    LoggerInterface
}

// Print ...
func (w *Wrapper) Print(v ...interface{}) {
	v = w.formatter.Format(w.lvl, v...)
	v = append(v, w.formatter.GetSuffix(w.lvl))
	w.logger.Print(v...)
}

// Printf ...
func (w *Wrapper) Printf(format string, v ...interface{}) {
	suffix := w.formatter.GetSuffix(w.lvl)
	v = w.formatter.Format(w.lvl, v...)
	w.logger.Printf("%s"+format+suffix, v...)
}

// Println ...
func (w *Wrapper) Println(v ...interface{}) {
	v = w.formatter.Format(w.lvl, v...)
	v = append(v, w.formatter.GetSuffix(w.lvl))
	w.logger.Println(v...)
}

// Fatal ...
func (w *Wrapper) Fatal(v ...interface{}) {
	v = w.formatter.Format(w.lvl, v...)
	v = append(v, w.formatter.GetSuffix(w.lvl))
	w.logger.Fatal(v...)
}

// Fatalf ...
func (w *Wrapper) Fatalf(format string, v ...interface{}) {
	suffix := w.formatter.GetSuffix(w.lvl)
	v = w.formatter.Format(w.lvl, v...)
	w.logger.Fatalf("%s"+format+suffix, v...)
}

// Fatalln ...
func (w *Wrapper) Fatalln(v ...interface{}) {
	v = w.formatter.Format(w.lvl, v...)
	v = append(v, w.formatter.GetSuffix(w.lvl))
	w.logger.Fatalln(v...)
}

// Panic ...
func (w *Wrapper) Panic(v ...interface{}) {
	v = w.formatter.Format(w.lvl, v...)
	v = append(v, w.formatter.GetSuffix(w.lvl))
	w.logger.Fatal(v...)
}

// Panicf ...
func (w *Wrapper) Panicf(format string, v ...interface{}) {
	suffix := w.formatter.GetSuffix(w.lvl)
	v = w.formatter.Format(w.lvl, v...)
	w.logger.Panicf("%s"+format+suffix, v...)
}

// Panicln ...
func (w *Wrapper) Panicln(v ...interface{}) {
	v = w.formatter.Format(w.lvl, v...)
	v = append(v, w.formatter.GetSuffix(w.lvl))
	w.logger.Panicln(v...)
}
