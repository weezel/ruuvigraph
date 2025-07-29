package logging

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/fatih/color"
)

type ColorHandlerOptions struct {
	SlogOpts slog.HandlerOptions
}

type ColorHandler struct {
	slog.Handler
	l *log.Logger
}

func NewColorHandler(out io.Writer, opts ColorHandlerOptions) *ColorHandler {
	return &ColorHandler{
		Handler: slog.NewTextHandler(out, &opts.SlogOpts),
		l:       log.New(out, "", 0),
	}
}

func (h *ColorHandler) Handle(_ context.Context, r slog.Record) error {
	level := r.Level.String() + ":"

	switch r.Level {
	case slog.LevelDebug:
		level = color.MagentaString(level)
	case slog.LevelInfo:
		level = color.BlueString(level)
	case slog.LevelWarn:
		level = color.YellowString(level)
	case slog.LevelError:
		level = color.RedString(level)
	}

	timeStr := r.Time.Format("[15:04:05.000]")
	msg := color.CyanString(r.Message)

	var attrs []string
	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, fmt.Sprintf("%s=%v", attr.Key, attr.Value.Any()))
		return true
	})

	attrStr := ""
	if len(attrs) > 0 {
		attrStr = " [" + strings.Join(attrs, " ") + "]"
	}

	h.l.Println(timeStr, level, msg+attrStr)
	return nil
}

func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColorHandler{
		Handler: h.Handler.WithAttrs(attrs),
		l:       h.l,
	}
}

func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return &ColorHandler{
		Handler: h.Handler.WithGroup(name),
		l:       h.l,
	}
}

func NewColorLogHandler() *slog.Logger {
	h := NewColorHandler(
		os.Stdout,
		ColorHandlerOptions{
			SlogOpts: slog.HandlerOptions{
				Level:     slog.LevelInfo,
				AddSource: true,
			},
		})
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger
}
