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

package config

import (
	"os"

	"zebrium.com/stserver/common/zconfig"
	"zebrium.com/stserver/common/zlog"
)

const (
	// ConstConfigFilepath is the path of the stats config
	ConstConfigFilepath = "/data/zebrium/etc/config/stats.json.txt"
)

// GlobalConfig is the structure to containe merge config data
type GlobalConfig struct {
	StatsEnabled bool // Is stats server enabled?

	StatsHTTPSEnabled bool // Is stats https server enabled?
	StatsHTTPPort     int  // Port to listen on

	StatsStingestHost    string // Host running stingest
	StatsStingestPort    int    // Port where stingest is listening
	StatsStingestTimeout int    // Timeout for requests to stingest

	StatsKeepTmpFiles bool // Don't delete temporary files

	StatsReqsMax int    // Max number of requests to process at once
	StatsReqsDir string // Location for stats reqs data

	StatsSerializerQD int // Serializer queue depth: number of web requests to process concurrently.

	ZVCacherDropLWSecs int // Low water mark time threshold for dropping cache entries.
	ZVCacherDropHWSecs int // High water mark time threshold for dropping cache entries.

	ZVCacherEntriesLW int // Low wanter mark threshold count of cache entries.
	ZVCacherEntriesHW int // High wanter mark threshold count of cache entries.

	ZVCacherCacheBuckets int // Number of cache buckets.

	BrdigerStIngestsQD    int  // Bridger, Max stingests to submit in parallel.
	BrdigerNotifyStIngest bool // Should we notify stingest of new requests?
}

// GlCfg is the global structure containing the config data
var GlCfg GlobalConfig

func init() {
	zval, zok := os.LookupEnv("ZEBRIUM_STATS_CONFIG_FILE_LOC")
	if zok && len(zval) > 0 {
		zconfig.ReadZConfig(zval)
	} else {
		zconfig.ReadZConfig(ConstConfigFilepath)
	}

	GlCfg.StatsEnabled = (zconfig.GetZConfigInt("stats.enabled") == 1)
	GlCfg.StatsHTTPSEnabled = (zconfig.GetZConfigInt("stats.https_enabled") == 1)
	GlCfg.StatsHTTPPort = zconfig.GetZConfigInt("stats.http_port")
	GlCfg.StatsStingestHost = zconfig.GetZConfigStr("stats.stingest_host")
	GlCfg.StatsStingestPort = zconfig.GetZConfigInt("stats.stingest_port")
	GlCfg.StatsStingestTimeout = zconfig.GetZConfigInt("stats.stingest_timeout")
	GlCfg.StatsKeepTmpFiles = (zconfig.GetZConfigInt("stats.keep_tmp_files") == 1)
	GlCfg.StatsReqsMax = zconfig.GetZConfigInt("stats.reqs_max")
	GlCfg.StatsReqsDir = zconfig.GetZConfigStr("stats.reqs_dir")
	GlCfg.StatsSerializerQD = zconfig.GetZConfigInt("stats.serializer_qd")

	GlCfg.ZVCacherDropLWSecs = zconfig.GetZConfigInt("zvcacher.drop_after_secs_lw")
	GlCfg.ZVCacherDropHWSecs = zconfig.GetZConfigInt("zvcacher.drop_after_secs_hw")
	GlCfg.ZVCacherEntriesLW = zconfig.GetZConfigInt("zvcacher.cache_entries_low_wmark")
	GlCfg.ZVCacherEntriesHW = zconfig.GetZConfigInt("zvcacher.cache_entries_high_wmark")
	GlCfg.ZVCacherCacheBuckets = zconfig.GetZConfigInt("zvcacher.cache_bkts")

	GlCfg.BrdigerStIngestsQD = zconfig.GetZConfigInt("bridger.stingests_qd")
	GlCfg.BrdigerNotifyStIngest = (zconfig.GetZConfigInt("bridger.notify_stingest") != 0)
}

func Dump() {
	zlog.Info("Stats Configs:\n")
	zlog.Info(" StatsEnabled %v\n", GlCfg.StatsEnabled)
	zlog.Info(" StatsHttpsEnabled %v\n", GlCfg.StatsHTTPSEnabled)
	zlog.Info(" StatsHttpPort %d\n", GlCfg.StatsHTTPPort)
	zlog.Info(" StatsStingestHost %s\n", GlCfg.StatsStingestHost)
	zlog.Info(" StatsStingestPort %d\n", GlCfg.StatsStingestPort)
	zlog.Info(" StatsKeepTmpFiles %d\n", GlCfg.StatsKeepTmpFiles)
	zlog.Info(" StatsReqsMax %d\n", GlCfg.StatsReqsMax)
	zlog.Info(" StatsReqsDir %s\n", GlCfg.StatsReqsDir)
	zlog.Info(" StatsSerializerQD %d\n", GlCfg.StatsSerializerQD)
	zlog.Info(" ZVCacherDropLWSecs %d\n", GlCfg.ZVCacherDropLWSecs)
	zlog.Info(" ZVCacherDropHWSecs %d\n", GlCfg.ZVCacherDropHWSecs)
	zlog.Info(" ZVCacherEntriesLW %d\n", GlCfg.ZVCacherEntriesLW)
	zlog.Info(" ZVCacherEntriesHW %d\n", GlCfg.ZVCacherEntriesHW)
	zlog.Info(" ZVCacherCacheBuckets %d\n", GlCfg.ZVCacherCacheBuckets)
	zlog.Info(" BrdigerStIngestsQD %d\n", GlCfg.BrdigerStIngestsQD)
	zlog.Info(" BrdigerNotifyStIngest %v\n", GlCfg.BrdigerNotifyStIngest)
}
