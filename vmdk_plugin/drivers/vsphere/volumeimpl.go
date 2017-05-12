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

import "github.com/docker/go-plugins-helpers/volume"

// VolumeDriver interface used by the refcountedVolume module to handle
// recovery mounts/unmounts.
type VolumeImpl interface {
	Create(volume.Request) volume.Response
	Mount(volume.MountRequest, plugin_utils.VolumeInfo) (string, error)
	Unmount(string) error
	Get(string) (map[string]interface{}, error)
	Inspect(volume.Request) volume.Response
	Remove(volume.Request) volume.Response
	Path(volume.Request) volume.Response
	List() ([]*volume.Volume, error)
	GetMountPoint(string) string
	IsMounted(string) bool
}
