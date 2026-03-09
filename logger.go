package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
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
	Sync()
}

type stdLogger struct {
	l    *slog.Logger
	sync func()
}

type logMessage struct {
	buf  *bytes.Buffer
	sync chan struct{}
}

type prettyHandler struct {
	opts             slog.HandlerOptions
	ch               chan logMessage
	groups           []string
	precomputedAttrs []byte
	dropped          *atomic.Uint64
}

type LogConfig struct {
	Stdout    bool
	FilePaths []string
	Level     slog.Level
}

var bufPool = sync.Pool{
	New: func() any {
		b := new(bytes.Buffer)
		b.Grow(512)
		return b
	},
}

func (l stdLogger) Debug(args ...any) {
	l.l.Debug(fmt.Sprint(args...))
}

func (l stdLogger) Debugf(format string, args ...any) {
	l.l.Debug(fmt.Sprintf(format, args...))
}

func (l stdLogger) Info(args ...any) {
	l.l.Info(fmt.Sprint(args...))
}

func (l stdLogger) Infof(format string, args ...any) {
	l.l.Info(fmt.Sprintf(format, args...))
}

func (l stdLogger) Warn(args ...any) {
	l.l.Warn(fmt.Sprint(args...))
}

func (l stdLogger) Warnf(format string, args ...any) {
	l.l.Warn(fmt.Sprintf(format, args...))
}

func (l stdLogger) Error(args ...any) {
	l.l.Error(fmt.Sprint(args...))
}

func (l stdLogger) Errorf(format string, args ...any) {
	l.l.Error(fmt.Sprintf(format, args...))
}

func (l stdLogger) Fatal(args ...any) {
	l.l.Error(fmt.Sprint(args...))
	l.Sync()
	os.Exit(1)
}

func (l stdLogger) Fatalf(format string, args ...any) {
	l.l.Error(fmt.Sprintf(format, args...))
	l.Sync()
	os.Exit(1)
}

func (l stdLogger) Panic(args ...any) {
	msg := fmt.Sprint(args...)
	l.l.Error(msg)
	l.Sync()
	panic(msg)
}

func (l stdLogger) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.l.Error(msg)
	l.Sync()
	panic(msg)
}

func (l stdLogger) Sync() {
	if l.sync != nil {
		l.sync()
	}
}

func (h *prettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *prettyHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	var b [64]byte
	tb := r.Time.AppendFormat(b[:0], "2006-01-02 15:04:05")
	buf.Write(tb)
	buf.WriteByte(' ')

	buf.WriteByte('[')
	buf.WriteString(r.Level.String())
	buf.WriteByte(']')

	pad := 7 - len(r.Level.String())
	for i := 0; i < pad; i++ {
		buf.WriteByte(' ')
	}
	buf.WriteByte(' ')

	buf.WriteString(r.Message)

	if len(h.precomputedAttrs) > 0 {
		buf.Write(h.precomputedAttrs)
	}

	r.Attrs(func(a slog.Attr) bool {
		appendAttr(buf, h.groups, a)
		return true
	})

	buf.WriteByte('\n')

	select {
	case h.ch <- logMessage{buf: buf}:
	default:
		h.dropped.Add(1)
		bufPool.Put(buf)
	}

	return nil
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h

	var buf bytes.Buffer
	buf.Write(h.precomputedAttrs)

	for _, a := range attrs {
		appendAttr(&buf, h2.groups, a)
	}

	h2.precomputedAttrs = make([]byte, buf.Len())
	copy(h2.precomputedAttrs, buf.Bytes())

	return &h2
}

func appendAttr(buf *bytes.Buffer, groups []string, a slog.Attr) {
	if a.Key == "" && a.Value.Kind() != slog.KindGroup {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			groups = append(groups, a.Key)
		}
		for _, attr := range a.Value.Group() {
			appendAttr(buf, groups, attr)
		}
		return
	}
	buf.WriteByte(' ')
	for _, g := range groups {
		buf.WriteString(g)
		buf.WriteByte('.')
	}
	buf.WriteString(a.Key)
	buf.WriteByte('=')
	buf.WriteString(a.Value.String())
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	h2.groups = append(h2.groups[:len(h2.groups):len(h2.groups)], name)
	return &h2
}

func (h *prettyHandler) Sync() {
	syncChan := make(chan struct{})
	h.ch <- logMessage{sync: syncChan}
	<-syncChan
}

func startWorker(ch <-chan logMessage, out io.Writer, dropped *atomic.Uint64) {
	bw := bufio.NewWriterSize(out, 64*1024)

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					bw.Flush()
					return
				}
				if msg.buf != nil {
					if d := dropped.Swap(0); d > 0 {
						dropMsg := fmt.Sprintf("%s [WARN] [INTERNAL] Dropped %d messages due to buffer overflow\n", time.Now().Format("2006-01-02 15:04:05"), d)
						bw.WriteString(dropMsg)
					}
					bw.Write(msg.buf.Bytes())
					bufPool.Put(msg.buf)
				}
				if msg.sync != nil {
					bw.Flush()
					close(msg.sync)
				}
			case <-ticker.C:
				if d := dropped.Swap(0); d > 0 {
					dropMsg := fmt.Sprintf("%s [WARN] [INTERNAL] Dropped %d messages due to buffer overflow\n", time.Now().Format("2006-01-02 15:04:05"), d)
					bw.WriteString(dropMsg)
				}
				bw.Flush()
			}
		}
	}()
}

func NewLogger(cfg LogConfig) Logger {
	var writers []io.Writer

	if cfg.Stdout {
		writers = append(writers, os.Stdout)
	}

	for _, p := range cfg.FilePaths {
		if p == "" {
			continue
		}
		if dir := filepath.Dir(p); dir != "." && dir != "" {
			os.MkdirAll(dir, 0755)
		}
		logFile, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			writers = append(writers, logFile)
		} else {
			fmt.Fprintf(os.Stderr, "Aether Logger: Failed to open %s (%v)\n", p, err)
		}
	}

	var writer io.Writer
	if len(writers) == 0 {
		writer = io.Discard
	} else if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	ch := make(chan logMessage, 8192)
	dropped := &atomic.Uint64{}
	startWorker(ch, writer, dropped)

	handler := &prettyHandler{
		ch:      ch,
		dropped: dropped,
		opts: slog.HandlerOptions{
			Level: cfg.Level,
		},
	}

	return stdLogger{
		l:    slog.New(handler),
		sync: handler.Sync,
	}
}

func newStdLogger() Logger {
	return NewLogger(LogConfig{
		Stdout: true,
		Level:  slog.LevelInfo,
	})
}
