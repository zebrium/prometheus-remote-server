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

import (
	"sync"
	"time"

	"container/list"

	"zebrium.com/stserver/common/zassert"
	"zebrium.com/stserver/common/zlog"
	"zebrium.com/stserver/common/zdummydb"

	"zebrium.com/stserver/prometheus/ztypes"

	"zebrium.com/stserver/stats/bridger"
	"zebrium.com/stserver/stats/config"
	"zebrium.com/stserver/stats/zvcacher"
)

const (
	WorkerStatsDumpSeconds = 60
)

// All the scraped blobs from a single HTTP request is represented
// with this structure.
type InstReq struct {
	account   string      // Account information. (account is like customer name)
	instances []string    // set of scraped blobs.
	respCh    chan string // Channel, where we get response.
}

// Stats.
type WorkerStat struct {
	fbytes    uint64    // Actual bytes, assuming all are full blobs (uncompressed).
	nbytes    uint64    // uncompressed blobs size that we got over wire.
	cbytes    uint64    // Compressed blobs size that we got over wire.
	nreqs     uint64    // Number of requests.
	nfull     uint64    // Number of full blobs.
	nincr     uint64    // Number of incremental blobs.
	nfsamples uint64    // Number of stats samples from full blobs.
	nisamples uint64    // Number of stats samples from incremental blobs.
	nlabels   uint64    // Number of labels.
	ntstamps  uint64    // Number of samples with custom time stamps.
	nmisses   uint64    // Number of misses in the cacher.
	nerrs     uint64    // Number of errors.
	ts        time.Time // Lat updated time.
}

// Response info.
type InstReqResp struct {
	req *ztypes.StWbIReq // Request.
	res *int8            // Response code.
}

// Worker info.
type WorkerInfo struct {
	getCh      chan *InstReq  // Channel, where the requests get serialized.
	putCh      chan *InstReq  // Channel to release the serializer lock.
	exitCh     chan string    // Exit channel.
	exitWaiter sync.WaitGroup // Exit wait queue.
	stopping   bool           // Are we stopping?
	alldone    bool           // Is everything done, when stopping?

	statsLock sync.Mutex             // Lock for updating stats.
	stats     map[string]*WorkerStat // In memory Statastics.
	statsTs   time.Time              // Last updated time of in memory Statastics.

	maxQD int             // Max serializer queue depth.
	curQD int             // Current queue depth.
	amap  map[string]bool // Active serializers.
	waitq *list.List      // Wait queue.
}

var winfo WorkerInfo

func getWorkerInfo() *WorkerInfo {
	return &winfo
}

func init() {
	winfo.getCh = make(chan *InstReq, 1)
	winfo.putCh = make(chan *InstReq, 1)
	winfo.exitCh = make(chan string, 0)

	winfo.stats = make(map[string]*WorkerStat)
	winfo.curQD = 0
	winfo.maxQD = config.GlCfg.StatsSerializerQD
	winfo.amap = make(map[string]bool)
	winfo.waitq = list.New()
}

// Works on one target's scraped data feom a single http request.
func (wi *WorkerInfo) doProcessRequests(account string, irrs []*InstReqResp, wgp *sync.WaitGroup) {
	defer wgp.Done()

	zlog.Info("Number of requests %d for instance %s", len(irrs), irrs[0].req.Instance)
	for _, irr := range irrs {
		*irr.res = 1 // Resend the full blob.
		// Once we see a failure for one, we would not try the rest, and ask them to send full blobs.
	}

	// For each scraped blob.
	for _, irr := range irrs {
		ireq := irr.req

		// Get the deflated full blob.
		ins, err := zvcacher.GetZVCacher().UpdateAndGet(account, ireq)
		var fail = false
		var miss = false
		if err != nil {
			fail = true
		}
		if !fail && ins == nil {
			miss = true
		}
		wi.updateStatsIReq(account, ireq, miss, fail)
		if err != nil {
			zlog.Error("Failed to look up %s: %s", ireq.Instance, err)
			*irr.res = 2
			return
		}
		if ins == nil {
			*irr.res = 1 // Resend the full blob
			return
		}

		// Now that we have full blob, give it to bridger module for further processing.
		ntstamps, nbytes, err := bridger.GetBridgerAPI().Push(ins)
		wi.updateStatsTstamps(account, ntstamps, nbytes)
		if err != nil {
			zlog.Error("Failed to write instance data for %s: %s", ins.IName, err)
			*irr.res = 2
		} else {
			*irr.res = 0
		}

		zvcacher.GetZVCacher().Put(ins)
	}

}

func (wi *WorkerInfo) updateStatsReq(account string, nbytes uint64, cbytes uint64, nlabels uint64) {
	wi.statsLock.Lock()
	astats, ok := wi.stats[account]
	if !ok {
		astats = &WorkerStat{}
		wi.stats[account] = astats
	}
	astats.nbytes += nbytes
	astats.cbytes += cbytes
	astats.nlabels += nlabels
	astats.nreqs++
	astats.ts = time.Now()
	wi.statsLock.Unlock()
}

func (wi *WorkerInfo) updateStatsTstamps(account string, ntstamps uint64, nbytes uint64) {
	wi.statsLock.Lock()
	astats, ok := wi.stats[account]
	zassert.Zassert(ok, "missing stats for account %s", account)
	astats.ntstamps += ntstamps
	astats.fbytes += nbytes
	wi.statsLock.Unlock()
}

func (wi *WorkerInfo) updateStatsIReq(account string, ireq *ztypes.StWbIReq, miss bool, fail bool) {
	wi.statsLock.Lock()
	astats, ok := wi.stats[account]
	if !ok {
		astats = &WorkerStat{}
		wi.stats[account] = astats
	}
	if ireq.IsIncr {
		astats.nisamples += uint64(ireq.NSamples)
		astats.nincr++
	} else {
		astats.nfsamples += uint64(ireq.NSamples)
		astats.nfull++
	}
	if miss {
		astats.nmisses++
	}
	if fail {
		astats.nerrs++
	}
	astats.ts = time.Now()
	wi.statsLock.Unlock()
}

func (wi *WorkerInfo) dumpStats() {
	wi.statsLock.Lock()
	for account, astats := range wi.stats {
		if astats.ts.Before(wi.statsTs) {
			continue
		}
		// stats updated since the last dump
		compr := uint64(0)
		if astats.cbytes > 0 {
			compr = astats.fbytes / astats.cbytes
		}
		zlog.Info("Account summary stats for %s, reqs=%d fbytes=%d bytes=%d cbytes=%d compr=%d tstamps=%d misses=%d errs=%d full_blobs=%d samples=%d labels=%d incr_blobs=%d samples=%d\n",
			account, astats.nreqs, astats.fbytes, astats.nbytes, astats.cbytes, compr, astats.ntstamps, astats.nmisses,
			astats.nerrs, astats.nfull, astats.nfsamples, astats.nlabels, astats.nincr, astats.nisamples)
	}
	wi.statsTs = time.Now()
	wi.statsLock.Unlock()
}

func (wi *WorkerInfo) dumpStatsWorker() {
	zlog.Info("Starting stats dump worker (interval=%d seconds)", WorkerStatsDumpSeconds)
	for {
		select {
		case <-time.After(WorkerStatsDumpSeconds * time.Second):
			wi.dumpStats()
		}
	}
}

// Entry point of the HTTP POST request.
// Each request can have set of blobs.
func (wi *WorkerInfo) processRequest(rid int32, account string, token string, nbytes uint64, cbytes uint64, req ztypes.StWbReq) ([]int8, error) {

	zlog.Info("Got request id=%d account=%s Vvar=%d size=%d instances=%d", rid, account, req.Version, cbytes, req.NInsts)
	resp := make([]int8, len(req.IData))
	if len(req.IData) == 0 {
		// This is the health check request from scrapper, report ok.
		return resp, nil
	}

	var nlabels = uint64(0)
	for _, id := range req.IData {
		zlog.Debug("  Instance=%s NSamples=%d IsIncr=%v Gen=%d GTs=%d Samples=%d", id.Instance, id.NSamples, id.IsIncr, id.Gen, id.GTs, len(id.Samples))
	}

	sreq := &InstReq{
		account:   account,
		instances: make([]string, len(req.IData)),
		respCh:    make(chan string, 1),
	}
	for i, id := range req.IData {
		sreq.instances[i] = id.Instance
	}

	// Serialize multiple requests for the same scrape target, if there is any.
	// Also limit the number of HTTP requests, that we can simulataniously handle (max queue depth coming from config).
	wi.getCh <- sreq
	<-sreq.respCh

	imap := make(map[string][]*InstReqResp)
	for i, id := range req.IData {
		irr := &InstReqResp{req: id, res: &resp[i]}
		imap[id.Instance] = append(imap[id.Instance], irr)
	}

	var wg sync.WaitGroup
	wg.Add(len(imap))

	// Process all the targets in this request.
	for _, irrs := range imap {
		go wi.doProcessRequests(account, irrs, &wg)
	}
	wg.Wait()
	wi.updateStatsReq(account, nbytes, cbytes, nlabels)

	// Release the serializer.
	wi.putCh <- sreq
	<-sreq.respCh

	zlog.Info("Done with request id=%d account=%s", rid, account)
	return resp, nil
}

func (wi *WorkerInfo) lookupToken(token string) (string, int, error) {
	cred, err := zdummydb.GetTokenCred(token)
	if err != nil {
		// DO not log error here...
		return "", 0, err
	}

	return cred.Account, cred.SpollSecs, nil
}

func (wi *WorkerInfo) uniqifyInstance(account string, ins string) string {
	return "^zaccount:" + account + "$" + ins
}

// Check to see, if we cannot issue this request, because of some other
// active request on the same scraped target.
func (wi *WorkerInfo) getSerializerForReq(req *InstReq) bool {

	isok := true
	for _, inst := range req.instances { // Make sure no instances are active
		_, isactive := wi.amap[wi.uniqifyInstance(req.account, inst)]
		if isactive {
			isok = false
			break
		}
	}

	if isok {
		for _, inst := range req.instances {
			wi.amap[wi.uniqifyInstance(req.account, inst)] = true
		}
	}

	return isok
}

// Main state machine for the serializer.
func (wi *WorkerInfo) mainStateMachine() {
	// Are we stopping?
	if wi.stopping && wi.curQD == 0 {
		wi.alldone = true
		return
	}

	// If the conccurrancy allows, work on the next request.
	for wi.waitq.Len() > 0 && wi.curQD < wi.maxQD {
		e := wi.waitq.Front()
		req := e.Value.(*InstReq)

		isok := wi.getSerializerForReq(req)
		if !isok {
			break
		}

		wi.waitq.Remove(e)
		wi.curQD++
		req.respCh <- "ready"
	}

	if wi.waitq.Len() > 0 || wi.curQD > 0 {
		zlog.Info("SERIALIZER-STATS: serializer rnq %d active %d\n", wi.waitq.Len(), wi.curQD)
	}
}

func (wi *WorkerInfo) processPut(req *InstReq) {
	for _, ins := range req.instances {
		_, ok := wi.amap[wi.uniqifyInstance(req.account, ins)]
		zassert.Zassert(ok, "Instance %s missing from instance map", ins)
	}
	zassert.Zassert(wi.curQD > 0, "Invalid curqd %d", wi.curQD)
	wi.curQD--
	for _, ins := range req.instances {
		// We can see duplicate instance requests in one POST.
		delete(wi.amap, wi.uniqifyInstance(req.account, ins))
	}
	req.respCh <- "done"
	zlog.Info("SERIALIZER-STATS: serializer rnq %d active %d\n", wi.waitq.Len(), wi.curQD)
}

// Serializer main thread.
func (wi *WorkerInfo) serializerWorker() {

	for !wi.alldone {
		select {
		case req := <-wi.getCh:
			wi.waitq.PushBack(req)
			wi.mainStateMachine()
		case preq := <-wi.putCh:
			wi.processPut(preq)
			wi.mainStateMachine()
		case <-wi.exitCh:
			wi.mainStateMachine()
		}
	}

	zlog.Info("Worker serializer thread exited")
	wi.exitWaiter.Done()
}

func (wi *WorkerInfo) startWorker() {
	zlog.Info("Starting worker serializer")
	wi.exitWaiter.Add(1)
	go wi.serializerWorker()
	go wi.dumpStatsWorker()
}

func (wi *WorkerInfo) stopWorker() {
	zlog.Info("Stopping worker serializer")

	wi.stopping = true
	wi.exitCh <- "exit"
	wi.exitWaiter.Wait()
	close(wi.exitCh)

	zlog.Info("Stopped worker serializer")
}
