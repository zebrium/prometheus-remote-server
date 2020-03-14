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

package worker

// All the exported APIs from worker go here.

import (
	"zebrium.com/stserver/common/zmodule"

	"zebrium.com/stserver/prometheus/ztypes"
)

type WorkerAPI struct{}

var glAPI WorkerAPI

func GetWorkerAPI() *WorkerAPI {
	return &glAPI
}

type WorkerModType struct{}

var WorkerModInst WorkerModType

// Authenticates the token.
func (w *WorkerAPI) CheckToken(token string) (string, int, error) {
	return getWorkerInfo().lookupToken(token)
}

// Entry point for the request.
func (w *WorkerAPI) ProcessRequest(rid int32, account string, token string, nbytes uint64, cbytes uint64, req ztypes.StWbReq) ([]int8, error) {
	return getWorkerInfo().processRequest(rid, account, token, nbytes, cbytes, req)
}

func (mod *WorkerModType) Init() error {
	return nil
}

func (mod *WorkerModType) Start() error {
	getWorkerInfo().startWorker()
	return nil
}

func (mod *WorkerModType) Alive() bool {
	return true
}

func (mod *WorkerModType) Stop() error {
	getWorkerInfo().stopWorker()
	return nil
}

func (mod *WorkerModType) Fini() error {
	return nil
}

func (mod *WorkerModType) Name() string {
	return "worker"
}

func RegisterModule() {
	zmodule.RegisterModule(&WorkerModInst)
}
