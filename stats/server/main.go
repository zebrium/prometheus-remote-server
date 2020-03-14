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

package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"sync/atomic"
	"syscall"

	"zebrium.com/stserver/common/zlog"
	"zebrium.com/stserver/common/zmodule"

	"zebrium.com/stserver/prometheus/ztypes"

	"zebrium.com/stserver/stats/config"
	"zebrium.com/stserver/stats/modules"
	"zebrium.com/stserver/stats/worker"
)

const (
	statsServerPidFile = "/zebrium/var/run/stats.%s.pid"
)

// These are populated by the build script
var gbuild_changeset string
var gbuild_branch string
var gbuild_date string

func init() {
	zlog.Info("=================================================")
	zlog.Info("stats_svc_bin starting %s %s %s", gbuild_changeset, gbuild_branch, gbuild_date)

	modules.RegisterModules()
	zmodule.Init()
	config.Dump()
}

func sendError(w http.ResponseWriter, ver int8, rErr error) {
	errResp := ztypes.StWbResp{
		Version:     ver,
		MinPollSecs: 0,
		Err:         rErr.Error(),
		NInsts:      0,
		IErrCodes:   make([]int8, 0),
	}

	enc := gob.NewEncoder(w)
	err := enc.Encode(errResp)
	if err != nil {
		zlog.Error("Failed to encode response: %s", err)
		return
	}
}

type WriterWithoutTLSErrors struct {
	re *regexp.Regexp
	wr io.Writer
}

func (w *WriterWithoutTLSErrors) Write(p []byte) (n int, err error) {
	if w.re.Match(p) {
		return len(p), nil
	}
	return w.wr.Write(p)
}

type ReaderWithBytes struct {
	rdr    io.Reader
	nbytes int
}

func (r *ReaderWithBytes) Read(p []byte) (n int, err error) {
	n, err = r.rdr.Read(p)
	r.nbytes += n
	return n, err
}

var glReqCount int32
var glInflight int32

// Handle the stats HTTP request.
// Get : typically gets called by load balancer or the front end apache/nginx for keep alive.
// POST: This is how, stats collector sends the requests.
func handleStats(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/zstats" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	switch r.Method {
	case "GET":
		fmt.Fprintf(w, "OK\n")
		//http.Error(w, "204 GET not supported", http.StatusNoContent)
		return
	case "POST":
		token := r.Header.Get("Zebrium-Token")
		contentType := r.Header.Get("Content-type")

		zlog.Debug("Token: %s, Content-type: %s", token, contentType)

		if atomic.LoadInt32(&glInflight) > int32(config.GlCfg.StatsReqsMax) {
			err := fmt.Errorf("Too many pending requests (%d)", glInflight)
			zlog.Error("%s", err)
			sendError(w, 0, err)
			return
		}
		atomic.AddInt32(&glInflight, 1)
		rid := atomic.AddInt32(&glReqCount, 1)
		defer atomic.AddInt32(&glInflight, -1)

		if len(token) == 0 {
			err := fmt.Errorf("Request has empty token")
			zlog.Error("%s", err)
			sendError(w, 0, err)
			return
		}

		if contentType != "application/octet-stream" {
			err := fmt.Errorf("Request has invalid content-type (%s)", contentType)
			zlog.Error("%s", err)
			sendError(w, 0, err)
			return
		}

		// Authenticate the token.
		account, spollsecs, err := worker.GetWorkerAPI().CheckToken(token)
		if err != nil {
			zlog.Error("%s", err)
			sendError(w, 0, err)
			return
		}

		var cRdr = &ReaderWithBytes{rdr: r.Body}
		rdr, err := gzip.NewReader(cRdr)
		if err != nil {
			zlog.Error("Invalid request, gzip reader failed, err %s", err)
			sendError(w, 0, err)
			return
		}
		defer rdr.Close()
		var bRdr = &ReaderWithBytes{rdr: rdr}
		var dec = gob.NewDecoder(bRdr)
		var req ztypes.StWbReq
		err = dec.Decode(&req)
		if err != nil {
			zlog.Error("Invalid request, decode failed, err %s", err)
			sendError(w, 0, err)
			return
		}

		// Work on this request.
		errCodes, err := worker.GetWorkerAPI().ProcessRequest(rid, account, token, uint64(bRdr.nbytes), uint64(cRdr.nbytes), req)

		sErr := ""
		if err != nil {
			sErr = err.Error()
		}

		// Prepare and send the response.
		resp := ztypes.StWbResp{
			Version:     req.Version,
			MinPollSecs: int8(spollsecs),
			Err:         sErr,
			NInsts:      int32(len(errCodes)),
			IErrCodes:   errCodes,
		}

		enc := gob.NewEncoder(w)
		err = enc.Encode(resp)
		if err != nil {
			zlog.Error("Failed to encode response: %s", err)
			return
		}

	default:
		fmt.Fprintf(w, "Sorry, only GET and POST methods are supported.")
	}
}

func main() {
	portp := flag.String("port", "", "Port to connect to stats server on")
	flag.Parse()
	port := *portp
	if len(port) == 0 {
		port = strconv.Itoa(config.GlCfg.StatsHTTPPort)
	}
	pidFile := fmt.Sprintf(statsServerPidFile, port)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	zlog.Info("Starting stats server service\n")

	defer func() {
		if _, err := os.Stat(pidFile); err == nil { // Attempt cleanup
			os.Remove(pidFile)
		}
	}()
	if !config.GlCfg.StatsEnabled {
		zlog.Info("Stats server disabled in config, exiting\n")
		return
	}

	var pemFile string
	var err error
	if config.GlCfg.StatsHTTPSEnabled {
		pemFile = os.Getenv("PEMFILE")
		_, err = os.Stat(pemFile)
		if err != nil {
			zlog.Error("Could not access pem file %s: %s", pemFile, err)
			pemFile = ""
		}
	}

	zmodule.Start()

	go func(port string) {
		// Avoid logging TLS handshake errors which result from port scans
		var w = WriterWithoutTLSErrors{wr: os.Stderr, re: regexp.MustCompile(`TLS handshake error from 127\.0\.0\.1.*EOF\n$`)}
		log.SetOutput(&w)
		http.HandleFunc("/", handleStats)

		if len(pemFile) != 0 {
			zlog.Info("Starting HTTPS server on :%s (cert:%s)", port, pemFile)
			err = http.ListenAndServeTLS(":"+port, pemFile, pemFile, nil)
		} else {
			zlog.Info("Starting HTTP server on :%s", port)
			err = http.ListenAndServe(":"+port, nil)
		}
		if err != nil {
			zlog.Fatal("%s\n", err)
		}
	}(port)

	<-sigs

	zmodule.Stop()
	zmodule.Fini()
}
