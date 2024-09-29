package utils

import (
	"fmt"
	"os"
)

type Logger interface {
	Tracef(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})
}

type NoopLogger struct{}

var _ Logger = &NoopLogger{}

func (l *NoopLogger) Tracef(format string, args ...interface{}) {}
func (l *NoopLogger) Debugf(format string, args ...interface{}) {}
func (l *NoopLogger) Infof(format string, args ...interface{})  {}
func (l *NoopLogger) Warnf(format string, args ...interface{})  {}
func (l *NoopLogger) Errorf(format string, args ...interface{}) {}
func (l *NoopLogger) Fatalf(format string, args ...interface{}) {
	os.Stderr.WriteString(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func (l *NoopLogger) Panicf(format string, args ...interface{}) {
	panic(fmt.Errorf(format, args...))
}
