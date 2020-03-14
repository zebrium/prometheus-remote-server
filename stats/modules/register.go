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

package modules

import (
	"zebrium.com/stserver/stats/bridger"
	"zebrium.com/stserver/stats/worker"
	"zebrium.com/stserver/stats/zvcacher"

	"zebrium.com/stserver/common/zlog"
)

// This exports API to register all the modules of the merge executor in the right order.

// RegisterModules registers the various merge modules for startup/teardown
func RegisterModules() {
	zlog.Info("Registering modules")
	// Ordering matters.
	zvcacher.RegisterModule()
	bridger.RegisterModule()
	worker.RegisterModule()
}
