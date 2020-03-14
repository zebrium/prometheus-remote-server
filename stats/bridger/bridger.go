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

// This module takes the full blob of stats and does two things:
// 1. Creates the local prometheus stat files.
// 2. Sends a GRPC, if configured to notify of this new stat file for further processing.
//    Further software pipeline processing, typically works on this file:
//      by learning metrics and grouping the related ones for anamoly detection.
//      loading the grouped stats to an analytics database where we can run further
//      machine learning to do anamoly detection.

import (
	"bytes"
	"compress/gzip"
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zebrium.com/stserver/common/zassert"
	"zebrium.com/stserver/common/zlog"
	"zebrium.com/stserver/common/zutils"
	"google.golang.org/grpc"

	"zebrium.com/stserver/prometheus/pkg/labels"

	"zebrium.com/stserver/stingest/dtypes"

	stingestpb "zebrium.com/stserver/proto/stingest"

	"zebrium.com/stserver/stats/config"
	"zebrium.com/stserver/stats/zstypes"
)

// Request to send for further processing.
type stiReq struct {
	tmpDir string                         // Temp directory, where we wrote the stats.
	req    *stingestpb.StatsIngestRequest // GRPC request.
	cchan  *chan bool                     // Completion channel.
}

// If we need to change some known metric names, like moveing key_values into metric names.
type MnameAdjusterInfo struct {
	mname      string   // Metric name to look for.
	ylabels    []string // Yes, labels names that are needed.
	slabel     string   // Selector label. that we take the value of it, and appends to metric name.
	svalueMLen int      // Max allowed length for the selected selector label's value.
	tfilter    string   // type filter.
}

// Module info.
type BridgerInfo struct {
	stiReqCh  chan *stiReq // Stingest requests channel.
	stiRespCh chan error   // stingest response channel from workers.

	stiRnq *list.List // Runq of requests (*stingestpb.StatsIngestRequest)
	stiCQD int        // Current worker queue depth.
	stiMQD int        // Max queue depth.

	exitCh     chan string    // Exit channel.
	exitWaiter sync.WaitGroup // Exit waiter.
	stopping   bool           // Are we stopping?
	alldone    bool           // Is all done for stopping?

	stingestWorkerReady int32 // Is stingest worker ready to accept requests?

	mnamesMap map[string]*MnameAdjusterInfo // Metric name adjust info map.
}

var binfo BridgerInfo

func getBridgerInfo() *BridgerInfo {
	return &binfo
}

// State machine that limits the maximum number of simulatanious requests that
// can be active for the next stage of processing.
func (bi *BridgerInfo) mainStateMachine() {
	if bi.stopping && bi.stiCQD == 0 {
		bi.alldone = true
		return
	}

	for bi.stiRnq.Len() > 0 && bi.stiCQD < bi.stiMQD {
		e := bi.stiRnq.Front()
		req := e.Value.(*stiReq)
		bi.stiRnq.Remove(e)
		bi.stiCQD++

		go bi.ingestInstanceWorker(req.tmpDir, req.req)
	}

	if bi.stiCQD > 0 || bi.stiRnq.Len() > 0 {
		zlog.Info("BRIDGER-STATS: rnq %d active %d\n", bi.stiRnq.Len(), bi.stiCQD)
	}
}

func (bi *BridgerInfo) processStiResponse(err error) {
	zassert.Zassert(bi.stiCQD > 0, "Invalid qd %d", bi.stiCQD)
	bi.stiCQD--
	zlog.Info("BRIDGER-STATS: rnq %d active %d\n", bi.stiRnq.Len(), bi.stiCQD)
}

func (bi *BridgerInfo) processStiRequest(req *stiReq) {
	bi.stiRnq.PushBack(req)
}

// During a process restart, all the files that were not processed
// gets deleted. It should be ok, as they get resent from the scraper.
func (bi *BridgerInfo) replayLocalFIleReqs() {
	// For now, just delete the old stats that were not sent to stingest.
	err := zutils.ReuseDir(config.GlCfg.StatsReqsDir)
	if err != nil {
		zassert.Zdie("Cannot reuse dir %s, err %s", config.GlCfg.StatsReqsDir, err)
	}

	atomic.StoreInt32(&bi.stingestWorkerReady, 1)
}

// State machine thread, that limit the maximum number of simulatanious requests that
// can be active for the next stage of processing.
func (bi *BridgerInfo) stIngestWorker() {
	bi.replayLocalFIleReqs()

	for !bi.alldone {
		select {
		case req := <-bi.stiReqCh:
			bi.processStiRequest(req)
			bi.mainStateMachine()
		case res := <-bi.stiRespCh:
			bi.processStiResponse(res)
			bi.mainStateMachine()
		case <-bi.exitCh:
			bi.mainStateMachine()
		}
	}

	zlog.Info("Worker serializer thread exited")
	bi.exitWaiter.Done()
}

// Worker that does the work of informing to the next stage.
func (bi *BridgerInfo) ingestInstanceWorker(tmpDir string, req *stingestpb.StatsIngestRequest) {
	zlog.Info("Sending to stingest for Ingesting files at %s for account=%s instance=%s", tmpDir, req.Account, req.InstanceId)

	host := config.GlCfg.StatsStingestHost + ":" + strconv.Itoa(config.GlCfg.StatsStingestPort)
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		zlog.Error("Failed to connect: %s\n", err)
		bi.stiRespCh <- err
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.GlCfg.StatsStingestTimeout)*time.Second)
	defer cancel()

	c := stingestpb.NewStatsIngestSvcClient(conn)

	r, err := c.Ingest(ctx, req)

	if err != nil {
		zlog.Error("could not talk to stats ingest: %s\n", err)
		bi.stiRespCh <- err
		return
	}
	bi.stiRespCh <- nil

	zlog.Info("stingest response for account %s instance %s : %v\n", req.Account, req.InstanceId, r.Err)

	if r.Err != stingestpb.StatsIngestErrorCode_OK {
		zlog.Error("Could not ingest from %s: %v\n", tmpDir, r.Err)
	}
}

// We adjust some known metric names, to help our grouping algorithm in the next stage.
func (bi *BridgerInfo) adjustMetricName(name string, ls labels.Labels, stype string) (string, string) {

	if minfo, isok := bi.mnamesMap[name]; isok {
		if len(minfo.tfilter) > 0 && minfo.tfilter != stype {
			return name, ""
		}

		svalue := ""
		for _, ml := range minfo.ylabels {

			isok = false
			for _, l := range ls {

				if len(svalue) == 0 && l.Name == minfo.slabel {
					if len(l.Value) > minfo.svalueMLen {
						return name, ""
					}
					svalue = "_" + l.Value

					// skip this label.
				}

				if l.Name == ml {
					isok = true
					break
				}
			}

			if !isok {
				return name, ""
			}

		}

		return name + svalue, minfo.slabel
	}

	return name, ""
}

func (bi *BridgerInfo) getLocalLabelsString(clblsMap map[string]string, ls labels.Labels, slabel string) string {
	var b bytes.Buffer

	nr := 0
	b.WriteByte('{')
	for _, l := range ls {
		if l.Name == labels.MetricName || (len(slabel) > 0 && slabel == l.Name) {
			continue
		}
		val, isok := clblsMap[l.Name]
		if isok && val == l.Value {
			continue
		}
		if nr > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(l.Value))
		nr++
	}
	b.WriteByte('}')

	return b.String()
}

// Writes the full blob into plain text stats file.
func (bi *BridgerInfo) writeStatsFile(fpath string, ins *zstypes.InsCacheInfo) (map[string]string, uint64, uint64, error) {
	tbytes := uint64(0)

	fp, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE, 0644)
	var ntstamps = uint64(0)
	if err != nil {
		return nil, ntstamps, tbytes, err
	}
	defer func() {
		err := fp.Close()
		if err != nil {
			zlog.Error("Failed to close file %s, err %s", fpath, err)
		}
	}()

	w, err := gzip.NewWriterLevel(fp, gzip.BestSpeed)
	if err != nil {
		zlog.Error("Could not create gzip writer for file %s err %s", fpath, err)
		return nil, ntstamps, tbytes, err
	}

	clblsMap := make(map[string]string) // Common labels map.
	if len(ins.Samples) > 0 {
		for i := 0; i < len(ins.Samples[0].Ls); i++ {
			clblsMap[ins.Samples[0].Ls[i].Name] = ins.Samples[0].Ls[i].Value
		}
	}
	for _, s := range ins.Samples {
		tbytes += uint64(len(s.Help) + len(s.Type) + 16)
		for _, clbl := range s.Ls {
			tbytes += uint64(len(clbl.Name) + len(clbl.Value))
			val, isok := clblsMap[clbl.Name]
			if isok && val != clbl.Value {
				delete(clblsMap, clbl.Name)
			}
		}
	}

	lmName := ""
	for _, s := range ins.Samples {
		oname := s.Ls.Get(labels.MetricName)
		if len(oname) == 0 {
			zlog.Warn("Missing metric name for sample %v", s)
			continue
		}
		name, slabel := bi.adjustMetricName(oname, s.Ls, s.Type)
		if len(slabel) > 0 {
			delete(clblsMap, slabel)
		}

		baseMname := name
		if s.Type == "summary" {
			if strings.HasSuffix(baseMname, "_sum") {
				baseMname = strings.TrimSuffix(baseMname, "_sum")
			} else if strings.HasSuffix(baseMname, "_count") {
				baseMname = strings.TrimSuffix(baseMname, "_count")
			}
		} else if s.Type == "histogram" {
			if strings.HasSuffix(baseMname, "_sum") {
				baseMname = strings.TrimSuffix(baseMname, "_sum")
			} else if strings.HasSuffix(baseMname, "_count") {
				baseMname = strings.TrimSuffix(baseMname, "_count")
			} else if strings.HasSuffix(baseMname, "_bucket") {
				baseMname = strings.TrimSuffix(baseMname, "_bucket")
			}
		}

		if lmName != baseMname {
			fmt.Fprintf(w, "# HELP %s %s\n", baseMname, s.Help)
			if s.Type == "unknown" {
				fmt.Fprintf(w, "# TYPE %s %s\n", baseMname, "untyped")
			} else {
				fmt.Fprintf(w, "# TYPE %s %s\n", baseMname, s.Type)
			}
		}
		if s.Ts != ins.DTs {
			ntstamps++
			fmt.Fprintf(w, "%s%s %f %d\n", name, bi.getLocalLabelsString(clblsMap, s.Ls, slabel), s.Val, s.Ts)
		} else {
			fmt.Fprintf(w, "%s%s %f\n", name, bi.getLocalLabelsString(clblsMap, s.Ls, slabel), s.Val)
		}

		lmName = baseMname
	}

	err = w.Close()
	if err != nil {
		zlog.Error("Could not close gzip writer for file %s err %s", fpath, err)
		return nil, ntstamps, tbytes, err
	}

	return clblsMap, ntstamps, tbytes, err
}

// Entry point for taking the request and writing to local temp files followed by sending them
// to the next stage for further processing.
func (bi *BridgerInfo) pushInstance(ins *zstypes.InsCacheInfo) (uint64, uint64, error) {

	meta := dtypes.StFileMeta{
		Account:      ins.AName,
		Filetype:     "prometheus",
		Iid:          ins.IName,
		IsCompressed: true,
		DTs:          ins.DTs,
		OLabels:      ins.OLabels,
	}

	if config.GlCfg.BrdigerNotifyStIngest {
		for atomic.LoadInt32(&bi.stingestWorkerReady) == 0 {
			zlog.Info("Waiting for stingest req_q worker to be ready")
			time.Sleep(5 * time.Second)
		}
	}

	dmepoch := time.Now().UnixNano() / 1000
	dts := strconv.FormatInt(dmepoch, 10)
	tmpDir := path.Join(config.GlCfg.StatsReqsDir, ins.AName, strings.Replace(ins.IName, ":", "_", 1), dts)
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		zlog.Error("Failed to create directory %s: %s", tmpDir, err)
		return uint64(0), uint64(0), err
	}
	tmpMFpath := filepath.Join(tmpDir, dts+".json")
	tmpDFpath := filepath.Join(tmpDir, dts+".data.gz")

	clblsMap, ntstamps, nbytes, err := bi.writeStatsFile(tmpDFpath, ins)
	if err != nil {
		zlog.Error("Failed to write to file %s, err %s\n", tmpDFpath, err)
		return ntstamps, nbytes, err
	}

	if len(clblsMap) > 0 {
		meta.Kvs = make([]dtypes.KvPair, 0, len(clblsMap))
		for k, v := range clblsMap {
			meta.Kvs = append(meta.Kvs, dtypes.KvPair{N: k, V: v})
		}
	}
	mjson, err := json.Marshal(meta)
	if err != nil {
		zlog.Error("Failed to marshal file metadata %v: %s", meta, err)
		return ntstamps, nbytes, err
	}
	err = ioutil.WriteFile(tmpMFpath, mjson, 0644)
	if err != nil {
		zlog.Error("Failed to write to file %s, err %s\n", tmpMFpath, err)
		return ntstamps, nbytes, err
	}

	// If configured, send to the next stage for further processing.
	if config.GlCfg.BrdigerNotifyStIngest {
		req := stingestpb.StatsIngestRequest{
			Account:    ins.AName,
			DFpath:     tmpDFpath,
			MFpath:     tmpMFpath,
			InstanceId: ins.IName,
			Type:       meta.Filetype,
			DTs:        ins.DTs,
		}

		sReq := &stiReq{tmpDir: tmpDir, req: &req}
		bi.stiReqCh <- sReq
	}

	return ntstamps, nbytes, nil
}

func init() {
	binfo.stiMQD = config.GlCfg.BrdigerStIngestsQD

	binfo.stiReqCh = make(chan *stiReq, 1)
	binfo.stiRespCh = make(chan error, binfo.stiMQD)

	binfo.exitCh = make(chan string, 0)

	binfo.stiRnq = list.New()

	binfo.mnamesMap = make(map[string]*MnameAdjusterInfo)
	binfo.mnamesMap["node_cpu_seconds_total"] = &MnameAdjusterInfo{mname: "node_cpu_seconds_total", ylabels: []string{"cpu", "mode"}, slabel: "mode", svalueMLen: 16, tfilter: "counter"}
	binfo.mnamesMap["node_cpu_guest_seconds_total"] = &MnameAdjusterInfo{mname: "node_cpu_guest_seconds_total", ylabels: []string{"cpu", "mode"}, slabel: "mode", svalueMLen: 16, tfilter: "counter"}
}

func (bi *BridgerInfo) startWorker() {
	if config.GlCfg.BrdigerNotifyStIngest {
		zlog.Info("Starting stingest req_q worker")
		bi.exitWaiter.Add(1)
		go bi.stIngestWorker()
	}
}

func (bi *BridgerInfo) stopWorker() {
	if config.GlCfg.BrdigerNotifyStIngest {
		zlog.Info("Stopping stingest req_q worker")

		bi.stopping = true
		bi.exitCh <- "exit"
		bi.exitWaiter.Wait()
		close(bi.exitCh)

		zlog.Info("Stopped stingest req_q worker")
	}
}
