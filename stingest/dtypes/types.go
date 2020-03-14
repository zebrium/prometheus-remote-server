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

package dtypes

import (
	"crypto/sha1"
	"strings"

	"zebrium.com/stserver/common/zassert"
)

// All the data types that are common across different stats ingest packages.

type KvPair struct {
	N string `json:"key"`   // Name.
	V string `json:"value"` // Value.
}

type KvPairs []KvPair

func (kvs KvPairs) Len() int      { return len(kvs) }
func (kvs KvPairs) Swap(i, j int) { kvs[i], kvs[j] = kvs[j], kvs[i] }
func (kvs KvPairs) Less(i, j int) bool {
	// We want ze_ keys to be first.
	if strings.HasPrefix(kvs[i].N, "ze_") && !strings.HasPrefix(kvs[j].N, "ze_") {
		return true
	}
	if strings.HasPrefix(kvs[j].N, "ze_") && !strings.HasPrefix(kvs[i].N, "ze_") {
		return false
	}

	return kvs[i].N < kvs[j].N
}

// Metadata file format.
type StFileMeta struct {
	Account      string   `json:"account"`       // Account
	Filetype     string   `json:"type"`          // File type, eg: "prometheus"
	Iid          string   `json:"iid"`           // Unique instance id  (instance + "." + job).
	IsCompressed bool     `json:"is_compressed"` // Is the data file compressed.
	Kvs          []KvPair `json:"kvs"`           // Global kvs to append to all records.
	DTs          int64    `json:"dts"`           // Global timestamp for all samples, if the sample is not there.
	OLabels      []string `json:"olabels"`       // Optional label names.
}

type StRecordType int8

const (
	StRecord_COUNTER   StRecordType = 0
	StRecord_GAUGE     StRecordType = 1
	StRecord_HISTOGRAM StRecordType = 2
	StRecord_SUMMARY   StRecordType = 3
)

type RecordDropType int8

const (
	RecordDrop_NODROP  RecordDropType = 0
	RecordDrop_MISSING RecordDropType = 1
	RecordDrop_STALE   RecordDropType = 2
)

type StRecord struct {
	Mnm  string       // Metric full name.
	Mhlp []byte       // Metric help name.
	Kvs  KvPairs      // Vector of key/value pairs, sorted alphabetically by key.
	Ts   int64        // Sample time stamp in epoch.
	Sv   float64      // Sample value.
	Tp   StRecordType // Record type.
}

type MtRecord struct {
	Snm      string          // Stat's group name (aka view or aka etype in logs).
	Suidx    int             // Stat's group unique index.
	Svar_num int             // svar number in the all_stats database.
	Kid      uint64          // Hash of all keys together (not values) sorted.
	Mid      uint64          // Metric id (hash of metric name and type, just computed one).
	Kvid     uint64          // Hash of all the keys and values together sorted (just computed once).
	MKvid    uint64          // Hash of metric name and all the keys and values (time-series ID).
	SKvid    uint64          // Hash of stat group name and all keys and values (stat generator ID).
	Vid      [sha1.Size]byte // Sha1 of all the Values (not the keys).
	Lsv      float64         // Last sample value.
	Lts      int64           // Last sample's time stamp.
	Drop     RecordDropType  // Should we drop this record?
}

type DiRecord struct {
	Snm   string       // Stat's group name (aka view or aka etype in logs).
	Suidx int          // Stat's group unique index.
	Tp    StRecordType // Record type.
	Kid   uint64       // Hash of all keys together (not values) sorted.
	SKvid uint64       // Hash of snm, keys and values (snm first)
	Ts    int64        // Sample time stamp in epoch.
	Lts   int64        // Last time stamp in epoch.
	Cs    []float64    // Counter values, either diff'd or actual values.
	// Ds    []string        // Dimensions, values for each key.
	Vid [sha1.Size]byte // Sha1 of all the Values (not the keys).
}

type StErrorCode int32

const (
	ErrorCode_OK                StErrorCode = 0
	ErrorCode_DB_FAILURE        StErrorCode = 1
	ErrorCode_PROMPARSER_FAILED StErrorCode = 2
	ErrorCode_NAMING_FAILED     StErrorCode = 3
	ErrorCode_DIFFCACHER_FAILED StErrorCode = 4
	ErrorCode_BEQ_Q_FAILED      StErrorCode = 5
)

func SterrorcodeString(e StErrorCode) string {
	switch e {
	case ErrorCode_OK:
		return "ErrorCode_OK"
	case ErrorCode_DB_FAILURE:
		return "ErrorCode_DB_FAILURE"
	case ErrorCode_PROMPARSER_FAILED:
		return "ErrorCode_PROMPARSER_FAILED"
	case ErrorCode_NAMING_FAILED:
		return "ErrorCode_NAMING_FAILED"
	case ErrorCode_DIFFCACHER_FAILED:
		return "ErrorCode_DIFFCACHER_FAILED"
	case ErrorCode_BEQ_Q_FAILED:
		return "ErrorCode_BEQ_Q_FAILED"
	}
	return "ErrorCode_UNKNOWN"
}

type StFeRes struct {
	Req *StFeReq    // Original request.
	Err StErrorCode // Error code.
}

type StFeReq struct {
	Account string         // Account.
	DFpath  string         // Data file path.
	MFpath  string         // Metadata file path.
	Iid     string         // Unique Instance id per account  (instance + "." + job).
	DTs     int64          // Default time stamp to use for all records, epoch in msecs.
	Ftype   string         // File data type. eg: "prometheus".
	IidHash uint64         // xxhash64 of account and iid.
	Epoch   int64          // Last update to request
	Status  StFeReqStatus  // Status of the request
	Bytes   int            // Size of request payload (data+metadata)
	ResCh   *chan *StFeRes // Completion channel to reply for this request.
	OLabels []string       // Optional label names.
}

type StBeReq struct {
	Account string // Account.
	DFpath  string // Data file path.
	MFpath  string // Metadata file path.
	Iid     string // Unique Instance id per account  (instance + "." + job).
}

func GetStypeStr(stype StRecordType) string {
	switch stype {
	case StRecord_COUNTER:
		return "counter"
	case StRecord_GAUGE:
		return "gauge"
	case StRecord_HISTOGRAM:
		return "histogram"
	case StRecord_SUMMARY:
		return "summary"
	default:
		zassert.Zdie("Unknown stype %d", stype)
		return "unknown"
	}
}

type StFeReqStatus int32

const (
	StFeReqStatusQueued  StFeReqStatus = 0
	StFeReqStatusActive  StFeReqStatus = 1
	StFeReqStatusUnknown StFeReqStatus = 2
)

func GetStFeReqStatusStr(status StFeReqStatus) string {
	switch status {
	case StFeReqStatusQueued:
		return "Queued"
	case StFeReqStatusActive:
		return "Active"
	default:
		return "Unknown"
	}
}
