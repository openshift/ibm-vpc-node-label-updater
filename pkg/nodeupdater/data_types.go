/**
 * Copyright 2020 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//Package nodeupdater ...
package nodeupdater

import (
	"net/url"
	"time"
)

// NodeInfo ...
type NodeInfo struct {
	InstanceID string
	Region     string
	Zone       string
}

// StorageSecretConfig ...
type StorageSecretConfig struct {
	RiaasEndpointURL *url.URL
	IAMAccessToken   string
}

// AccessTokenResponse ...
type AccessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	Expiration   int64  `json:"expiration,omitempty"`
}

// Instance ...
type Instance struct {
	Href                    string              `json:"href,omitempty"`
	ID                      string              `json:"id,omitempty"`
	Name                    string              `json:"name,omitempty"`
	Memory                  int64               `json:"memory,omitempty"`
	ResourceGroup           *ResourceGroup      `json:"resource_group,omitempty"`
	Vcpu                    *Vcpu               `json:"vcpu,omitempty"`
	Vpc                     *Vpc                `json:"vpc,omitempty"`
	CreatedAt               *time.Time          `json:"created_at,omitempty"`
	Status                  string              `json:"status,omitempty"`
	VolumeAttachments       *[]VolumeAttachment `json:"volume_attachments,omitempty"`
	NetworkInterfaces       *[]NetworkInterface `json:"network_interfaces,omitempty"`
	PrimaryNetworkInterface *NetworkInterface   `json:"primary_network_interface,omitempty"`
	BootVolumeAttachment    *VolumeAttachment   `json:"boot_volume_attachment,omitempty"`

	Zone    *Zone    `json:"zone,omitempty"`
	CRN     string   `json:"crn,omitempty"`
	Image   *Image   `json:"image,omitempty"`
	Profile *Profile `json:"profile,omitempty"`
}

// Zone ...
type Zone struct {
	Name string `json:"name,omitempty"`
	Href string `json:"href,omitempty"`
}

// Profile ...
type Profile struct {
	Name string `json:"name,omitempty"`
	Href string `json:"href,omitempty"`
}

// ResourceGroup ...
type ResourceGroup struct {
	Href string `json:"href,omitempty"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// VolumeAttachment ...
type VolumeAttachment struct {
	ID     string  `json:"id,omitempty"`
	Href   string  `json:"href,omitempty"`
	Name   string  `json:"name,omitempty"`
	Volume *Volume `json:"volume,omitempty"`
	Device *Device `json:"device,omitempty"`
}

// Device ...
type Device struct {
	ID string `json:"id,omitempty"`
}

// Volume ...
type Volume struct {
	ID   string `json:"id,omitempty"`
	Href string `json:"href,omitempty"`
	Name string `json:"name,omitempty"`
	CRN  string `json:"crn,omitempty"`
}

// Image ...
type Image struct {
	ID   string `json:"id,omitempty"`
	Href string `json:"href,omitempty"`
	Name string `json:"name,omitempty"`
	CRN  string `json:"crn,omitempty"`
}

// Vpc ...
type Vpc struct {
	ID           string `json:"id,omitempty"`
	Href         string `json:"href,omitempty"`
	Name         string `json:"name,omitempty"`
	CRN          string `json:"crn,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
}

// Vcpu ...
type Vcpu struct {
	Architecture string `json:"architecture,omitempty"`
	Count        int64  `json:"count,omitempty"`
}

// NetworkInterface ...
type NetworkInterface struct {
	ID                 string  `json:"id,omitempty"`
	Href               string  `json:"href,omitempty"`
	Name               string  `json:"name,omitempty"`
	PrimaryIpv4Address string  `json:"primary_ipv4_address,omitempty"`
	ResourceTyoe       string  `json:"resource_type,omitempty"`
	Subnet             *Subnet `json:"subnet,omitempty"`
}

// Subnet ...
type Subnet struct {
	ID           string `json:"id,omitempty"`
	Href         string `json:"href,omitempty"`
	Name         string `json:"name,omitempty"`
	CRN          string `json:"crn,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
}

// InstanceList ...
type InstanceList struct {
	First      *HReference `json:"first,omitempty"`
	Next       *HReference `json:"next,omitempty"`
	Instances  []*Instance `json:"instances"`
	Limit      int         `json:"limit,omitempty"`
	TotalCount int         `json:"total_count,omitempty"`
}

// HReference ...
type HReference struct {
	Href string `json:"href,omitempty"`
}
