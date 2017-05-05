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

package plugin_utils

// This file holds utility/helper methods required in plugin module

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
)

const (
	linuxMountsFile = "/proc/mounts"
)

// GetMountInfo - return a map of mounted volumes and devices
func GetMountInfo(mountRoot string) (map[string]string, error) {
	volumeMap := make(map[string]string)
	data, err := ioutil.ReadFile(linuxMountsFile)

	if err != nil {
		log.Errorf("Can't get info from %s (%v)", linuxMountsFile, err)
		return volumeMap, err
	}

	for _, line := range strings.Split(string(data), "\n") {
		field := strings.Fields(line)
		if len(field) < 2 {
			continue // skip empty line and lines too short to have our mount
		}
		// fields format: [/dev/sdb /mnt/vmdk/vol1 ext2 rw,relatime 0 0]
		if filepath.Dir(field[1]) != mountRoot {
			continue
		}
		volumeMap[filepath.Base(field[1])] = field[0]
	}
	return volumeMap, nil
}

// CheckAlreadyMounted - check if volume is already mounted on the mountRoot
func CheckAlreadyMounted(name string, mountRoot string) bool {
	volumeMap, err := GetMountInfo(mountRoot)

	if err != nil {
		return false
	}

	if _, ok := volumeMap[name]; ok {
		return true
	}
	return false
}

// GetDatastore - get datastore from volume metadata
// Note "datastore" key is defined in vmdkops service
func GetDatastore(name string, volumeMeta map[string]interface{}) string {
	datastore, _ := volumeMeta["datastore"].(string)
	return datastore
}

// GetFullVolumeName - append datastore to the volume name
func GetFullVolumeName(name string, datastore string) string {
	s := []string{name, datastore}
	return strings.Join(s, "@")
}

// IsFullVolumeName - return if name is full volume name i.e. volume@datastore
func IsFullVolumeName(name string) bool {
	return strings.ContainsAny(name, "@")
}
