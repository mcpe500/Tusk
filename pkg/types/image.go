package types

import (
	"time"
)

type ImageIndex struct {
	SchemaVersion int              `json:"schemaVersion"`
	MediaType     string           `json:"mediaType,omitempty"`
	Manifests     []ImageManifest2 `json:"manifests"`
}

type ImageManifest2 struct {
	MediaType string         `json:"mediaType"`
	Digest    string         `json:"digest"`
	Size      int64          `json:"size"`
	Platform  ImagePlatform  `json:"platform"`
	URLs      []string       `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ImagePlatform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

type ImageManifest struct {
	SchemaVersion int              `json:"schemaVersion"`
	MediaType     string           `json:"mediaType,omitempty"`
	Config        Descriptor       `json:"config"`
	Layers        []Descriptor     `json:"layers"`
	Subject       *Descriptor      `json:"subject,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

type Descriptor struct {
	MediaType string            `json:"mediaType"`
	Digest    string            `json:"digest"`
	Size      int64             `json:"size"`
	URLs      []string          `json:"urls,omitempty"`
	Platform  *ImagePlatform    `json:"platform,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ImageConfig struct {
	Architecture string                 `json:"architecture"`
	OS           string                 `json:"os"`
	Config       ImageConfigDetails     `json:"config"`
	RootFS       RootFS                 `json:"rootfs"`
	History      []ImageHistory         `json:"history"`
	Created      *time.Time            `json:"created,omitempty"`
}

type ImageConfigDetails struct {
	Env          []string              `json:"Env,omitempty"`
	Cmd          []string              `json:"Cmd,omitempty"`
	Entrypoint   []string              `json:"Entrypoint,omitempty"`
	WorkingDir   string                `json:"WorkingDir,omitempty"`
	ExposedPorts map[string]struct{}   `json:"ExposedPorts,omitempty"`
	Labels       map[string]string     `json:"Labels,omitempty"`
	User         string                `json:"User,omitempty"`
	Volume       []string             `json:"Volume,omitempty"`
}

type RootFS struct {
	Type     string   `json:"type"`
	DiffIDs  []string `json:"diff_ids"`
}

type ImageHistory struct {
	Created    *time.Time           `json:"created,omitempty"`
	CreatedBy  string               `json:"created_by,omitempty"`
	Comment    string               `json:"comment,omitempty"`
	EmptyLayer bool                `json:"empty_layer,omitempty"`
}

type ManifestList []ImageManifest2

type ImageInfo struct {
	ID      string `json:"id"`
	Tags    []string `json:"tags"`
	Size    int64  `json:"size"`
	Created string `json:"created"`
}