package consensus

import (
	"github.com/hashicorp/go-hclog"
	"io"
	"log"
	"os"
)

const INFO = hclog.Info

type defaultConsensusLogger struct {
	level hclog.Level
}

func (d *defaultConsensusLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Error:
		d.Error(msg, args...)
	case hclog.Warn:
		d.Warn(msg, args...)
	case hclog.Info:
		d.Info(msg, args...)
	case hclog.Debug:
		d.Debug(msg, args...)
	case hclog.Trace:
		d.Trace(msg, args...)
	case hclog.NoLevel:
		return
	}
}

func (d *defaultConsensusLogger) Trace(msg string, args ...interface{}) {
	return // no need
}

func (d *defaultConsensusLogger) Debug(msg string, args ...interface{}) {
	if d.level == hclog.Debug {
		log.Printf("%s %v\n", msg, args)
	}
}

func (d *defaultConsensusLogger) Info(msg string, args ...interface{}) {
	log.Printf("%s %v\n", msg, args)
}

func (d *defaultConsensusLogger) Warn(msg string, args ...interface{}) {
	log.Printf("%s %v\n", msg, args)
}

func (d *defaultConsensusLogger) Error(msg string, args ...interface{}) {
	log.Printf("%s %v\n", msg, args)
}

func (d *defaultConsensusLogger) IsTrace() bool {
	return false // no need
}

func (d *defaultConsensusLogger) IsDebug() bool {
	return d.level == hclog.Debug
}

func (d *defaultConsensusLogger) IsInfo() bool {
	return d.level != hclog.Debug
}

func (d *defaultConsensusLogger) IsWarn() bool {
	return d.level != hclog.Debug
}

func (d *defaultConsensusLogger) IsError() bool {
	return d.level != hclog.Debug
}

func (d *defaultConsensusLogger) ImpliedArgs() []interface{} {
	return []interface{}{}
}

func (d *defaultConsensusLogger) With(args ...interface{}) hclog.Logger {
	return d
}

func (d *defaultConsensusLogger) Name() string {
	return "consensus"
}

func (d *defaultConsensusLogger) Named(name string) hclog.Logger {
	return d
}

func (d *defaultConsensusLogger) ResetNamed(name string) hclog.Logger {
	return d
}

func (d *defaultConsensusLogger) SetLevel(level hclog.Level) {
	d.level = level
}

func (d *defaultConsensusLogger) GetLevel() hclog.Level {
	return d.level
}

func (d *defaultConsensusLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(os.Stderr, "consensus", log.LstdFlags|log.Lmicroseconds)
}

func (d *defaultConsensusLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return os.Stderr
}
