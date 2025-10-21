package log

import (
	"encoding/json"
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
	logger, _ := New("debug", false)

	fns := map[string]func(msg string, fields ...Field){
		"info":  logger.Info,
		"debug": logger.Debug,
		"warn":  logger.Warn,
		"error": logger.Error,
	}
	for level, fn := range fns {
		logOutput := captureStderr(func() {
			fn("log message", String("text", "dummy text"), Int("int", 123), Bool("bool", true), Error(errors.New("dummy error")))
		})
		assertLogCorrect(t, level, logOutput)
	}
	// panic needs to be wrapped
	logOutput := captureStderr(func() {
		assert.Panics(t, func() {
			logger.Panic("log message", String("text", "dummy text"), Int("int", 123), Bool("bool", true), Error(errors.New("dummy error")))
		}, "log.Panic should panic")
	})
	assertLogCorrect(t, "panic", logOutput)
}

func TestOneliner(t *testing.T) {
	logger, _ := New("info", true)

	logOutput := captureStderr(func() {
		logger.Info("some message", String("text", "dummy text"))
		logger.Info("main message", String("more", "more text"), MainMessage())
		logger.Info("other message", String("other", "other text"))
		logger.Flush()
	})

	result := decodeLogs(t, logOutput)
	assert.Equal(t, result["msg"], "main message")
	assert.Equal(t, result["text"], "dummy text")
	assert.Equal(t, result["more"], "more text")
	assert.Equal(t, result["other"], "other text")
}

func TestOnelinerWithoutMainMessage(t *testing.T) {
	logger, _ := New("info", true)

	logOutput := captureStderr(func() {
		logger.Info("first message", String("text", "dummy text"))
		logger.Info("second message", String("more", "more text"))
		logger.Info("third message", String("other", "other text"))
		logger.Flush()
	})

	result := decodeLogs(t, logOutput)
	assert.Equal(t, result["msg"], "first message") // it takes the first one
	assert.Equal(t, result["text"], "dummy text")
	assert.Equal(t, result["more"], "more text")
	assert.Equal(t, result["other"], "other text")
}

func TestOnelinerMultipleLevels(t *testing.T) {
	logger, _ := New("info", true)

	logOutput := captureStderr(func() {
		logger.Info("info message", String("text", "dummy text"))
		logger.Warn("warn message", String("memory", "low"))
		logger.Info("info message", String("other", "other text"))
		logger.Flush()
	})

	result := decodeLogs(t, logOutput)
	assert.Equal(t, result["msg"], "warn message") // it takes the first of the highest level
	assert.Equal(t, result["text"], "dummy text")
	assert.Equal(t, result["memory"], "low")
	assert.Equal(t, result["other"], nil) // Info logging ignored after Warn
}

func decodeLogs(t *testing.T, logOutput []byte) map[string]any {
	result := make(map[string]any)
	err := json.Unmarshal(logOutput, &result)
	assert.NoError(t, err)
	return result
}

func assertLogCorrect(t *testing.T, level string, logOutput []byte) {
	result := decodeLogs(t, logOutput)
	assert.Equal(t, result["level"], level)
	assert.Equal(t, result["msg"], "log message")
	assert.Equal(t, result["text"], "dummy text")
	assert.Equal(t, result["int"], float64(123))
	assert.Equal(t, result["int"], float64(123))
	assert.Equal(t, result["bool"], true)
	assert.Equal(t, result["error"], "dummy error")
}

func captureStderr(f func()) []byte {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()

	// redirect /dev/stderr to the pipe
	originalStderr := os.Stderr.Fd()
	if err := syscall.Dup2(int(w.Fd()), int(originalStderr)); err != nil {
		panic(err)
	}

	f()

	// restore original stderr
	_ = syscall.Dup2(int(originalStderr), int(w.Fd()))

	buf := make([]byte, 10240)
	n, _ := r.Read(buf)
	return buf[:n]
}
