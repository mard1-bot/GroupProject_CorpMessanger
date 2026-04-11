package logger

import (
	"fmt"
	"log/slog"
	"os"
)

type Logger struct{ *slog.Logger }

func New(env, level string) (*Logger, error) {
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level %q", level)
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	return &Logger{Logger: slog.New(handler).With("service", "api", "env", env)}, nil
}
func (l *Logger) Handler() *slog.Logger { return l.Logger }
