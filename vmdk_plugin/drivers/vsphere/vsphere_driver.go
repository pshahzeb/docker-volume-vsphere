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
	"github.com/vmware/docker-volume-vsphere/vmdk_plugin/utils/config"
)

const (
	version   = "vSphere Volume Driver v0.4"
	fileType  = "file"
	vmdkImpl  = "vmdk"
	nfsImpl   = "nfs"
)

type MountedVolume struct {
	// List of mount IDs that mounted this volume
	mountIDs []string
	// VolumeImpl driver for this volume
	fsType  string
}

// VolumeDriver - vSphere driver struct
type VolumeDriver struct {
	refCounts  *refcount.RefCountsMap
	mountedVols map[string]MountedVolume
	config config.Config
}

var volumeBackingMap map[string]VolumeImpl

// getDSLabel - Split volume name into volume name and DS label
func (d *VolumeDriver) getDSLabel(name string) string {
	if list := strings.split(name, "@"); len(list) > 1 {
		return list[1]
	}
	return ""
}

func (d *VolumeDriver) getVolumeFromMap(name string) (VolumeImpl, string) {
	// If the volume is mounted then get the backing for
	// it from the mounted volumes map.
	if fsType, ok := d.mountedVols[name]; ok {
		return volumeBackingMap[fsType], fsType
	}

	// Else figure the FS type for the label and use the
	// volume impl for that.
	dslabel := d.getDSLabel(name)
	if dslabel != "" && d.config.RemoteDirs {
		if rdir, ok := d.config.RemoteDirs[dslabel]; ok {
			return volumeBackingMap[rdir.FSType], rdir.FSType
		}
	}
	return volumeBackingMap[vmdkType], ""
}

// NewVolumeDriver creates Driver which to real ESX (useMockEsx=False) or a mock
func NewVolumeDriver(port int, useMockEsx bool, mountDir string, driverName string, configFile string) *VolumeDriver {
	var d *VolumeDriver

	d.config, err := config.Load(configFile)	
	if err != nil {
		log.Warning("Failed to load config file - ", configFile)
		return nil, err
	}

	// Init all known backends - VMDK and network volume drivers
	d = new(VolumeDriver)
	volumeBackingMap[vmdkType], err := vmdk.Init(*port, *useMockEsx, mountRoot)
	if err != nil {
		return nil
	}

	volumeBackingMap[nfsImpl], err := network.Init(mountRoot, config)
	if err != nil {
		return nil
	}

	refCounts :=  refcount.NewRefCountsMap()
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
	volImpl, _ := getVolumeFromMap(r.Name)
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
	// addition opts that specify the exported fs to
	// create the volume
	if volType, ok := r.Options[volType]; ok == true {
		if volType == fileType {
			return volumeBackingMap[fileType].Create(r)
		}
	}
	// If a DS label was specified the backing that recognizes
	// the DS gets to create the volume
	dslabel := d.getDSLabel(r.Name)
	if dslabel != "" && volumeBackingMap[fileType].IsKnownDS(dslabel) {
		return volumeBackingMap[fileType].Create(r)
	}
	return volumeBackingMap[vmdkType].Create(r)
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
	volImpl, _ := d.getVolumeFromMap(r.Name)
	return volImpl.Remove(r)
}

// Path - give docker a reminder of the volume mount path
func (d *VolumeDriver) Path(r volume.Request) volume.Response {
	volImpl, _ := d.getVolumeFromMap(r.Name)
	return volImpl.Path(r)
}

// Mount - Provide a volume to docker container - called once per container start.
func (d *VolumeDriver) Mount(r volume.MountRequest) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Mounting volume ")

	volImpl, _ := d.getVolumeFromMap(r.Name)

	// lock the state
	d.refCounts.LockStateLock()
	defer d.refCounts.UnlockStateLock()

	// Checked by refcounting thread until refmap initialized
	d.refCounts.MarkDirty()

	// Get the full name for the given volume
	volInfo, err := plugin_utils.GetVolumeInfo(r.Name, "", d)
	if err != nil {
		log.Errorf("Unable to get volume info for volume %s. err:%v", r.Name, err)
		return volume.Response{Err: err.Error()}
	}

	fname = volInfo.VolumeName
	d.mountedVolumes[fname].fsType = fstype

	// If the volume is already mounted , increase the refcount.
	// Note: for new keys, GO maps return zero value, so no need for if_exists.
	refcnt := d.incrRefCount(fname) // save map traversal
	log.Debugf("volume name=%s refcnt=%d", fname, refcnt)
	if refcnt > 1 || volImpl.IsMounted(fname) {
		log.WithFields(
			log.Fields{"name": fname, "refcount": refcnt},
		).Info("Already mounted, skipping mount. ")
		return volume.Response{Mountpoint: volImpl.GetMountPoint(fname)}
	}

	response := volImpl.Mount(r, volInfo)
	if response.Err != "" {
		d.decrRefCount(fname)
		d.refCounts.ClearDirty()
	}
	return response	
}

// Unmount request from Docker. If mount refcount is drop to 0.
// Unmount and detach from VM
func (d *VolumeDriver) Unmount(r volume.UnmountRequest) volume.Response {
	log.WithFields(log.Fields{"name": r.Name}).Info("Unmounting Volume ")
	volImpl, _ := d.getVolumeFromMap(r.Name)

	// lock the state
	d.refCounts.StateMtx.Lock()
	defer d.refCounts.StateMtx.Unlock()

	if d.refCounts.GetInitSuccess() != true {
		// if refcounting hasn't been succesful,
		// no refcounting, no unmount. All unmounts are delayed
		// until we succesfully populate the refcount map
		d.refCounts.MarkDirty()
		return volume.Response{Err: ""}
	}

	fname, exist := d.mountIDtoName[r.ID]
	if exist {
		delete(d.mountIDtoName, r.ID) //cleanup the map
	} else {
		volumeInfo, err := plugin_utils.GetVolumeInfo(r.Name, "", d)
		if err != nil {
			log.Errorf("Unable to get volume info for volume %s. err:%v", r.Name, err)
			return volume.Response{Err: err.Error()}
		}
		fname = volumeInfo.VolumeName
	}

	// if refcount has been succcessful, Normal flow
	// if the volume is still used by other containers, just return OK
	refcnt, err := d.decrRefCount(fname)
	if err != nil {
		// something went wrong - yell, but still try to unmount
		log.WithFields(
			log.Fields{"name": fname, "refcount": refcnt},
		).Error("Refcount error - still trying to unmount...")
	}
	log.Debugf("volume name=%s refcnt=%d", fname, refcnt)
	if refcnt >= 1 {
		log.WithFields(
			log.Fields{"name": fname, "refcount": refcnt},
		).Info("Still in use, skipping unmount request. ")
		return volume.Response{Err: ""}
	}

	return volImpl.Umount(volume.UnmountRequest{Name: fname})
}

// Capabilities - Report plugin scope to Docker
func (d *VolumeDriver) Capabilities(r volume.Request) volume.Response {
	return volume.Response{Capabilities: volume.Capability{Scope: "global"}}
}

// MountVolume - mount a volume without reference counting
func (d *VolumeDriver) MountVolume(string, string, string, bool, bool) (string, error) {

}

// UnmountVolume - unmount a volume without reference counting
func (d *VolumeDriver) UnmountVolume(string) error {

}

// GetVolume - get volume data.
func (d *VolumeDriver) GetVolume(string) (map[string]interface{}, error) {

}

// VolumesInRefMap - return a list of volumes from the refcounter
func (d *VolumeDriver) VolumesInRefMap() []string {

}
