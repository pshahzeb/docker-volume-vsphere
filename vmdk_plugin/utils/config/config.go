// Copyright 2016 VMware, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

// Read the plugin configuration file. The file is stored in JSON.
// See default-config.json at the root of the project.

import (
	"encoding/json"
	"io/ioutil"
)

const (
	// Default paths - used in log init in main() and test:

	// DefaultConfigPath is the default location of Log configuration file
	DefaultConfigPath = "/etc/docker-volume-vsphere.conf"
	// DefaultLogPath is the default location of log (trace) file
	DefaultLogPath = "/var/log/docker-volume-vsphere.log"

	// Local constants
	defaultMaxLogSizeMb  = 100
	defaultMaxLogAgeDays = 28
	defaultLogLevel      = "info"
)

// RemoteDir describes a network shared folder.
type RemoteDir struct {
	Addr string `json:",omitempty"`
	Args string `json:",omitempty"`
	Path string `json:",omitempty"`
	Fstype string `json:",omitempty"`
	VolPath string `json:",omitempty"`
	Src string `json:",omitempty"`
}

// RemoteDirList - table of remote dirs and the default table entry 
// to use to place volumes.
type RemoteDirList struct {
	Default string `json:",omitempty"`
	RemoteDirTbl map[string]RemoteDir `json:",omitempty"`

// Config stores the configuration for the plugin
type Config struct {
	Driver        string `json:",omitempty"`
	LogPath       string `json:",omitempty"`
	MaxLogSizeMb  int    `json:",omitempty"`
	MaxLogAgeDays int    `json:",omitempty"`
	LogLevel      string `json:",omitempty"`
	Target        string `json:",omitempty"`
	Project       string `json:",omitempty"`
	Host          string `json:",omitempty"`
	RemoteDirs    RemoteDirList `json:",omitempty"`
}

// Load the configuration from a file and return a Config.
func Load(path string) (Config, error) {
	jsonBlob, err := ioutil.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(jsonBlob, &config); err != nil {
		return Config{}, err
	}
	SetDefaults(&config)
	return config, nil
}

// SetDefaults for any config setting that is at its `bottom`
func SetDefaults(config *Config) {
	if config.LogPath == "" {
		config.LogPath = DefaultLogPath
	}
	if config.MaxLogSizeMb == 0 {
		config.MaxLogSizeMb = defaultMaxLogSizeMb
	}
	if config.MaxLogAgeDays == 0 {
		config.MaxLogAgeDays = defaultMaxLogAgeDays
	}
	if config.LogLevel == "" {
		config.LogLevel = defaultLogLevel
	}
}
