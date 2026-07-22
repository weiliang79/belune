package worker

import (
	"fmt"
	"log/slog"
)

// asynqLogger adapts asynq's logging interface onto slog.
//
// Without it asynq writes its own format straight to stderr:
//
//	asynq: pid=234659 2026/07/22 02:58:13.003782 INFO: Scheduler starting
//
// which carries its own timestamp and level and so sits outside every level
// filter, colour and module label the rest of the process uses — and, because
// the platform log viewer reads severity off the line, arrives there as plain
// Info regardless of what asynq actually meant.
//
// asynq's levels map straight across. Fatal has no slog equivalent; it is
// logged at Error rather than exiting, because asynq calls it for conditions
// the server itself then handles.
type asynqLogger struct{}

func (asynqLogger) Debug(args ...any) { slog.Debug(fmt.Sprint(args...)) }
func (asynqLogger) Info(args ...any)  { slog.Info(fmt.Sprint(args...)) }
func (asynqLogger) Warn(args ...any)  { slog.Warn(fmt.Sprint(args...)) }
func (asynqLogger) Error(args ...any) { slog.Error(fmt.Sprint(args...)) }
func (asynqLogger) Fatal(args ...any) { slog.Error(fmt.Sprint(args...)) }
