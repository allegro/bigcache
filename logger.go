package bigcache

import (
	"log"
	"os"
)

// Logger is invoked when `Config.Verbose=true`
type Logger interface {
	Printf(format string, v ...interface{})
}

var _ Logger = &log.Logger{}

// DefaultLogger returns a `Logger` implementation
// backed by stdlib's log
func DefaultLogger() *log.Logger {
	return log.New(os.Stdout, "", log.LstdFlags)
}

func newLogger(custom Logger) Logger {
	if custom != nil {
		return custom
	}

	return DefaultLogger()
}
