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

package zassert

// This module exports assertion API

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"zebrium.com/stserver/common/zlog"
)

func Zassert_equal(val1 interface{}, val2 interface{}) {
	if val1 != val2 {
		_, fn, line, _ := runtime.Caller(1)
		zlog.Error("Stack trace %s", debug.Stack())
		zlog.Fatal("Assertion failed %v (%T) != %v (%T) at %s:%d\n", val1, val1, val2, val2, fn, line)
	}
}

func Zassert_notequal(val1 interface{}, val2 interface{}) {
	if val1 == val2 {
		_, fn, line, _ := runtime.Caller(1)
		zlog.Error("Stack trace %s", debug.Stack())
		zlog.Fatal("Assertion failed %v (%T) != %v (%T) at %s:%d\n", val1, val1, val2, val2, fn, line)
	}
}

func Zassert(cond bool, msg string, args ...interface{}) {
	if !cond {
		_, fn, line, _ := runtime.Caller(1)
		zlog.Error("Stack trace %s", debug.Stack())
		zlog.Fatal("Assertion failed at %s:%d %s", fn, line, fmt.Sprintf(msg, args...))
	}
}

func Zchk_equal(val1 interface{}, val2 interface{}) {
	if val1 != val2 {
		_, fn, line, _ := runtime.Caller(1)
		zlog.Error("Stack trace %s", debug.Stack())
		zlog.Fatal("Assertion failed %v (%T) != %v (%T) at %s:%d\n", val1, val1, val2, val2, fn, line)
	}
}

func Zchk_notequal(val1 interface{}, val2 interface{}) {
	if val1 == val2 {
		_, fn, line, _ := runtime.Caller(1)
		zlog.Error("Stack trace %s", debug.Stack())
		zlog.Fatal("Assertion failed %v (%T) != %v (%T) at %s:%d\n", val1, val1, val2, val2, fn, line)
	}
}

func Zchk(cond bool, msg string, args ...interface{}) {
	if !cond {
		_, fn, line, _ := runtime.Caller(1)
		zlog.Error("Stack trace %s", debug.Stack())
		zlog.Fatal("Assertion failed at %s:%d %s", fn, line, fmt.Sprintf(msg, args...))
	}
}

func Zdie(msg string, args ...interface{}) {
	_, fn, line, _ := runtime.Caller(1)
	zlog.Error("Stack trace %s", debug.Stack())
	zlog.Fatal("Zdie at at %s:%d %s", fn, line, fmt.Sprintf(msg, args...))
}
