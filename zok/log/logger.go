package log

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const DefaultLogName = "logs/messages.log"

var (
	LogLevel zapcore.Level

	filename string
	opts     Options
	logger   *zap.Logger
	sugar    *zap.SugaredLogger
	w        *LogrotateWriter
)

func init() {
	if v, exists := os.LookupEnv("LOG_LEVEL"); exists {
		_ = LogLevel.Set(v)
	}
}

type LogEntry struct {
	Level   zapcore.Level `json:"level"`
	Time    time.Time     `json:"ts"`
	Message string        `json:"msg"`
	Stack   string        `json:"stack,omitempty"`
}

type multiWriteCloser struct {
	rotated *LogrotateWriter
	stdout  *os.File
	out     io.Writer
}

func (w *multiWriteCloser) Close() error {
	return w.rotated.Close()
}

func (w *multiWriteCloser) Write(p []byte) (n int, err error) {
	return w.out.Write(p)
}

type Mode string

const (
	Stdout Mode = "stdout"
	File   Mode = "file"
	Both   Mode = "both"
)

type Options struct {
	Mode     Mode
	Filename string
}

func Open(options Options) {
	opts = options
	filename = opts.Filename

	var err error

	if options.Mode == Stdout {
		c := zap.NewProductionConfig()
		c.Level = zap.NewAtomicLevelAt(LogLevel)
		c.OutputPaths = []string{"stdout"}
		c.Encoding = "console"
		c.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
		c.EncoderConfig.CallerKey = zapcore.OmitKey
		c.EncoderConfig.StacktraceKey = zapcore.OmitKey
		logger, err = c.Build()
		if err != nil {
			panic(err)
		}
		sugar = logger.Sugar()
		return
	}

	var iw io.Writer

	if options.Mode == Both {
		w = NewLogrotateWriter(LogrotateOption{
			Filename:   filepath.Join(filepath.Clean(filename)),
			MaxSize:    4 << 20,
			MaxBackups: 6,
			Compress:   true,
		})
		iw = &multiWriteCloser{
			rotated: w,
			stdout:  os.Stdout,
			out:     io.MultiWriter(w, os.Stdout),
		}
	} else {
		w = NewLogrotateWriter(LogrotateOption{
			Filename:   filepath.Join(filepath.Clean(filename)),
			MaxSize:    4 << 20,
			MaxBackups: 6,
			Compress:   true,
		})
		iw = w
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	encoderConfig.CallerKey = zapcore.OmitKey
	encoderConfig.StacktraceKey = zapcore.OmitKey
	enc, ws := zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(iw)

	v := zap.New(zapcore.NewCore(enc, ws, LogLevel))
	logger = v
	sugar = v.Sugar()
}

func Close() (err error) {
	if opts.Mode != Stdout {
		err = logger.Sync()
	}
	if w != nil {
		if err2 := w.Close(); err2 != nil && err == nil {
			err = err2
		}
	}
	return
}

func Filename() string {
	return filename
}

func Rotate() error {
	if w == nil {
		return nil
	}
	return w.Rotate()
}

func DebugFields(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func InfoFields(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func WarnFields(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

func ErrorFields(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func PanicFields(msg string, fields ...zap.Field) {
	logger.Panic(msg, fields...)
}

func FatalFields(msg string, fields ...zap.Field) {
	logger.Fatal(msg, fields...)
}

func Debugw(msg string, args ...any) {
	sugar.Debugw(msg, args...)
}

func Infow(msg string, args ...any) {
	sugar.Infow(msg, args...)
}

func Warnw(msg string, args ...any) {
	sugar.Warnw(msg, args...)
}

func Errorw(msg string, args ...any) {
	sugar.Errorw(msg, args...)
}

func Panicw(msg string, args ...any) {
	sugar.Panicw(msg, args...)
}

func Fatalw(msg string, args ...any) {
	sugar.Fatalw(msg, args...)
}

func Debug(args ...any) {
	sugar.Debugln(args...)
}

func Info(args ...any) {
	sugar.Infoln(args...)
}

func Warn(args ...any) {
	sugar.Warnln(args...)
}

func Debugf(format string, args ...any) {
	sugar.Debugf(format, args...)
}
func Infof(format string, args ...any) {
	sugar.Infof(format, args...)
}

func Warnf(format string, args ...any) {
	sugar.Warnf(format, args...)
}

func t(s string, err error) (msg string, fields []zap.Field) {
	if s != "" {
		msg = s + ": "
	}

	if err != nil {
		msg += err.Error()
	}

	if err, ok := AsTracedError(err); ok {
		fields = append(fields, zap.String("stack", err.Stack()))
	}

	return
}

func ErrorP(prefix string, e error) {
	msg, fields := t(prefix, e)
	ErrorFields(msg, fields...)
}

func PanicP(prefix string, e error) {
	msg, fields := t(prefix, e)
	PanicFields(msg, fields...)
}

func FatalP(prefix string, e error) {
	msg, fields := t(prefix, e)
	FatalFields(msg, fields...)
}

func Error(e error) {
	msg, fields := t("", e)
	ErrorFields(msg, fields...)
}

func Panic(e error) {
	msg, fields := t("", e)
	PanicFields(msg, fields...)
}

func Fatal(e error) {
	msg, fields := t("", e)
	FatalFields(msg, fields...)
}
