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
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/drivers"
)

const (
	// consts for finding and parsing linux mount information
	linuxMountsFile = "/proc/mounts"
)

// GetMountInfo - return a map of mounted volumes and devices
func GetMountInfo(mountRoot string) (map[string]string, error) {
	volumeMountMap := make(map[string]string) //map [volume mount path] -> device
	data, err := ioutil.ReadFile(linuxMountsFile)

	if err != nil {
		log.Errorf("Can't get info from %s (%v)", linuxMountsFile, err)
		return volumeMountMap, err
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
		volumeMountMap[filepath.Base(field[1])] = field[0]
	}
	return volumeMountMap, nil
}

// AlreadyMounted - check if volume is already mounted on the mountRoot
func AlreadyMounted(name string, mountRoot string) bool {
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
func GetDatastore(name string, d drivers.VolumeDriver) (string, map[string]interface{}, error) {
	volumeMeta, err := d.GetVolume(name)
	if err != nil {
		log.Errorf("Unable to get volume metadata %s (err: %v)", name, err)
		return "", nil, err
	}
	return volumeMeta["datastore"].(string), volumeMeta, nil
}

// GetNameFromRefmap - get names from refmap
func GetNameFromRefmap(volName string, d drivers.VolumeDriver) string {
	volumeNameList := d.VolumesInRefMap()

	count := 0
	fullname := ""

	for _, name := range volumeNameList {
		refVolName := strings.Split(name, "@")[0]
		if refVolName != volName {
			continue
		}
		// if there are collisions, return
		if count > 0 {
			return ""
		}
		count++
		fullname = name
	}
	return fullname
}

// GetFullNameAndMeta - return a qualified volume and metadata(if a get trip was made)
func GetFullNameAndMeta(name string, datastoreName string, d drivers.VolumeDriver) (string, map[string]interface{}, error) {
	if strings.ContainsAny(name, "@") {
		return name, nil, nil
	}
	if datastoreName != "" {
		return strings.Join([]string{name, datastoreName}, "@"), nil, nil
	}

	// find full volume names using refmap if possible
	if fullVolumeName := GetNameFromRefmap(name, d); fullVolumeName != "" {
		return fullVolumeName, nil, nil
	}

	datastoreName, volumeMeta, err := GetDatastore(name, d)
	if err != nil {
		return "", nil, err
	}
	return strings.Join([]string{name, datastoreName}, "@"), volumeMeta, nil
}
