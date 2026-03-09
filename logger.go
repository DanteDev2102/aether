package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type Logger interface {
	Debug(args ...any)
	Debugf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Panic(args ...any)
	Panicf(format string, args ...any)
}

type stdLogger struct {
	l *slog.Logger
}

func (l stdLogger) Debug(args ...any)                 { l.l.Debug(fmt.Sprint(args...)) }
func (l stdLogger) Debugf(format string, args ...any) { l.l.Debug(fmt.Sprintf(format, args...)) }

func (l stdLogger) Info(args ...any)                 { l.l.Info(fmt.Sprint(args...)) }
func (l stdLogger) Infof(format string, args ...any) { l.l.Info(fmt.Sprintf(format, args...)) }

func (l stdLogger) Warn(args ...any)                 { l.l.Warn(fmt.Sprint(args...)) }
func (l stdLogger) Warnf(format string, args ...any) { l.l.Warn(fmt.Sprintf(format, args...)) }

func (l stdLogger) Error(args ...any)                 { l.l.Error(fmt.Sprint(args...)) }
func (l stdLogger) Errorf(format string, args ...any) { l.l.Error(fmt.Sprintf(format, args...)) }

func (l stdLogger) Fatal(args ...any) {
	l.l.Error(fmt.Sprint(args...))
	os.Exit(1)
}
func (l stdLogger) Fatalf(format string, args ...any) {
	l.l.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func (l stdLogger) Panic(args ...any) {
	msg := fmt.Sprint(args...)
	l.l.Error(msg)
	panic(msg)
}
func (l stdLogger) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.l.Error(msg)
	panic(msg)
}

type prettyHandler struct {
	opts slog.HandlerOptions
	w    io.Writer
}

func (h *prettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *prettyHandler) Handle(ctx context.Context, r slog.Record) error {
	levelStr := "[" + r.Level.String() + "]"
	
	timeStr := r.Time.Format("2006-01-02 15:04:05")

	msg := fmt.Sprintf("%s %-7s %s", timeStr, levelStr, r.Message)

	fmt.Fprintln(h.w, msg)
	return nil
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *prettyHandler) WithGroup(name string) slog.Handler       { return h }

func newStdLogger() Logger {
	logFilePath := filepath.Join(".", "aether.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	
	var writer io.Writer
	if err == nil {
		writer = io.MultiWriter(os.Stdout, logFile)
	} else {
		writer = os.Stdout
	}

	handler := &prettyHandler{
		w: writer,
		opts: slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
	}
	
	return stdLogger{l: slog.New(handler)}
}
