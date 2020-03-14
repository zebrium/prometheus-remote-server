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

// Simple config reader from json file.

// NOTE: DO NOT SEND ANY OUPUT TO STDOUT. All the output SHOULD go to stderr, as some programs assume this from this package.

package zconfig

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//const (
//	CONST_config_filepath = "/zebrium/etc/config/cfg.json.txt"
//)

type ConfigInfo struct {
	cfgs      map[string]interface{} // All configs.
	is_inited bool                   // Is init'd.
	cfg_file  string
}

var gl ConfigInfo = ConfigInfo{is_inited: false, cfg_file: ""}

func ReadZConfig(file string) {
	if gl.is_inited {
		if gl.cfg_file != file {
			fmt.Fprintf(os.Stderr, "Config is already initialized with file %s, new file %s\n", gl.cfg_file, file)
			os.Exit(1)
			return
		}
		return
	}

	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open config file %s err %s\n", file, err)
		os.Exit(1)
		return
	}
	defer f.Close()

	jstr := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		str := scanner.Text()
		if cidx := strings.Index(str, "#"); cidx >= 0 {
			str = str[:cidx]
		}
		jstr = jstr + str
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s err %s\n", file, err)
		os.Exit(1)
		return
	}

	data := []byte(jstr)
	if err := json.Unmarshal(data, &gl.cfgs); err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse json data from file %s err %s\n", file, err)
		os.Exit(1)
		return
	}
	gl.cfg_file = file
	gl.is_inited = true
}

//func init() {
//	ReadZConfig(CONST_config_filepath)
//}

func GetZConfigInt(cfg_name string) int {
	if !gl.is_inited {
		fmt.Fprintf(os.Stderr, "Config is not initialized")
		os.Exit(1)
		return 0
	}

	return int(gl.cfgs[cfg_name].(float64))
}

func GetZConfigStr(cfg_name string) string {
	if !gl.is_inited {
		fmt.Fprintf(os.Stderr, "Config is not initialized")
		os.Exit(1)
		return ""
	}

	return gl.cfgs[cfg_name].(string)
}
