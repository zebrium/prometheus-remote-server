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

package zmodule

// This module export module interface to all other module,

import (
	"container/list"

	"zebrium.com/stserver/common/zlog"
)

var modules *list.List

type ModuleIface interface {
	Init() error
	Start() error
	Alive() bool
	Stop() error
	Fini() error
	Name() string
}

func RegisterModule(mod ModuleIface) error {
	if modules == nil {
		modules = list.New()
	}

	zlog.Debug("Registering module %s\n", mod.Name())
	modules.PushBack(mod)
	return nil
}

func Init() {
	if modules == nil {
		return
	}
	for e := modules.Front(); e != nil; e = e.Next() {
		var mod ModuleIface = e.Value.(ModuleIface)
		zlog.Debug("Initializing module %s\n", mod.Name())
		if err := mod.Init(); err != nil {
			zlog.Fatal("Failed to Init module %s\n", mod.Name())
		}
	}
}

func Start() {
	if modules == nil {
		return
	}
	for e := modules.Front(); e != nil; e = e.Next() {
		var mod ModuleIface = e.Value.(ModuleIface)
		zlog.Debug("Starting module %s\n", mod.Name())
		if err := mod.Start(); err != nil {
			zlog.Fatal("Failed to Start module %s\n", mod.Name())
		}
	}
}

func Alive() bool {
	if modules == nil {
		return true
	}
	ret := true
	for e := modules.Front(); e != nil; e = e.Next() {
		var mod ModuleIface = e.Value.(ModuleIface)
		if !mod.Alive() {
			zlog.Error("Module %s failed liveness check\n", mod.Name())
			ret = false
		}
	}
	return ret
}

func Stop() {
	if modules == nil {
		return
	}
	for e := modules.Back(); e != nil; e = e.Prev() {
		var mod ModuleIface = e.Value.(ModuleIface)
		zlog.Debug("Stopping module %s\n", mod.Name())
		if err := mod.Stop(); err != nil {
			zlog.Fatal("Failed to Stop module %s\n", mod.Name())
		}
	}
}

func Fini() {
	if modules == nil {
		return
	}
	for e := modules.Back(); e != nil; e = e.Prev() {
		var mod ModuleIface = e.Value.(ModuleIface)
		zlog.Debug("Fini module %s\n", mod.Name())
		if err := mod.Fini(); err != nil {
			zlog.Fatal("Failed to Fini module %s\n", mod.Name())
		}
	}
}
