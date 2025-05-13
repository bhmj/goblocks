package log

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type (
	FieldType             uint8
	ContextMetaLoggerType string
)

const (
	BoolType FieldType = iota
	IntType
	StringType
	StringsType
	ErrorType
	TimeType
	DurationType
	AnyType
	MessageType

	ContextMetaLogger ContextMetaLoggerType = "ContextMetaLogger"
)

type Field struct {
	Key       string
	Type      FieldType
	Integer   int64
	String    string
	Interface interface{}
}

// MetaLogger implements logging functionality
type MetaLogger interface { //nolint:interfacebloat
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	DPanic(msg string, fields ...Field)
	Panic(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	Infof(template string, args ...interface{})
	Add(fields ...Field)
	With(fields ...Field) MetaLogger
	Sync() error
	SetContextLogger(ctx context.Context) context.Context
}

type logger struct {
	externalLogger *zap.Logger
}

// NewNop returns no-op logger
func NewNop() MetaLogger {
	return &logger{externalLogger: zap.NewNop()}
}

// New returns new logger. 'level' defines required logging level.
func New(level string, oneline bool) (MetaLogger, error) {
	// check level values
	zapLevels := map[string]zapcore.Level{
		"debug":  zap.DebugLevel,
		"info":   zap.InfoLevel,
		"warn":   zap.WarnLevel,
		"error":  zap.ErrorLevel,
		"dpanic": zap.DPanicLevel,
		"panic":  zap.PanicLevel,
		"fatal":  zap.FatalLevel,
	}

	config := zap.NewProductionConfig()
	if level == "debug" {
		config.DisableCaller = false
	} else {
		config.Sampling = nil
		config.DisableCaller = true
	}

	config.Level.SetLevel(zapLevels[level])
	config.Encoding = "json"
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.EncodeTime = timeEncoder
	config.DisableStacktrace = true

	l, err := config.Build()
	if err != nil {
		return nil, fmt.Errorf("construct logger from config: %w", err)
	}
	return &logger{externalLogger: l}, nil
}

// SetContextLogger puts a meta logger into the context.
func (l *logger) SetContextLogger(ctx context.Context) context.Context {
	return context.WithValue(ctx, ContextMetaLogger, l)
}

// GetContextLogger returns a meta logger extracted from context.
func GetContextLogger(ctx context.Context) MetaLogger {
	loggerReference := ctx.Value(ContextMetaLogger)
	return loggerReference.(MetaLogger)
}

func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	// set up time format for logging
	encoded := t.UTC().AppendFormat([]byte{}, "2006-01-02T15:04:05.000Z")
	enc.AppendByteString(encoded)
}

func Bool(key string, val bool) Field {
	var i int64
	if val {
		i = 1
	}
	return Field{Key: key, Type: BoolType, Integer: i}
}

func Int(key string, val int) Field {
	return Field{Key: key, Type: IntType, Integer: int64(val)}
}

func Int64(key string, val int64) Field {
	return Field{Key: key, Type: IntType, Integer: val}
}

func String(key string, val string) Field {
	return Field{Key: key, Type: StringType, String: val}
}

func Strings(key string, val []string) Field {
	return Field{Key: key, Type: StringsType, Interface: val}
}

func Error(err error) Field {
	return Field{Key: "error", Type: ErrorType, Interface: err}
}

func Time(key string, val time.Time) Field {
	return Field{Key: key, Type: TimeType, Interface: val}
}

func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Type: DurationType, Interface: val}
}

func Stack(name string) Field {
	return Field{Key: name, Type: StringType, String: zap.Stack("").String}
}

func Any(name string, val any) Field {
	return Field{Key: name, Type: AnyType, Interface: val}
}

func MainMessage() Field {
	return Field{Key: "", Type: MessageType}
}

func convert(fields []Field) []zap.Field {
	var zapfields []zap.Field
	for _, field := range fields {
		switch field.Type {
		case BoolType:
			zapfields = append(zapfields, zap.Bool(field.Key, field.Integer > 0))
		case IntType:
			zapfields = append(zapfields, zap.Int64(field.Key, field.Integer))
		case StringType:
			zapfields = append(zapfields, zap.String(field.Key, field.String))
		case StringsType:
			zapfields = append(zapfields, zap.Strings(field.Key, field.Interface.([]string)))
		case TimeType:
			zapfields = append(zapfields, zap.Time(field.Key, field.Interface.(time.Time)))
		case DurationType:
			zapfields = append(zapfields, zap.Duration(field.Key, field.Interface.(time.Duration)))
		case ErrorType:
			zapfields = append(zapfields, zap.String(field.Key, field.Interface.(error).Error()))
		case AnyType:
			zapfields = append(zapfields, zap.Any(field.Key, field.Interface))
		}
	}
	return zapfields
}

func (l *logger) Debug(msg string, fields ...Field) {
	l.log(int(zap.DebugLevel), msg, fields...)
}

func (l *logger) Info(msg string, fields ...Field) {
	l.log(int(zap.InfoLevel), msg, fields...)
}

func (l *logger) Warn(msg string, fields ...Field) {
	l.log(int(zap.WarnLevel), msg, fields...)
}

func (l *logger) Error(msg string, fields ...Field) {
	l.log(int(zap.ErrorLevel), msg, fields...)
}

func (l *logger) DPanic(msg string, fields ...Field) {
	l.log(int(zap.DPanicLevel), msg, fields...)
}

func (l *logger) Panic(msg string, fields ...Field) {
	l.log(int(zap.PanicLevel), msg, fields...)
}

func (l *logger) Fatal(msg string, fields ...Field) {
	l.log(int(zap.FatalLevel), msg, fields...)
}

func (l *logger) Infof(template string, args ...interface{}) {
	l.externalLogger.Sugar().Infof(template, args...)
}

// Add adds fields to current logger (cloning external logger under the hood)
func (l *logger) Add(fields ...Field) {
	zapfields := convert(fields)
	l.externalLogger = l.externalLogger.With(zapfields...)
}

// With returns a clone of the current logger with added fields
func (l *logger) With(fields ...Field) MetaLogger {
	zapfields := convert(fields)
	return &logger{
		externalLogger: l.externalLogger.With(zapfields...),
	}
}

func (l *logger) Sync() error {
	return l.externalLogger.Sync() //nolint:wrapcheck
}

func (l *logger) log(level int, msg string, mfields ...Field) {
	fields := convert(mfields)
	switch level {
	case int(zap.DebugLevel):
		l.externalLogger.Debug(msg, fields...)
	case int(zap.InfoLevel):
		l.externalLogger.Info(msg, fields...)
	case int(zap.WarnLevel):
		l.externalLogger.Warn(msg, fields...)
	case int(zap.ErrorLevel):
		l.externalLogger.Error(msg, fields...)
	case int(zap.PanicLevel):
		l.externalLogger.Panic(msg, fields...)
	case int(zap.FatalLevel):
		l.externalLogger.Fatal(msg, fields...)
	}
}
