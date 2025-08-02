// Package logger provides structured logging configuration and setup for the application.
package logger

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

func New(level string) zerolog.Logger {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339Nano

	logLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		logLevel = zerolog.InfoLevel
		// Use a basic logger to print this warning, as the main one isn't configured yet.
		fmt.Fprintf(os.Stderr, "Invalid log level '%s', defaulting to 'info'\n", level)
	}

	gitRevision := "unknown"
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		for _, v := range buildInfo.Settings {
			if v.Key == "vcs.revision" {
				gitRevision = v.Value
				break
			}
		}
	}

	l := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		Level(logLevel).
		With().
		Timestamp().
		Caller().
		Int("pid", os.Getpid()).
		Str("go_version", buildInfo.GoVersion).
		Str("git_revision", gitRevision).
		Logger()

	zerolog.DefaultContextLogger = &l
	return l
}
