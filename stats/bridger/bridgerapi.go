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

package bridger

// All the exported APIs from bridger go here.

import (
	"zebrium.com/stserver/common/zmodule"

	"zebrium.com/stserver/stats/zstypes"
)

type BridgerAPI struct{}

var glAPI BridgerAPI

func GetBridgerAPI() *BridgerAPI {
	return &glAPI
}

// Entry point for taking the request and writing to local temp files followed by sending them
// to the next stage for further processing.
func (*BridgerAPI) Push(ins *zstypes.InsCacheInfo) (uint64, uint64, error) {
	return getBridgerInfo().pushInstance(ins)
}

type BridgerModType struct{}

var BridgerModInst BridgerModType

func (mod *BridgerModType) Init() error {
	return nil
}

func (mod *BridgerModType) Start() error {
	getBridgerInfo().startWorker()
	return nil
}

func (mod *BridgerModType) Alive() bool {
	return true
}

func (mod *BridgerModType) Stop() error {
	getBridgerInfo().stopWorker()
	return nil
}

func (mod *BridgerModType) Fini() error {
	return nil
}

func (mod *BridgerModType) Name() string {
	return "bridger"
}

func RegisterModule() {
	zmodule.RegisterModule(&BridgerModInst)
}
