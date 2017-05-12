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

package network

//
// File VolumeImpl Driver.
//
// Provide support for NFS based file backed volumes.
//

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	"path/filepath"
	"sync"
	"time"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/utils/config"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/utils/plugin_utils"
)

const (
)

// VolumeImplDriver - File backed volume drier meta-data
type VolumeImplDriver struct {
	config config.Config
}

// NewVolumeImplDriver creates Driver which to real ESX (useMockEsx=False) or a mock
func Init(mountDir string, config config.Config) (*VolumeImplDriver, error) {
	var d *VolumeImplDriver

	// Init all known backends - VMDK and network volume drivers
	d = new(VolumeImplDriver)
	d.config = config

	return d, nil
}

// Get info about a single volume
func (d *VolumeImplDriver) Get(r volume.Request) volume.Response {
	status, err := d.GetVolume(r.Name)
	if err != nil {
		return volume.Response{Err: err.Error()}
	}
	mountpoint := getMountPoint(r.Name)
	return volume.Response{Volume: &volume.Volume{Name: r.Name,
		Mountpoint: mountpoint,
		Status:     status}}
}

// List volumes known to the driver
func (d *VolumeImplDriver) List(r volume.Request) volume.Response {
	volumes, err := d.GetVolumeList()
	if err != nil {
		return volume.Response{Err: err.Error()}
	}
	responseVolumes := make([]*volume.Volume, 0, len(volumes))
	for _, vol := range volumes {
		mountpoint := d.GetMountPoint(vol.Name)
		responseVol := volume.Volume{Name: vol.Name, Mountpoint: mountpoint}
		responseVolumes = append(responseVolumes, &responseVol)
	}
	return volume.Response{Volumes: responseVolumes}
}

// GetVolume - return volume meta-data.
func (d *VolumeImplDriver) GetVolume(name string) (map[string]interface{}, error) {
}

// Create - create a volume.
func (d *VolumeImplDriver) Create(r volume.Request) volume.Response {
	// Check if the default label is to be used or pick the
	// specified label and mount that if not already mounted
	dslabel := plugin_utils.GetDSLabel(r.Name)

	if dslabel == "" {
		// Use the default label else error
		dslabel = d.config.RemoteDirs.Default
	}

	// Check if the folder for the named volume already exists
	// else create it in the vol-path specified for this label.
	if !isMounted(dsLabel) {
		// Mount the remote dir for this label
	}
	// Verify folder for the named volume doesn't exist and create it
}

// Remove - removes individual volume. Docker would call it only if is not using it anymore
func (d *VolumeImplDriver) Remove(r volume.Request) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Removing volume ")

	return volume.Response{Err: ""}
}

// Path - give docker a reminder of the volume mount path
func (d *VolumeImplDriver) Path(r volume.Request) volume.Response {
	return volume.Response{Mountpoint: getMountPoint(r.Name)}
}

// Mount - Provide a volume to docker container - called once per container start.
func (d *VolumeImplDriver) Mount(r volume.MountRequest) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Mounting volume ")

	return volume.Response{Mountpoint: mountpoint}
}

// Unmount request from Docker. If mount refcount is drop to 0.
// Unmount and detach from VM
func (d *VolumeImplDriver) Unmount(r volume.UnmountRequest) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Unmounting Volume ")

	return volume.Response{Err: ""}
}

func (d *VolumeDriver) MountVolume(name string, fstype string, id string, isReadOnly bool, skipAttach bool) (string, error) {

}
