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
	"fmt"
	"hash/fnv"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"zebrium.com/stserver/common/zassert"
	"zebrium.com/stserver/common/zlog"
	"zebrium.com/stserver/prometheus/ztypes"
	"zebrium.com/stserver/stats/config"
	"zebrium.com/stserver/stats/zstypes"
)

const (
	RPR_periodic_wakeup_secs_normal     = 60
	RPR_periodic_wakeup_secs_try_hard   = 30
	RPR_periodic_wakeup_secs_try_harder = 5
	MAX_reaped_per_iter                 = 1000
)

// Cache of all instances across all accounts(customers).
type cacheMap struct {
	nr   int32                   // Number of entries in all buckets. Can be updated atomically.
	bkts []*zstypes.InsCacheInfo // Hash buckets (NOTE, NOTE: do not append/resize, once allocated)
	lk   sync.RWMutex            // RW lock protecting bkt chains.

	hwSecs        int       // High water mark age timeout seconds.
	lwSecs        int       // Low water mark age timeout seconds.
	hwNr          int       // High water mark, where reaper will be woken up to reap entries.
	lwNr          int       // Low wter mark.
	rprWakeupSecs int       // Reaper periodic wakeup seconds.
	rprRq         int32     // Do we have reaper request already in process?
	rprCh         chan bool // Reaper thread waits on this channel.

	exitCh     chan string    // Exit channel.
	stopRpr    bool           // Should we stop the reaper thread?
	exitWaiter sync.WaitGroup // Waitq for reaper to be stopped.
}

var glCache cacheMap

func init() {
	nbkts := config.GlCfg.ZVCacherCacheBuckets
	glCache.bkts = make([]*zstypes.InsCacheInfo, nbkts) // NOTE: DO NOT resize this, as we use naked pointers.
	glCache.nr = 0

	glCache.hwSecs = config.GlCfg.ZVCacherDropHWSecs
	glCache.lwSecs = config.GlCfg.ZVCacherDropLWSecs
	glCache.hwNr = config.GlCfg.ZVCacherEntriesHW
	glCache.lwNr = config.GlCfg.ZVCacherEntriesLW
	glCache.rprRq = 0
	glCache.rprWakeupSecs = RPR_periodic_wakeup_secs_normal

	glCache.rprCh = make(chan bool, 1)
	glCache.stopRpr = false
	glCache.exitCh = make(chan string, 1)
}

func startWorker() {
	zlog.Info("Starting zvcacher reaper thread")
	glCache.exitWaiter.Add(1)
	go reaper()
}

func stopWorker() {
	zlog.Info("Stopping zvcacher reaper thread")
	glCache.stopRpr = true
	glCache.exitCh <- "exit"
	close(glCache.exitCh)

	glCache.exitWaiter.Wait()
	zlog.Info("Stopped zvcacher reaper thread")
}

func time2msec(t time.Time) int64 {
	return t.Unix()*1000 + int64(t.Nanosecond())/int64(time.Millisecond)
}

// Hash of account and iname: instance is : job+instance of prometheus.
func (c *cacheMap) hash(account string, iname string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(account))
	h.Write([]byte("."))
	h.Write([]byte(iname))
	return h.Sum32() % uint32(len(c.bkts))
}

// Should be called with write lock held.
func (c *cacheMap) deleteOne(ie *zstypes.InsCacheInfo) {
	bi := c.hash(ie.AName, ie.IName)

	pie := &c.bkts[bi]
	cie := c.bkts[bi]

	for cie != nil {
		if cie == ie {
			*pie = ie.Next
			zassert.Zassert(atomic.LoadInt32(&c.nr) > 0, "Invalid ninuse %d in cacheMap", atomic.LoadInt32(&c.nr))
			atomic.AddInt32(&c.nr, -1)
			return
		}
		pie = &cie.Next
		cie = cie.Next
	}
	zassert.Zdie("ERROR: could not find instance in cache for account %s instance %s\n", ie.AName, ie.IName)
}

func (c *cacheMap) wakeupReaperIfNeeded() {
	if atomic.LoadInt32(&c.nr) > int32(c.hwNr) && atomic.LoadInt32(&c.rprRq) == int32(0) {
		if atomic.CompareAndSwapInt32(&c.rprRq, 0, 1) {
			zlog.Info("Waking up reaper nr in cache %d", atomic.LoadInt32(&c.nr))
			c.rprCh <- true
		}
	}
}

// Caller needs to have the read lock.
func (c *cacheMap) addCacheEntry(ie *zstypes.InsCacheInfo) {
	bi := c.hash(ie.AName, ie.IName)

	zlog.Debug("Adding cache entry: account %s instance %s to bucket %d\n", ie.AName, ie.IName, bi)

	ie.Next = c.bkts[bi]
	sptr := (*reflect.SliceHeader)(unsafe.Pointer(&c.bkts))
	barrPtr := (*[4294967295]*zstypes.InsCacheInfo)(unsafe.Pointer(sptr.Data))
	for !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&barrPtr[bi])), unsafe.Pointer(ie.Next), unsafe.Pointer(ie)) {
		ie.Next = c.bkts[bi]
	}
	atomic.AddInt32(&c.nr, 1)
	zlog.Debug("Done adding cache entry: account %s instance %s to bucket %d ninuse %d\n", ie.AName, ie.IName, bi, atomic.LoadInt32(&c.nr))
}

// Update the cache with the new blob, and get back the full blob.
// Every get needs to do a put.
func (c *cacheMap) updateAndGet(account string, wreq *ztypes.StWbIReq) (*zstypes.InsCacheInfo, error) {
	bid := c.hash(account, wreq.Instance)

	c.lk.RLock()
	var mie *zstypes.InsCacheInfo
	for ie := c.bkts[bid]; ie != nil; ie = ie.Next {
		if ie.AName == account && ie.IName == wreq.Instance {
			mie = ie
			zassert.Zassert(mie.LAts != 0, "Get is already active on instance %s account %s", account, ie.IName)
			mie.LAts = 0
			break
		}
	}
	c.lk.RUnlock()

	if wreq.IsIncr && (mie == nil || mie.Gen+1 != wreq.Gen || wreq.NSamples != len(mie.Samples)) {
		if mie != nil {
			zlog.Info("Need full blob: gens %d %d nsamples %d %d\n", mie.Gen, wreq.Gen, len(mie.Samples), wreq.NSamples)
			mie.LAts = time2msec(time.Now())
		}
		return nil, nil
	}

	if mie == nil {
		zassert.Zassert(wreq.IsIncr == false, "Not a full blob")
		mie = &zstypes.InsCacheInfo{AName: account, IName: wreq.Instance, Gen: wreq.Gen, LAts: 0, DTs: wreq.GTs, OLabels: wreq.OLabels}
		c.lk.RLock()
		c.addCacheEntry(mie)
		c.lk.RUnlock()
		c.wakeupReaperIfNeeded()
	}

	mie.DTs = wreq.GTs
	if wreq.IsIncr {
		zassert.Zassert(mie.Gen+1 == wreq.Gen && wreq.NSamples == len(mie.Samples), "Invalid state: gens %d %d nsamples %d %d\n", mie.Gen, wreq.Gen, len(mie.Samples), wreq.NSamples)
		mie.Gen = wreq.Gen
		cwidx := 0
		for i := 0; i < len(mie.Samples); i++ {
			if len(wreq.Samples) > cwidx && wreq.Samples[cwidx].Idx == int32(i) {
				mie.Samples[i].Ts = mie.Samples[i].Ts + wreq.Samples[cwidx].Ts
				mie.Samples[i].Val = mie.Samples[i].Val + wreq.Samples[cwidx].Val
				cwidx++
			} else {
				// No change in record, just update the time stamp.
				mie.Samples[i].Ts = wreq.GTs
			}
		}
		return mie, nil
	}

	// Full blob in the request.
	if wreq.NSamples != len(wreq.Samples) {
		err := fmt.Errorf("Invalid request sample count %d != %d", wreq.NSamples, len(wreq.Samples))
		zlog.Warn(err.Error())
		mie.LAts = time2msec(time.Now())
		return nil, err
	}
	zlog.Info("Received full blob for instance %s account %s samples %d\n", account, wreq.Instance, len(wreq.Samples))
	mie.Gen = wreq.Gen
	mie.OLabels = wreq.OLabels
	if wreq.NSamples != len(mie.Samples) {
		mie.Samples = make([]zstypes.InsSample, wreq.NSamples)
	}
	for i := 0; i < wreq.NSamples; i++ {
		wsi := &wreq.Samples[i]
		si := &mie.Samples[i]
		si.Ls = wsi.Ls
		si.Help = wsi.Help
		si.Type = wsi.Type
		si.Ts = wsi.Ts
		si.Val = wsi.Val
	}
	return mie, nil
}

// Releases the reference obtained from get.
func (c *cacheMap) put(ins *zstypes.InsCacheInfo) error {
	ins.LAts = time2msec(time.Now())
	return nil
}

// Heap to age out inactive entries from cache because of memory pressure.
type rprHeap struct {
	ces  []*zstypes.InsCacheInfo // Cache entries slice.
	ts   []int64                 // Copied Time stamps from InsCacheInfo
	nr   int                     // Number valid in the above slice.
	nmax int                     // Max allowed elements in the ces slice.
}

func (h *rprHeap) Len() int           { return h.nr }
func (h *rprHeap) Less(i, j int) bool { return h.ts[i] > h.ts[j] } // Highest ts at the root.
func (h *rprHeap) Swap(i, j int) {
	h.ces[i], h.ces[j] = h.ces[j], h.ces[i]
	h.ts[i], h.ts[j] = h.ts[j], h.ts[i]
}
func (h *rprHeap) isFull() bool { return h.nr == h.nmax }

func (h *rprHeap) Push(x interface{}) {
	zassert.Zassert(h.nr < h.nmax, "Number of reaper heap %d >= max %d", h.nr, h.nmax)
	h.ces[h.nr] = x.(*zstypes.InsCacheInfo)
	h.ts[h.nr] = x.(*zstypes.InsCacheInfo).LAts
	h.nr++
}

func (h *rprHeap) Pop() interface{} {
	zassert.Zassert(h.nr > 0, "Invalid number in reaper heap %d", h.nr)
	rtn := h.ces[h.nr]
	h.ces[h.nr] = nil
	h.ts[h.nr] = 0
	h.nr--
	return rtn
}

// Actual work that gets done to age out old entries from cache.
func doReap() {
	nrToReap := int(atomic.LoadInt32(&glCache.nr)) - glCache.lwNr
	if nrToReap <= 0 {
		return
	}
	if nrToReap > MAX_reaped_per_iter {
		nrToReap = MAX_reaped_per_iter
	}

	zlog.Info("reaper work started: nr_to_reap %d total_nr %d lw %d hw %d lwsecs %d hwsecs %d\n",
		nrToReap, atomic.LoadInt32(&glCache.nr),
		glCache.lwNr, glCache.hwNr, glCache.lwSecs, glCache.hwSecs)

	mtime := int64(glCache.lwSecs)
	if atomic.LoadInt32(&glCache.nr) >= int32(glCache.hwNr) {
		mtime = int64(glCache.hwSecs)
	}
	mtime = time2msec(time.Now().Add(-1 * time.Second * time.Duration(mtime)))

	rhp := &rprHeap{ces: make([]*zstypes.InsCacheInfo, nrToReap), ts: make([]int64, nrToReap), nr: 0, nmax: nrToReap}

	glCache.lk.RLock()

	for i := 0; i < len(glCache.bkts); i++ {
		if glCache.bkts[i] == nil {
			continue
		}

		for ie := glCache.bkts[i]; ie != nil; ie = ie.Next {
			if ie.LAts < mtime && ie.LAts != 0 {
				rhp.Push(ie)
				if rhp.isFull() {
					break
				}
			}
		}

		if rhp.isFull() {
			break
		}
	}
	glCache.lk.RUnlock()

	zlog.Info("Reaper collected %d entries to reap, total %d\n", rhp.nr, atomic.LoadInt32(&glCache.nr))
	nreaped := 0
	if rhp.nr != 0 {
		glCache.lk.Lock()
		for i := 0; i < rhp.nr; i++ {
			if rhp.ces[i].LAts != 0 && rhp.ces[i].LAts == rhp.ts[i] {
				glCache.deleteOne(rhp.ces[i])
				nreaped++
			}
		}
		glCache.lk.Unlock()
	}
	zlog.Info("Reaper reaped %d instances, collected %d total now %d\n", nreaped, rhp.nr, atomic.LoadInt32(&glCache.nr))

	if atomic.LoadInt32(&glCache.nr) >= int32(glCache.hwNr) {
		glCache.rprWakeupSecs = RPR_periodic_wakeup_secs_try_harder
	} else if atomic.LoadInt32(&glCache.nr) >= int32(glCache.lwNr) {
		glCache.rprWakeupSecs = RPR_periodic_wakeup_secs_try_hard
	} else {
		glCache.rprWakeupSecs = RPR_periodic_wakeup_secs_normal
	}
	atomic.StoreInt32(&glCache.rprRq, 0)
}

// Reaper thread, that takes care of aging out old entries from cache.
func reaper() {
	for {
		select {
		case _ = <-glCache.rprCh:
			if !glCache.stopRpr {
				doReap()
			}
		case <-time.After(time.Duration(glCache.rprWakeupSecs) * time.Second):
			doReap()
		case _ = <-glCache.exitCh:
			zlog.Info("Reaper thread exiting\n")
			glCache.exitWaiter.Done()
			return
		}
	}
}
