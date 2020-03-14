// Copyright 2019 Zebrium Inc
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Simple formatter to set log format

// NOTE: DO NOT SEND ANY OUPUT TO STDOUT. All the output SHOULD go to stderr, as some programs assume this from this package.

package zlog

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sync"
	"time"
)

// Severity is the syslog numeric severity for a message
type Severity int

const (
	OFF   = -1 // 1 would be better but syslog has reserved that
	CRIT  = 2
	ERROR = 3
	WARN  = 4
	NOTE  = 5
	INFO  = 6
	DEBUG = 7
)

type zlogState struct {
	o    io.Writer
	l    Severity
	sevs []string
	m    sync.Mutex
}

var glState zlogState

func init() {
	glState.o = os.Stdout
	glState.l = INFO
	glState.sevs = []string{"", "", "CRIT:", "ERROR:", "WARN:", "NOTE:", "INFO:", "DEBUG:"}
}

// SetOutput to send output to somewhere other than stdout
func SetOutput(w io.Writer) {
	glState.m.Lock()
	glState.o = w
	glState.m.Unlock()
}

func SetTraceLevel(l Severity) {
	glState.m.Lock()
	glState.l = l
	glState.m.Unlock()
}

func printf(sev Severity, format string, v ...interface{}) {

	// Skip messages below level
	if sev > glState.l || sev == OFF || glState.l == OFF {
		return
	}
	// Append trailing newline
	if len(format) == 0 || format[len(format)-1] != '\n' {
		format = format + "\n"
	}

	// Get runtime context
	pc, file, line, _ := runtime.Caller(2)
	timeStr := time.Now().Local().Format("2006-01-02T15:04:05.000-07:00")

	prefix := fmt.Sprintf("%s %5d %-6s %s:%d %s: ", timeStr, os.Getpid(), glState.sevs[int(sev)], path.Base(file), line, path.Base(runtime.FuncForPC(pc).Name()))
	fmt.Fprintf(glState.o, prefix+format, v...)
}

func Printf(format string, v ...interface{}) {
	printf(INFO, format, v...)
}

func Fatalf(format string, v ...interface{}) {
	printf(CRIT, format, v...)
	os.Exit(1)
}

func Debug(format string, v ...interface{}) {
	printf(DEBUG, format, v...)
}

func Info(format string, v ...interface{}) {
	printf(INFO, format, v...)
}

func Note(format string, v ...interface{}) {
	printf(NOTE, format, v...)
}

func Warn(format string, v ...interface{}) {
	printf(WARN, format, v...)
}

func Error(format string, v ...interface{}) {
	printf(ERROR, format, v...)
}

func Fatal(format string, v ...interface{}) {
	printf(CRIT, format, v...)
	os.Exit(1)
}

// Printf provides formatted log output
func (writer *Writer) Printf(format string, v ...interface{}) {
	printf(INFO, format, v...)
}

// Fatalf provides formatted log output
func (writer *Writer) Fatalf(format string, v ...interface{}) {
	printf(CRIT, format, v...)
	os.Exit(1)
}

// Writer exists as a wrapper for legacy code
type Writer struct {
}
