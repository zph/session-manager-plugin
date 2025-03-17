// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package log is used to initialize the logger.
package log

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// -------------------------------------------------------------------
// 1) Public Interfaces - EXACTLY the same as your original definitions
// -------------------------------------------------------------------

// BasicT represents structs capable of logging messages.
// This interface matches seelog.LoggerInterface, but we'll implement
// it using zerolog under the hood.
type BasicT interface {
	// [Formatted variants]
	Tracef(format string, params ...interface{})
	Debugf(format string, params ...interface{})
	Infof(format string, params ...interface{})
	Warnf(format string, params ...interface{}) error
	Errorf(format string, params ...interface{}) error
	Criticalf(format string, params ...interface{}) error

	// [Unformatted variants]
	Trace(v ...interface{})
	Debug(v ...interface{})
	Info(v ...interface{})
	Warn(v ...interface{}) error
	Error(v ...interface{}) error
	Critical(v ...interface{}) error

	// Flush flushes all the messages in the logger.
	Flush()

	// Close flushes all the messages in the logger and closes it. The logger cannot be used after this operation.
	Close()
}

// T represents structs capable of logging messages, plus context management.
type T interface {
	BasicT
	WithContext(context ...string) (contextLogger T)
}

// -------------------------------------------------------------------
// 2) Constants & Global Variables - Keep the same names/signatures
// -------------------------------------------------------------------

const (
	LogFileExtension   = ".log"
	ErrorLogFileSuffix = "errors"
)

var (
	DefaultLogDir      string
	ApplicationLogFile string
	ErrorLogFile       string

	loadedLogger *T
	lock         sync.RWMutex

	// pkgMutex is used for concurrency in the wrapper
	pkgMutex = new(sync.Mutex)
)

// -------------------------------------------------------------------
// 3) LogConfig & Additional Structures (same names, same fields)
// -------------------------------------------------------------------

// LogConfig is the struct holding relevant info for a logger instance.
type LogConfig struct {
	ClientName string
}

// ContextFormatFilter adds context strings to log messages.
type ContextFormatFilter struct {
	Context []string
}

func (f ContextFormatFilter) Filter(params ...interface{}) (newParams []interface{}) {
	newParams = make([]interface{}, len(f.Context)+len(params))
	for i, param := range f.Context {
		newParams[i] = param + " "
	}
	ctxLen := len(f.Context)
	for i, param := range params {
		newParams[ctxLen+i] = param
	}
	return newParams
}

func (f ContextFormatFilter) Filterf(format string, params ...interface{}) (newFormat string, newParams []interface{}) {
	newFormat = ""
	for _, param := range f.Context {
		newFormat += param + " "
	}
	newFormat += format
	newParams = params
	return
}

// -------------------------------------------------------------------
// 4) Our Zerolog-based Implementation
// -------------------------------------------------------------------

// zerologWrapper implements T (which includes BasicT). We store a zerolog.Logger plus context info.
type zerologWrapper struct {
	logger zerolog.Logger
	format ContextFormatFilter
	m      *sync.Mutex // optionally used for concurrency
}

// --- Satisfy BasicT ---

func (w *zerologWrapper) Tracef(format string, params ...interface{}) {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	newFmt, newParams := w.format.Filterf(format, params...)
	msg := fmt.Sprintf(newFmt, newParams...)
	w.logger.Trace().Msg(msg)
}

func (w *zerologWrapper) Debugf(format string, params ...interface{}) {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	newFmt, newParams := w.format.Filterf(format, params...)
	msg := fmt.Sprintf(newFmt, newParams...)
	w.logger.Debug().Msg(msg)
}

func (w *zerologWrapper) Infof(format string, params ...interface{}) {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	newFmt, newParams := w.format.Filterf(format, params...)
	msg := fmt.Sprintf(newFmt, newParams...)
	w.logger.Info().Msg(msg)
}

func (w *zerologWrapper) Warnf(format string, params ...interface{}) error {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	newFmt, newParams := w.format.Filterf(format, params...)
	msg := fmt.Sprintf(newFmt, newParams...)
	w.logger.Warn().Msg(msg)
	return nil
}

func (w *zerologWrapper) Errorf(format string, params ...interface{}) error {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	newFmt, newParams := w.format.Filterf(format, params...)
	msg := fmt.Sprintf(newFmt, newParams...)
	w.logger.Error().Msg(msg)
	return nil
}

func (w *zerologWrapper) Criticalf(format string, params ...interface{}) error {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	newFmt, newParams := w.format.Filterf(format, params...)
	msg := fmt.Sprintf(newFmt, newParams...)
	// No direct "critical" in zerolog: we can log as error or panic
	w.logger.Error().Msg("[CRITICAL] " + msg)
	return nil
}

func (w *zerologWrapper) Trace(v ...interface{}) {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	msg := fmt.Sprint(w.format.Filter(v...)...)
	w.logger.Trace().Msg(msg)
}

func (w *zerologWrapper) Debug(v ...interface{}) {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	msg := fmt.Sprint(w.format.Filter(v...)...)
	w.logger.Debug().Msg(msg)
}

func (w *zerologWrapper) Info(v ...interface{}) {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	msg := fmt.Sprint(w.format.Filter(v...)...)
	w.logger.Info().Msg(msg)
}

func (w *zerologWrapper) Warn(v ...interface{}) error {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	msg := fmt.Sprint(w.format.Filter(v...)...)
	w.logger.Warn().Msg(msg)
	return nil
}

func (w *zerologWrapper) Error(v ...interface{}) error {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	msg := fmt.Sprint(w.format.Filter(v...)...)
	w.logger.Error().Msg(msg)
	return nil
}

func (w *zerologWrapper) Critical(v ...interface{}) error {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()

	msg := fmt.Sprint(w.format.Filter(v...)...)
	w.logger.Error().Msg("[CRITICAL] " + msg)
	return nil
}

func (w *zerologWrapper) Flush() {
	// Zerolog has no buffered flush, so this is a no-op
}

func (w *zerologWrapper) Close() {
	// If the underlying zerolog.Logger had open files, you'd close them here.
	// We just leave it a no-op for demonstration.
}

// --- Satisfy T ---

func (w *zerologWrapper) WithContext(context ...string) (contextLogger T) {
	newCtx := make([]string, 0, len(w.format.Context)+len(context))
	newCtx = append(newCtx, w.format.Context...)
	newCtx = append(newCtx, context...)

	return &zerologWrapper{
		logger: w.logger,
		format: ContextFormatFilter{Context: newCtx},
		m:      w.m,
	}
}

// Helper to avoid repeated lock/unlock calls
func (w *zerologWrapper) lockIfNeeded() {
	if w.m != nil {
		w.m.Lock()
	}
}

func (w *zerologWrapper) unlockIfNeeded() {
	if w.m != nil {
		w.m.Unlock()
	}
}

// ReplaceDelegate lets us swap out the underlying zerolog instance, if desired.
func (w *zerologWrapper) ReplaceDelegate(newLogger zerolog.Logger) {
	w.lockIfNeeded()
	defer w.unlockIfNeeded()
	w.logger = newLogger
}

// -------------------------------------------------------------------
// 5) Functions to Keep the Old Public API, but skip actual config loading
// -------------------------------------------------------------------

// Logger initializes the logging system if not already loaded and returns the logger interface.
//
// Here, we assume the external package has ALREADY configured and returned a zerolog.Logger.
func Logger(useWatcher bool, clientName string) T {
	logConfig := LogConfig{
		ClientName: clientName,
	}
	if !isLoaded() {
		logger := logConfig.InitLogger(useWatcher)
		cache(logger)
	}
	return getCached()
}

// InitLogger just grabs an externally configured logger and wraps it.
func (config *LogConfig) InitLogger(useWatcher bool) (logger T) {
	// Suppose you have some external function: externalpkg.GetLogger() -> zerolog.Logger
	// We'll call that here.
	// For demonstration, let's pretend there's a global or function returning a pre-configured logger.
	zlog := getPreConfiguredZerolog()

	// Wrap it in our T interface
	logger = withContext(zlog)

	// The "watcher" stuff can remain a stub or be removed, depending on your needs
	if useWatcher {
		config.startWatcher(logger)
	}
	return
}

// startWatcher sets up file watching for config changes. Currently a no-op.
func (config *LogConfig) startWatcher(logger T) {
	// If you want dynamic reloading, implement it here.
}

// replaceLogger is a no-op or example of re-getting the external logger.
func (config *LogConfig) replaceLogger() {
	logger := getCached()
	zlog := getPreConfiguredZerolog()

	w, ok := logger.(*zerologWrapper)
	if !ok {
		logger.Errorf("Logger replace failed. The logger is not a zerologWrapper")
		return
	}
	w.ReplaceDelegate(zlog)
}

// withContext creates a new T with optional context.
func withContext(zlog zerolog.Logger, context ...string) T {
	w := &zerologWrapper{
		logger: zlog,
		format: ContextFormatFilter{Context: context},
		m:      pkgMutex,
	}
	return w
}

// isLoaded checks if a logger has been loaded.
func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return loadedLogger != nil
}

// cache stores the loaded logger globally.
func cache(logger T) {
	lock.Lock()
	defer lock.Unlock()
	loadedLogger = &logger
}

// getCached returns the globally cached logger.
func getCached() T {
	lock.RLock()
	defer lock.RUnlock()
	return *loadedLogger
}

// -------------------------------------------------------------------
// 6) Stub for "Pre-Configured" Zerolog Logger
// -------------------------------------------------------------------

// getPreConfiguredZerolog is where you'd call code from a package that
// has already set up a global or returns a fully configured zerolog.Logger.
func getPreConfiguredZerolog() zerolog.Logger {
	// For demonstration, we just create a default logger.
	// In reality, you'd do something like:
	//     return externalpkg.GetGlobalLogger()
	return log.Logger
}
