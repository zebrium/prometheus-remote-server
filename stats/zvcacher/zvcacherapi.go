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

package zvcacher

import (
	"zebrium.com/stserver/common/zmodule"
	"zebrium.com/stserver/prometheus/ztypes"
	"zebrium.com/stserver/stats/zstypes"
)

type ZVCacherAPI struct {
}

var glAPI ZVCacherAPI

func GetZVCacher() *ZVCacherAPI {
	return &glAPI
}

func (c *ZVCacherAPI) UpdateAndGet(account string, wreq *ztypes.StWbIReq) (*zstypes.InsCacheInfo, error) {
	return glCache.updateAndGet(account, wreq)
}

func (c *ZVCacherAPI) Put(ins *zstypes.InsCacheInfo) error {
	return glCache.put(ins)
}

type ZVCacherModType struct{}

var ZVCacherModInst ZVCacherModType

func (mod *ZVCacherModType) Init() error {
	return nil
}

func (mod *ZVCacherModType) Start() error {
	startWorker()
	return nil
}

func (mod *ZVCacherModType) Alive() bool {
	return true
}

func (mod *ZVCacherModType) Stop() error {
	stopWorker()
	return nil
}

func (mod *ZVCacherModType) Fini() error {
	return nil
}

func (mod *ZVCacherModType) Name() string {
	return "zvcacher"
}

func RegisterModule() {
	zmodule.RegisterModule(&ZVCacherModInst)
}
