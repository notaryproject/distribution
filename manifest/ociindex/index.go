package ociindex

import (
	"encoding/json"
	"errors"
	"fmt"

	v2 "github.com/aviral26/artifacts/specs-go/v2"
	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest"
	"github.com/opencontainers/go-digest"
)

// OCISchemaVersion provides a pre-initialized version structure for this
// packages OCIschema version of the manifest.
var OCISchemaVersion = manifest.Versioned{
	SchemaVersion: 2,
	MediaType:     v2.MediaTypeImageIndex,
}

func init() {
	imageIndexFunc := func(b []byte) (distribution.Manifest, distribution.Descriptor, error) {
		doi := new(DeserializedOCIIndex)
		err := doi.UnmarshalJSON(b)
		if err != nil {
			return nil, distribution.Descriptor{}, err
		}

		if doi.MediaType != "" && doi.MediaType != v2.MediaTypeImageIndex {
			err = fmt.Errorf("if present, mediaType in image index should be '%s' not '%s'",
				v2.MediaTypeImageIndex, doi.MediaType)

			return nil, distribution.Descriptor{}, err
		}

		dgst := digest.FromBytes(b)
		return doi, distribution.Descriptor{Digest: dgst, Size: int64(len(b)), MediaType: v2.MediaTypeImageIndex}, err
	}
	err := distribution.RegisterManifestSchema(v2.MediaTypeImageIndex, imageIndexFunc)
	if err != nil {
		panic(fmt.Sprintf("Unable to register OCI Image Index: %s", err))
	}
}

// OCIIndex references manifests for various platforms.
type OCIIndex struct {
	v2.Index
}

// References returns the distribution descriptors for the referenced image
// manifests.
func (oi OCIIndex) References() []distribution.Descriptor {
	dependencies := make([]distribution.Descriptor, len(oi.Manifests))
	for i := range oi.Manifests {
		dependencies[i] = distribution.Descriptor{
			MediaType:   oi.Manifests[i].MediaType,
			Annotations: oi.Manifests[i].Annotations,
			Digest:      oi.Manifests[i].Digest,
			Platform:    oi.Manifests[i].Platform,
			Size:        oi.Manifests[i].Size,
			URLs:        oi.Manifests[i].URLs,
		}
	}

	return dependencies
}

// DeserializedOCIIndex wraps OCIIndex with a copy of the original
// JSON.
type DeserializedOCIIndex struct {
	OCIIndex

	// canonical is the canonical byte representation of the Manifest.
	canonical []byte
}

// UnmarshalJSON populates a new OCIIndex struct from JSON data.
func (doi *DeserializedOCIIndex) UnmarshalJSON(b []byte) error {
	doi.canonical = make([]byte, len(b))
	// store manifest list in canonical
	copy(doi.canonical, b)

	// Unmarshal canonical JSON into OCIIndex object
	var index v2.Index
	if err := json.Unmarshal(doi.canonical, &index); err != nil {
		return err
	}

	doi.Index = index

	return nil
}

// MarshalJSON returns the contents of canonical. If canonical is empty,
// marshals the inner contents.
func (doi *DeserializedOCIIndex) MarshalJSON() ([]byte, error) {
	if len(doi.canonical) > 0 {
		return doi.canonical, nil
	}

	return nil, errors.New("JSON representation not initialized in DeserializedOCIIndex")
}

// Payload returns the raw content of the OCI index. The contents can be
// used to calculate the content identifier.
func (doi DeserializedOCIIndex) Payload() (string, []byte, error) {
	var mediaType string
	if doi.MediaType == "" {
		mediaType = v2.MediaTypeImageIndex
	} else {
		mediaType = doi.MediaType
	}

	return mediaType, doi.canonical, nil
}
