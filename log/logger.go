package log

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	slogzap "github.com/samber/slog-zap/v2"
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
	IntpType
	Float64Type
	Float64pType
	StringType
	StringsType
	StringpType
	ErrorType
	TimeType
	DurationType
	AnyType
	MessageType
)

const (
	ContextMetaLogger ContextMetaLoggerType = "ContextMetaLogger"

	zeroLevel = int(zap.DebugLevel) - 1
)

type Field struct {
	Key       string
	Type      FieldType
	Integer   int64
	Float64   float64
	String    string
	Interface any
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
	Flush()
	Verbose() MetaLogger
	SetContextLogger(ctx context.Context) context.Context
	SlogHandler() slog.Handler
}

type logger struct {
	externalLogger *zap.Logger
	oneline        bool
	level          int
	message        string
	fields         []Field
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
	return &logger{externalLogger: l, oneline: oneline, level: zeroLevel}, nil
}

// SetContextLogger puts a meta logger into the context.
func (l *logger) SetContextLogger(ctx context.Context) context.Context {
	return context.WithValue(ctx, ContextMetaLogger, l)
}

// GetContextLogger returns a meta logger extracted from context.
func GetContextLogger(ctx context.Context) MetaLogger {
	loggerReference := ctx.Value(ContextMetaLogger)
	ref, ok := loggerReference.(MetaLogger)
	if !ok {
		panic("meta logger not found in context")
	}
	return ref
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

func Float64(key string, val float64) Field {
	return Field{Key: key, Type: IntType, Float64: val}
}

func Float64p(key string, val *float64) Field {
	return Field{Key: key, Type: Float64pType, Interface: val}
}

func String(key string, val string) Field {
	return Field{Key: key, Type: StringType, String: val}
}

func Strings(key string, val []string) Field {
	return Field{Key: key, Type: StringsType, Interface: val}
}

func Stringp(key string, val *string) Field {
	return Field{Key: key, Type: StringpType, Interface: val}
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
	zapfields := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		switch field.Type {
		case BoolType:
			zapfields = append(zapfields, zap.Bool(field.Key, field.Integer > 0))
		case IntType:
			zapfields = append(zapfields, zap.Int64(field.Key, field.Integer))
		case Float64Type:
			zapfields = append(zapfields, zap.Float64(field.Key, field.Float64))
		case Float64pType:
			zapfields = append(zapfields, zap.Float64p(field.Key, field.Interface.(*float64)))
		case StringType:
			zapfields = append(zapfields, zap.String(field.Key, field.String))
		case StringsType:
			zapfields = append(zapfields, zap.Strings(field.Key, field.Interface.([]string))) //nolint:forcetypeassert
		case StringpType:
			zapfields = append(zapfields, zap.Stringp(field.Key, field.Interface.(*string))) //nolint:forcetypeassert
		case TimeType:
			zapfields = append(zapfields, zap.Time(field.Key, field.Interface.(time.Time))) //nolint:forcetypeassert
		case DurationType:
			zapfields = append(zapfields, zap.Duration(field.Key, field.Interface.(time.Duration))) //nolint:forcetypeassert
		case ErrorType:
			zapfields = append(zapfields, zap.String(field.Key, field.Interface.(error).Error())) //nolint:forcetypeassert
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
		oneline:        l.oneline,
		level:          l.level,
	}
}

func (l *logger) Sync() error {
	return l.externalLogger.Sync() //nolint:wrapcheck
}

// Flush outputs buffered log line
func (l *logger) Flush() {
	if !l.oneline {
		return
	}

	if len(l.fields) > 0 || l.message != "" {
		l.directLog(l.level, l.message, convert(l.fields)...)
	}
	l.message = ""
	l.level = zeroLevel // set minimum level to start from, for selecting main message
	l.fields = l.fields[:0]
}

func (l *logger) Verbose() MetaLogger {
	return &logger{
		externalLogger: l.externalLogger,
		oneline:        false,
		level:          zeroLevel,
	}
}

func (l *logger) log(level int, msg string, fields ...Field) {
	if !l.oneline {
		l.directLog(level, msg, convert(fields)...)
		return
	}

	if level < l.level {
		return
	}
	for _, fld := range fields {
		if fld.Type == MessageType {
			l.message = msg // "MainMessage" flag marks the log message ("msg" field)
		} else {
			l.fields = append(l.fields, fld)
		}
	}
	if level > l.level || l.message == "" {
		l.message = msg // take the first or the highest level message as main
	}
	l.level = level
}

func (l *logger) directLog(level int, msg string, fields ...zap.Field) {
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

var logLevels = map[zapcore.Level]slog.Level{ //nolint:gochecknoglobals
	zap.DebugLevel: slog.LevelDebug,
	zap.InfoLevel:  slog.LevelInfo,
	zap.WarnLevel:  slog.LevelWarn,
	zap.ErrorLevel: slog.LevelError,
}

func (l *logger) SlogHandler() slog.Handler {
	return slogzap.Option{Level: logLevels[l.externalLogger.Level()], Logger: l.externalLogger}.NewZapHandler()
}
