package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
	goLog "log"
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
	logFile *os.File
}

func NewLogger(level string, json bool) Logger {
	lg := logrus.New()

	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		lg.WithError(err).Info(level)
		lvl = logrus.InfoLevel
	}

	f, err := os.OpenFile(getLogPath(), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}

	lg.SetLevel(lvl)

	lg.SetOutput(os.Stdout)

	if json {
		lg.SetFormatter(&logrus.JSONFormatter{PrettyPrint: false})
	}

	return &log{lg.WithTime(time.Now()), f}
}

func (l *log) Level() string {
	return l.Level()
}
func (l *log) Close() {
	l.logFile.Close()
}

func getLogPath() string {
	var (
		path, file string
		now        = time.Now().Format(time.DateTime)
	)

	switch runtime.GOOS {
	case "windows":
		// %LOCALAPPDATA% Windows
		appData := os.Getenv("LOCALAPPDATA") // C:\Users\{username}\AppData\Local
		if appData == "" {
			goLog.Fatal("failed to get path to LOCALAPPDATA")
		}
		path = filepath.Join(appData, "badgerdb", "log")
		file = path + `\` + now + ".log"

	case "darwin", "linux", "android":
		homeDir := os.TempDir()
		path = filepath.Join(homeDir, ".badgerdb", "log")
		file = path + `/` + now + ".log"
	default:
		goLog.Fatal("unsupported OS")
	}

	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		goLog.Fatal(err)
	}

	return file
}
