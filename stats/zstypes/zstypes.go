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

package zstypes

import (
	"zebrium.com/stserver/prometheus/pkg/labels"
)

type InsSample struct {
	Ls   labels.Labels // Labels.
	Help string        // Help.
	Type string        // Type.
	Ts   int64         // Time stamp.
	Val  float64       // Value.
}

type InsCacheInfo struct {
	AName   string        // Account name.
	IName   string        // Instance name (instance + "." + job).
	Gen     int64         // Generation number.
	LAts    int64         // Last active time stamp in msecs epoch. (for internal use by zvcacher only)
	Next    *InsCacheInfo // Next in the cache (for internal use by zvcacher only).
	Samples []InsSample   // Samples.
	DTs     int64         //  Global timestamp for all samples, if the sample is not there.
	OLabels []string      // Optional label names.
}
