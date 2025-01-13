package logger

import (
	"bytes"
	"fmt"
	"github.com/filinvadim/warpnet/json"
	"github.com/labstack/gommon/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"os"
)

type (
	Core  = zapcore.Core
	Field = zap.Field
)

type UnifiedLogger struct {
	logger *zap.Logger
	level  zapcore.Level
	isJSON bool
	output io.Writer
	prefix string
	header string
}

func NewUnifiedLogger(logLevel string, isJSON bool) *UnifiedLogger {
	level := zapcore.InfoLevel
	if err := (&level).Set(logLevel); err != nil {
		level = zapcore.InfoLevel // Default to Info level if invalid
	}

	config := zap.NewProductionConfig()
	if !isJSON {
		config.Encoding = "console"
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	config.Level = zap.NewAtomicLevelAt(level)
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, _ := config.Build()

	return &UnifiedLogger{
		logger: logger,
		level:  level,
		isJSON: isJSON,
		output: os.Stdout,
	}
}

func (l *UnifiedLogger) Enabled(level zapcore.Level) bool {
	return level >= l.level
}

func (l *UnifiedLogger) With(fields []zapcore.Field) zapcore.Core {
	return l.logger.With(fields...).Core()
}

func (l *UnifiedLogger) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	ce = l.logger.Check(l.level, entry.Message)
	return ce
}

func (l *UnifiedLogger) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	return l.logger.Core().Write(entry, fields)
}

func (l *UnifiedLogger) Sync() error {
	return l.logger.Sync()
}

func (l *UnifiedLogger) Output() io.Writer {
	return l.output
}

func (l *UnifiedLogger) SetOutput(w io.Writer) {
	l.output = w
}

func (l *UnifiedLogger) Prefix() string {
	return l.prefix
}

func (l *UnifiedLogger) SetPrefix(p string) {
	l.prefix = p
}

func (l *UnifiedLogger) Level() log.Lvl {
	return log.Lvl(l.level)
}

func (l *UnifiedLogger) SetLevel(v log.Lvl) {
	l.level = zapcore.Level(v)
	l.logger = l.logger.WithOptions(zap.IncreaseLevel(l.level))
}

func (l *UnifiedLogger) SetHeader(h string) {
	l.header = h
}

func (l *UnifiedLogger) Print(i ...interface{}) {
	l.logger.Info(concatArgs(i...))
}

func (l *UnifiedLogger) Printf(format string, args ...interface{}) {
	l.logger.Info(concatFormat(format, args...))
}

func (l *UnifiedLogger) Printj(j log.JSON) {
	bt, _ := json.JSON.Marshal(j)
	l.logger.Info(string(bytes.TrimSpace(bt)))
}

func (l *UnifiedLogger) Debug(i ...interface{}) {
	l.logger.Debug(concatArgs(i...))
}

func (l *UnifiedLogger) Debugj(j log.JSON) {
	bt, _ := json.JSON.Marshal(j)
	l.logger.Debug(string(bytes.TrimSpace(bt)))
}

func (l *UnifiedLogger) Info(i ...interface{}) {
	l.logger.Info(concatArgs(i...))
}

func (l *UnifiedLogger) Infoj(j log.JSON) {
	bt, _ := json.JSON.Marshal(j)
	l.logger.Info(string(bytes.TrimSpace(bt)))
}

func (l *UnifiedLogger) Warn(i ...interface{}) {
	l.logger.Warn(concatArgs(i...))
}

func (l *UnifiedLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warn(concatFormat(format, args...))
}

func (l *UnifiedLogger) Warnj(j log.JSON) {
	bt, _ := json.JSON.Marshal(j)
	l.logger.Warn(string(bytes.TrimSpace(bt)))
}

func (l *UnifiedLogger) Error(i ...interface{}) {
	l.logger.Error(concatArgs(i...))
}

func (l *UnifiedLogger) Errorj(j log.JSON) {
	bt, _ := json.JSON.Marshal(j)
	l.logger.Error(string(bytes.TrimSpace(bt)))
}

func (l *UnifiedLogger) Fatal(i ...interface{}) {
	l.logger.Fatal(concatArgs(i...))
}

func (l *UnifiedLogger) Fatalj(j log.JSON) {
	bt, _ := json.JSON.Marshal(j)
	l.logger.Fatal(string(bytes.TrimSpace(bt)))
}

func (l *UnifiedLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatal(concatFormat(format, args...))
}

func (l *UnifiedLogger) Panic(i ...interface{}) {
	l.logger.Panic(concatArgs(i...))
}

func (l *UnifiedLogger) Panicj(j log.JSON) {
	bt, _ := json.JSON.Marshal(j)
	l.logger.Panic(string(bt))
}

func (l *UnifiedLogger) Panicf(format string, args ...interface{}) {
	l.logger.Panic(concatFormat(format, args...))
}

func (l *UnifiedLogger) Errorf(s string, i ...interface{}) {
	l.logger.Error(concatFormat(s, i...))
}

func (l *UnifiedLogger) Warningf(s string, i ...interface{}) {
	l.logger.Warn(concatFormat(s, i...))
}

func (l *UnifiedLogger) Infof(s string, i ...interface{}) {
	l.logger.Info(concatFormat(s, i...))
}

func (l *UnifiedLogger) Debugf(s string, i ...interface{}) {
	l.logger.Debug(concatFormat(s, i...))
}

func concatArgs(args ...interface{}) string {
	if len(args) == 1 {
		if str, ok := args[0].(string); ok {
			return str
		}
	}
	return fmt.Sprint(args...)
}

func concatFormat(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
