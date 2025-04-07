package consensus

import (
	"errors"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

type bridgeLogger interface {
	WithField(key string, value interface{}) *logrus.Entry
	WithFields(fields logrus.Fields) *logrus.Entry
	WithError(err error) *logrus.Entry
	Logln(level logrus.Level, args ...interface{})
	Traceln(args ...interface{})
	Debugln(args ...interface{})
	Infoln(args ...interface{})
	Println(args ...interface{})
	Warnln(args ...interface{})
	Warningln(args ...interface{})
	Errorln(args ...interface{})
	Fatalln(args ...interface{})
	Panicln(args ...interface{})
	SetLevel(level logrus.Level)
	GetLevel() logrus.Level
}

type consensusLogger struct {
	l    bridgeLogger
	lvl  logrus.Level
	name string
}

func newConsensusLogger(logLevel, name string) *consensusLogger {
	lvl, _ := logrus.ParseLevel(strings.TrimSpace(strings.ToLower(logLevel)))
	if lvl.String() == "unknown" {
		lvl = logrus.InfoLevel
	}
	l := logrus.New()
	l.SetLevel(lvl)
	return &consensusLogger{
		l:    l,
		lvl:  lvl,
		name: name,
	}
}

func (c *consensusLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	lvl, _ := logrus.ParseLevel(strings.TrimSpace(strings.ToLower(level.String())))
	c.l.Logln(lvl, msg, args)
}

func (c *consensusLogger) Trace(msg string, args ...interface{}) {
	c.l.Traceln(msg, args)
}

func (c *consensusLogger) Debug(msg string, args ...interface{}) {
	c.l.Debugln(msg, args)
}

func (c *consensusLogger) Info(msg string, args ...interface{}) {
	c.l.Infoln(msg, args)
}

func (c *consensusLogger) Warn(msg string, args ...interface{}) {
	c.l.Warnln(msg, args)
}

func (c *consensusLogger) Error(msg string, args ...interface{}) {
	for _, arg := range args {
		err, ok := arg.(error)
		if ok && errors.Is(err, raft.ErrNothingNewToSnapshot) {
			c.Debug(err.Error())
			return
		}
	}
	c.l.Errorln(msg, args)
}

func (c *consensusLogger) IsTrace() bool {
	return c.lvl == logrus.TraceLevel
}

func (c *consensusLogger) IsDebug() bool {
	return c.lvl == logrus.DebugLevel
}

func (c *consensusLogger) IsInfo() bool {
	return c.lvl == logrus.InfoLevel
}

func (c *consensusLogger) IsWarn() bool {
	return c.lvl == logrus.WarnLevel
}

func (c *consensusLogger) IsError() bool {
	return c.lvl == logrus.ErrorLevel
}

func (c *consensusLogger) ImpliedArgs() []interface{} {
	return []interface{}{}
}

func (c *consensusLogger) With(args ...interface{}) hclog.Logger {
	for i, arg := range args {
		c.l.WithField(strconv.Itoa(i), arg)
	}
	return c
}

func (c *consensusLogger) Name() string {
	return c.name
}

func (c *consensusLogger) Named(name string) hclog.Logger {
	c.name = name
	return c
}

func (c *consensusLogger) ResetNamed(name string) hclog.Logger {
	c.name = name
	return c
}

func (c *consensusLogger) SetLevel(level hclog.Level) {
	lvl, _ := logrus.ParseLevel(strings.TrimSpace(strings.ToLower(level.String())))
	if lvl.String() == "unknown" {
		lvl = logrus.InfoLevel
	}
	c.l.SetLevel(lvl)
	c.lvl = lvl
}

func (c *consensusLogger) GetLevel() hclog.Level {
	lvl := c.l.GetLevel()
	return hclog.LevelFromString(lvl.String())
}

func (c *consensusLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(os.Stderr, c.name, log.LstdFlags)
}

func (c *consensusLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return os.Stderr
}
