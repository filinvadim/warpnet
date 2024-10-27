package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

type Logger interface {
	Infoln(...interface{})
	Infof(string, ...interface{})
	Errorln(...interface{})
	Errorf(string, ...interface{})
	Debugln(...interface{})
	Fatalln(...interface{})
	Fatalf(string, ...interface{})
	Level() string
	Warningln(...interface{})
	Warningf(string, ...interface{})
}

type log struct {
	*logrus.Entry
}

func (l *log) Level() string {
	return l.Level()
}

func NewLogger(level string, json bool) Logger {
	lg := logrus.New()

	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		lg.WithError(err).Info(level)
		lvl = logrus.InfoLevel
	}
	lg.SetLevel(lvl)

	lg.SetOutput(os.Stdout)

	if json {
		lg.SetFormatter(&logrus.JSONFormatter{PrettyPrint: false})
	}

	return &log{lg.WithField("service", "solidus-reporter")}
}
