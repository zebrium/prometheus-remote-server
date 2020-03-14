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

package zutils

// This module export module interface to all other module,

// NOTE: DO NOT SEND ANY OUPUT TO STDOUT. All the output SHOULD go to stderr, as some programs assume this from this package.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"zebrium.com/stserver/common/zassert"
	"zebrium.com/stserver/common/zlog"
	"github.com/cespare/xxhash"
)

const (
	curlCmd = "/usr/bin/curl"
	pigzCmd = "/usr/bin/pigz"
	awsCmd  = "/usr/local/bin/aws"

	DefaultSlackURL      = "http://localhost:5800/api/v1/notify"
	DefaultDownloaderURL = "http://localhost:5800/api/v1/upload"
)

func DeleteFile(fpath string) error {
	if err := os.Remove(fpath); err != nil {
		fmt.Fprintf(os.Stderr, "Could not remove file %s err %s\n", fpath, err)
		return err
	} else {
		return nil
	}
}

func DeleteFileIfExists(fpath string) error {
	var err error
	if _, err = os.Stat(fpath); err == nil {
		return DeleteFile(fpath)
	} else if os.IsNotExist(err) {
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "Could not stat file %s err %s\n", fpath, err)
		return err
	}
}

func DeleteDir(dirpath string) error {
	if _, err := os.Stat(dirpath); os.IsNotExist(err) {
		return nil
	}
	if err := os.RemoveAll(dirpath); err != nil {
		fmt.Fprintf(os.Stderr, "Could not remove dir %s err %s\n", dirpath, err)
		return err
	} else {
		return nil
	}
}

func CreateDir(dirpath string) error {
	if err := os.MkdirAll(dirpath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create directory %s, err %s\n", dirpath, err)
		return err
	}
	return nil
}

func ReuseDir(dirpath string) error {
	if err := DeleteDir(dirpath); err != nil {
		return err
	}
	return CreateDir(dirpath)
}

func IsTmpFS(mpath string) bool {
	fh, err := os.Open("/proc/mounts")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open /proc/mounts, err %s\n", err)
		return false
	}
	defer fh.Close()

	reader := bufio.NewReader(fh)
	for {
		bread, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintf(os.Stderr, "Cannot find %s in /proc/mounts \n", mpath)
				break
			}
			fmt.Fprintf(os.Stderr, "Cannot read from /proc/mounts, err %s\n", err)
			return false
		}

		lwords := strings.Fields(string(bread))
		if len(lwords) > 3 && strings.HasPrefix(lwords[1], mpath) {
			if lwords[2] == "tmpfs" {
				fmt.Fprintf(os.Stderr, " %s is on tmpfs\n", mpath)
				return true
			}
			return false
		}
	}
	return false
}

func CopyFile(src, dst string) (err error) {
	sf, err := os.Open(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open file %s, err %s\n", src, err)
		return err
	}
	defer sf.Close()

	df, err := os.Create(dst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create file %s, err %s\n", dst, err)
		return err
	}
	defer df.Close()

	if _, err = io.Copy(df, sf); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot copy file %s to %s, err %s\n", src, dst, err)
		return err
	}

	err = df.Sync()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot sync file %s, err %s\n", dst, err)
		return err
	}

	return nil
}

func Hash64String(str string) uint64 {
	return xxhash.Sum64String(str)
}

func Hash64Byte(b []byte) uint64 {
	return xxhash.Sum64(b)
}

func ProcessIsAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	} else {
		err := process.Signal(syscall.Signal(0))
		return err == nil
	}
}

func ReadPidFile(pidFile string) (int, error) {
	b, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.Trim(string(b), "\n"))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func CheckPort(addr string) bool {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	if conn == nil {
		return false
	}
	conn.Close()
	return true
}

func GetIDN() string {
	val, ok := os.LookupEnv("ZE_IDN")
	if ok {
		return val
	}
	return ""
}

func SlackNotify(notifyUrl string, account string, msgtype string, msg string) error {
	type notification struct {
		Account string `json:"account"`
		MsgType string `json:"type"`
		Msg     string `json:"msg"`
	}
	var n = notification{
		Account: account,
		MsgType: msgtype,
		Msg:     msg,
	}
	out, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("Failed to marshal notification %v to json: %s", n, err)
	}

	var cmd = exec.Command(curlCmd, "--connect-timeout", "60", "--max-time", "120", "--data", string(out), "-X", "POST", notifyUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to send slack notification via downloader API: %s", err)
	}
	return nil
}

func SlackDirectNotify(host string, service string, account string, msgtype string, msg string) error {
	var slackUrl = os.Getenv("ZE_SLACK_DEBUG_WEBHOOK")
	var fullmsg = fmt.Sprintf("%s: Ze_Host=*%s/%s* Ze_Account=*%s* - %s", msgtype, host, service, account, msg)
	if len(slackUrl) == 0 {
		zlog.Warn("Skip sending %s slack message %s: no webhook", service, fullmsg)
		return nil
	}
	var curlOpts = []string{"--max-time", "10", "--connect-timeout", "10", "-X", "POST", "-H", "Content-type: application/json;charset=UTF-8", "--data", fmt.Sprintf("{\"text\":\"%s\"}", fullmsg), slackUrl}

	var cmd = exec.Command(curlCmd, curlOpts...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	var err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to send slack notification directly (%v): %s", cmd, err)
	}
	return nil
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func ArchiveBundle(s3Archive string, fpath string, failed bool, clean bool) {
	if len(s3Archive) == 0 {
		return
	}

	var _, err = os.Stat(fpath)
	if err != nil {
		zlog.Info("Skipping missing file %s", fpath)
		return
	}

	var gzFpath = fpath
	if path.Ext(fpath) != ".gz" && path.Ext(fpath) != ".tgz" {
		var gzFpath = fpath + ".gz"
		gz, err := os.Create(gzFpath)
		zassert.Zassert(err == nil, "Failed to create %s: %s", gzFpath, err)
		var cmd = exec.Command(pigzCmd, "-c", fpath)
		cmd.Stdout = gz
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		zassert.Zassert(err == nil, "Failed to compress %s: %s", fpath, err)
		err = gz.Close()
		zassert.Zassert(err == nil, "Failed to close %s: %s", gzFpath, err)
	}

	var outname = path.Base(gzFpath)
	if failed {
		outname = "failed_" + outname
	}
	var s3path = "s3://" + s3Archive + "/" + outname
	zlog.Info("Saving %s to S3 bucket %s", fpath, s3path)

	var cmd = exec.Command(awsCmd, "s3", "cp", gzFpath, s3path)
	// Don't capture stdout - it spews lots of garbage into the logs
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		zlog.Error("Failed to copy %s to S3: %s", gzFpath, err)
		return
	}
	if fpath != gzFpath {
		err = os.Remove(gzFpath)
		zassert.Zassert(err == nil, "Failed to remove %s: %s", gzFpath, err)
	}
	if clean {
		zlog.Info("Removing processed bundle at %s", fpath)
		err = os.Remove(fpath)
		zassert.Zassert(err == nil, "Failed to remove %s: %s", fpath, err)
	}
}
