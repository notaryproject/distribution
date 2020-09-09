package v2

// Artifact describes a registry artifact.
// This structure provides `application/vnd.oci.artifact.manifest.v1+json` mediatype when marshalled to JSON.
type Artifact struct {
	// MediaType is the media type of the object this schema refers to.
	MediaType string `json:"mediaType"`

	// ArtifactType is the artifact type of the object this schema refers to.
	ArtifactType string `json:"artifactType"`

	// Config references the index configuration.
	Config Descriptor `json:"config"`

	// Blobs references the content, such as signatures etc.
	Blobs []Descriptor `json:"blobs"`

	// Manifests is a collection of manifests this artifact is linked to.
	Manifests []Descriptor `json:"manifests"`
}
