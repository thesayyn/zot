// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package gql_generated

import (
	"time"
)

type Cve struct {
	ID          *string        `json:"Id"`
	Title       *string        `json:"Title"`
	Description *string        `json:"Description"`
	Severity    *string        `json:"Severity"`
	PackageList []*PackageInfo `json:"PackageList"`
}

type CVEResultForImage struct {
	Tag     *string `json:"Tag"`
	CVEList []*Cve  `json:"CVEList"`
}

type GlobalSearchResult struct {
	Images []*ImageSummary `json:"Images"`
	Repos  []*RepoSummary  `json:"Repos"`
	Layers []*LayerSummary `json:"Layers"`
}

type ImageInfo struct {
	Name        *string    `json:"Name"`
	Latest      *string    `json:"Latest"`
	LastUpdated *time.Time `json:"LastUpdated"`
	Description *string    `json:"Description"`
	Licenses    *string    `json:"Licenses"`
	Vendor      *string    `json:"Vendor"`
	Size        *string    `json:"Size"`
	Labels      *string    `json:"Labels"`
}

type ImageSummary struct {
	RepoName    *string    `json:"RepoName"`
	Tag         *string    `json:"Tag"`
	LastUpdated *time.Time `json:"LastUpdated"`
	IsSigned    *bool      `json:"IsSigned"`
	Size        *string    `json:"Size"`
	Platform    *OsArch    `json:"Platform"`
	Vendor      *string    `json:"Vendor"`
	Score       *int       `json:"Score"`
}

type ImgResultForCve struct {
	Name *string   `json:"Name"`
	Tags []*string `json:"Tags"`
}

type ImgResultForDigest struct {
	Name *string   `json:"Name"`
	Tags []*string `json:"Tags"`
}

type ImgResultForFixedCve struct {
	Tags []*TagInfo `json:"Tags"`
}

type LayerInfo struct {
	Size   *string `json:"Size"`
	Digest *string `json:"Digest"`
}

type LayerSummary struct {
	Size   *string `json:"Size"`
	Digest *string `json:"Digest"`
	Score  *int    `json:"Score"`
}

type ManifestInfo struct {
	Digest   *string      `json:"Digest"`
	Tag      *string      `json:"Tag"`
	IsSigned *bool        `json:"IsSigned"`
	Layers   []*LayerInfo `json:"Layers"`
}

type OsArch struct {
	Os   *string `json:"Os"`
	Arch *string `json:"Arch"`
}

type PackageInfo struct {
	Name             *string `json:"Name"`
	InstalledVersion *string `json:"InstalledVersion"`
	FixedVersion     *string `json:"FixedVersion"`
}

type RepoInfo struct {
	Manifests []*ManifestInfo `json:"Manifests"`
	Summary   *RepoSummary    `json:"Summary"`
}

type RepoSummary struct {
	Name        *string       `json:"Name"`
	LastUpdated *time.Time    `json:"LastUpdated"`
	Size        *string       `json:"Size"`
	Platforms   []*OsArch     `json:"Platforms"`
	Vendors     []*string     `json:"Vendors"`
	Score       *int          `json:"Score"`
	NewestTag   *ImageSummary `json:"NewestTag"`
}

type TagInfo struct {
	Name      *string    `json:"Name"`
	Digest    *string    `json:"Digest"`
	Timestamp *time.Time `json:"Timestamp"`
}
