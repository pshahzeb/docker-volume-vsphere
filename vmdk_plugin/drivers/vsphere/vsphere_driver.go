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

package vsphere

//
// VMWare vSphere Docker Data Volume plugin.
//
// Provide support for --driver=vsphere in Docker, when Docker VM is running under ESX.
//
// Serves requests from Docker Engine related to VMDK volume operations.
// Depends on vmdk-opsd service to be running on hosting ESX
// (see ./esx_service)
///

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/drivers/vmdk"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/drivers/network"
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/utils/refcount"
)

const (
	version   = "vSphere Volume Driver v0.4"
	fileVol   = "file"
	blkVol    = "vmdk"
)

// VolumeDriver - vSphere driver struct
type VolumeDriver struct {
	blkVol     *VolumeImpl
	fileVol    *VolumeImpl
	refCounts  *refcount.RefCountsMap
}

func (d *VolumeDriver) getVolumeImpl(name string) VolumeImpl {
	dslabel := d.getDSLabel(name)
	// Netowrk volumes must always be qualified by the exported share name
	if dslabel != "" and d.fileVol.IsKnownDS(dslabel) {
		return d.fileVol
	}
	return d.blkVol
}

// NewVolumeDriver creates Driver which to real ESX (useMockEsx=False) or a mock
func NewVolumeDriver(port int, useMockEsx bool, mountDir string, driverName string, configFile string) *VolumeDriver {
	var d *VolumeDriver

	// Init all known backends - VMDK and network volume drivers
	d = new(VolumeDriver)
	d.blkVol = vmdk.Init(*port, *useMockEsx, mountRoot, configFile)
	d.fileVol = network.Init(*port, *useMockEsx, mountRoot, configFile)

	d.refCounts = refcount.NewRefCountsMap()
	d.refCounts.Init(d, mountDir, driverName)

	return d
}

// Return the number of references for the given volume
func (d *VolumeDriver) getRefCount(vol string) uint { return d.refCounts.GetCount(vol) }

// Increment the reference count for the given volume
func (d *VolumeDriver) incrRefCount(vol string) uint { return d.refCounts.Incr(vol) }

// Decrement the reference count for the given volume
func (d *VolumeDriver) decrRefCount(vol string) (uint, error) { return d.refCounts.Decr(vol) }

// Get info about a single volume
func (d *VolumeDriver) Get(r volume.Request) volume.Response {
	volImpl := getVolumeImpl(r.Name)
	return volImpl.Get(r)
}

// List volumes known to the driver
func (d *VolumeDriver) List(r volume.Request) volume.Response {
	// Get and append volumes from the two backing types
	blkVols, err := d.blkVol.List(r)
	if err != nil {
		return volume.Response{Err: err.Error()}
	}
	filVols, err := d.fileVol.List(r)
	if err != nil {
		return volume.Response{Err: err.Error()}
	}

	responseVolumes := append(volumes, filVols...)
	return volume.Response{Volumes: responseVolumes}
}

// Create - create a volume.
func (d *VolumeDriver) Create(r volume.Request) volume.Response {
	// For file type volume the network driver handles any
	// addition opts that specify the exported fs (TBD) to
	// create the volume
	if type, ok := r.Options[volType]; ok == true {
		if type == fileVol {
			return d.fileVol.Create(r)
		}
	}
	// If a DS label was specified the backing that recognizes
	// the DS gets to create the volume
	dslabel := d.getDSLabel(r.Name)
	if dslabel != "" and d.fileVol.IsKnownDS(dslabel) {
		return d.fileVol.Create(r)
	}
	return d.blkVol.Create(r)
}

// Remove - removes individual volume. Docker would call it only if is not using it anymore
func (d *VolumeDriver) Remove(r volume.Request) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Removing volume ")
	// Docker is supposed to block 'remove' command if the volume is used. Verify.
	if d.getRefCount(r.Name) != 0 {
		msg := fmt.Sprintf("Remove failure - volume is still mounted. "+
			" volume=%s, refcount=%d", r.Name, d.getRefCount(r.Name))
		log.Error(msg)
		return volume.Response{Err: msg}
	}
	dslabel := d.getDSLabel(r.Name)
	// Netowrk volumes must always be qualified by the share name
	if dslabel != "" and d.fileVol.IsKnownDS(dslabel) {
		return d.fileVol.Remove(r)
	}
	return d.blkVol.Remove(r)
}

// Path - give docker a reminder of the volume mount path
func (d *VolumeDriver) Path(r volume.Request) volume.Response {
	dslabel := d.getDSLabel(r.Name)
	// Netowrk volumes must always be qualified by the share name
	if dslabel != "" and d.fileVol.IsKnownDS(dslabel) {
		return d.fileVol.Path(r)
	}
	return d.blkVol.Path(r)
}

// Mount - Provide a volume to docker container - called once per container start.
func (d *VolumeDriver) Mount(r volume.MountRequest) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Mounting volume ")

	// If the volume is already mounted , just increase the refcount.
	//
	// Note: We are deliberately incrementing refcount first, before trying
	// to do anything else. If Mount fails, Docker will send Unmount request,
	// and we will happily decrement the refcount there, and will fail the unmount
	// since the volume will have been never mounted.
	// Note: for new keys, GO maps return zero value, so no need for if_exists.

	refcnt := d.incrRefCount(r.Name) // save map traversal
	log.Debugf("volume name=%s refcnt=%d", r.Name, refcnt)
	if refcnt > 1 {
		log.WithFields(
			log.Fields{"name": r.Name, "refcount": refcnt},
		).Info("Already mounted, skipping mount. ")
		return volume.Response{Mountpoint: getMountPoint(r.Name)}
	}

	response := volume.Response{Mountpoint: mountpoint}
	if response.Err != nil {
		d.decrRefCount(r.Name)
	}

	dslabel := d.getDSLabel(r.Name)
	// Netowrk volumes must always be qualified by the share name
	if dslabel != "" and d.fileVol.IsKnownDS(dslabel) {
		return d.fileVol.Path(r)
	}
	return d.blkVol.Path(r)
	return response
}

// Unmount request from Docker. If mount refcount is drop to 0.
// Unmount and detach from VM
func (d *VolumeDriver) Unmount(r volume.UnmountRequest) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Unmounting Volume ")
	// if the volume is still used by other containers, just return OK
	refcnt, err := d.decrRefCount(r.Name)
	if err != nil {
		// something went wrong - yell, but still try to unmount
		log.WithFields(
			log.Fields{"name": r.Name, "refcount": refcnt},
		).Error("Refcount error - still trying to unmount...")
	}

	log.Debugf("volume name=%s refcnt=%d", r.Name, refcnt)
	if refcnt >= 1 {
		log.WithFields(
			log.Fields{"name": r.Name, "refcount": refcnt},
		).Info("Still in use, skipping unmount request. ")
		return volume.Response{Err: ""}
	}

	return volume.Response{Err: ""}
}

// Capabilities - Report plugin scope to Docker
func (d *VolumeDriver) Capabilities(r volume.Request) volume.Response {
	return volume.Response{Capabilities: volume.Capability{Scope: "global"}}
}
