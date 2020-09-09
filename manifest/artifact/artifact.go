package artifact

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest"
	v2 "github.com/notaryproject/artifacts/specs-go/v2"
	"github.com/opencontainers/go-digest"
)

// ArtifactVersion provides a pre-initialized version structure for this
// packages Artifact version of the manifest.
var ArtifactVersion = manifest.Versioned{
	SchemaVersion: 2,
	MediaType:     v2.MediaTypeArtifactManifest,
}

func init() {
	artifactFunc := func(b []byte) (distribution.Manifest, distribution.Descriptor, error) {
		d := new(DeserializedArtifact)
		err := d.UnmarshalJSON(b)
		if err != nil {
			return nil, distribution.Descriptor{}, err
		}

		if d.inner.MediaType != "" && d.inner.MediaType != v2.MediaTypeArtifactManifest {
			err = fmt.Errorf("if present, mediaType in artifact should be '%s' not '%s'",
				v2.MediaTypeArtifactManifest, d.inner.MediaType)

			return nil, distribution.Descriptor{}, err
		}

		dgst := digest.FromBytes(b)
		return d, distribution.Descriptor{Digest: dgst, Size: int64(len(b)), MediaType: v2.MediaTypeArtifactManifest}, err
	}
	err := distribution.RegisterManifestSchema(v2.MediaTypeArtifactManifest, artifactFunc)
	if err != nil {
		panic(fmt.Sprintf("Unable to register Artifact: %s", err))
	}
}

// Artifact references manifests for various registry artifacts.
type Artifact struct {
	inner v2.Artifact
}

// ArtifactType returns the artifactType of this Artifact.
func (a Artifact) ArtifactType() string {
	return a.inner.ArtifactType
}

// References returns the distribution descriptors for the referenced blobs.
func (a Artifact) References() []distribution.Descriptor {
	blobs := make([]distribution.Descriptor, len(a.inner.Blobs))
	for i := range a.inner.Blobs {
		blobs[i] = distribution.Descriptor{
			MediaType: a.inner.Blobs[i].MediaType,
			Digest:    a.inner.Blobs[i].Digest,
			Size:      a.inner.Blobs[i].Size,
		}
	}

	if a.inner.Config != nil {
		blobs = append(blobs, distribution.Descriptor{
			MediaType: a.inner.Config.MediaType,
			Digest:    a.inner.Config.Digest,
			Size:      a.inner.Config.Size,
		})
	}

	return blobs
}

// Manifests returns the distribution descriptors for the manifests that this artifact is linked to.
func (a Artifact) Manifests() []distribution.Descriptor {
	dependsOn := make([]distribution.Descriptor, len(a.inner.Manifests))
	for i := range a.inner.Manifests {
		dependsOn[i] = distribution.Descriptor{
			MediaType: a.inner.Manifests[i].MediaType,
			Digest:    a.inner.Manifests[i].Digest,
			Size:      a.inner.Manifests[i].Size,
		}
	}

	return dependsOn
}

// DeserializedArtifact wraps Artifact with a copy of the original JSON.
type DeserializedArtifact struct {
	Artifact

	// canonical is the canonical byte representation of the Artifact.
	canonical []byte
}

// UnmarshalJSON populates a new Artifact struct from JSON data.
func (d *DeserializedArtifact) UnmarshalJSON(b []byte) error {
	d.canonical = make([]byte, len(b))
	// store manifest list in canonical
	copy(d.canonical, b)

	// Unmarshal canonical JSON into Artifact object
	var artifact v2.Artifact
	if err := json.Unmarshal(d.canonical, &artifact); err != nil {
		return err
	}

	d.Artifact.inner = artifact

	return nil
}

// MarshalJSON returns the contents of canonical. If canonical is empty,
// marshals the inner contents.
func (d *DeserializedArtifact) MarshalJSON() ([]byte, error) {
	if len(d.canonical) > 0 {
		return d.canonical, nil
	}

	return nil, errors.New("JSON representation not initialized in DeserializedArtifact")
}

// Payload returns the raw content of the Artifact. The contents can be
// used to calculate the content identifier.
func (d DeserializedArtifact) Payload() (string, []byte, error) {
	var mediaType string
	if d.inner.MediaType == "" {
		mediaType = v2.MediaTypeArtifactManifest
	} else {
		mediaType = d.inner.MediaType
	}

	return mediaType, d.canonical, nil
}
