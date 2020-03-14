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

package ztypes

import (
	"zebrium.com/stserver/prometheus/pkg/labels"
)

// Content of HTTP request.
type StWbReq struct {
	Version int8        // Version of this request.
	Token   string      // Token to identify account.
	NInsts  int32       // Number of instances in this request.
	IData   []*StWbIReq // Data for N number of instances.
}

// Content of HTTP response.
type StWbResp struct {
	Version     int8   // Version of this response.
	MinPollSecs int8   // Minimum polling interval allowed (0 means no limit).
	Err         string // Error: empty string for success.
	NInsts      int32  // Number of responses in this.
	IErrCodes   []int8 // // Error codes for each instance, if Err is 0 above. 0 for success. 1 for send full blob.
}

// One scrape instance's blob.
type StWbIReq struct {
	Instance string       // Instance name.
	NSamples int          // Number of samples.
	IsIncr   bool         // Is this incremental blob?
	Gen      int64        // Generation number.
	GTs      int64        // Global timestamp for all samples, if the sample is not there.
	Samples  []StWbSample // Samples.
	OLabels  []string     // Optional label names.
}

// One stat sample.
type StWbSample struct {
	Idx  int32         // Index In the array. Used for incremental blob (helps to not send same values). Should be ascending in slice.
	Ls   labels.Labels // Labels, empty for incremental blob.
	Help string        // Help info. empty for incremental blob.
	Type string        // Type of this counter. empty for incremental blob.
	Ts   int64         // Time stamp. diff for incremental blob.
	Val  float64       // Value. diff for incremental blob.
}

type ZQueueCb interface {
	SendCompleted(err error, mps int)
}

// Tuple that contains full and optionally an incremental blob.
type ZBlobTuple struct {
	FlBlob    *StWbIReq // Full Blob.
	IncrBlob  *StWbIReq // Incremental Blob.
	Cb        ZQueueCb  // Callback info.
	SkipWaitQ bool      // Should we skip the waitq for this one.
}
