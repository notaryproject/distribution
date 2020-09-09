package storage

import (
	"context"
	"fmt"

	"github.com/docker/distribution"
	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/ociindex"
	"github.com/opencontainers/go-digest"
)

// ociIndexHandler is a ManifestHandler that covers OCI index with config.
type ociIndexHandler struct {
	repository distribution.Repository
	blobStore  distribution.BlobStore
	ctx        context.Context
	referrerMetadataStoreFunc
}

func (oih *ociIndexHandler) Unmarshal(ctx context.Context, dgst digest.Digest, content []byte) (distribution.Manifest, error) {
	dcontext.GetLogger(oih.ctx).Debug("(*ociIndexHandler).Unmarshal")

	doi := &ociindex.DeserializedOCIIndex{}
	if err := doi.UnmarshalJSON(content); err != nil {
		return nil, err
	}

	return doi, nil
}

func (oih *ociIndexHandler) Put(ctx context.Context, ociIndex distribution.Manifest, skipDependencyVerification bool) (digest.Digest, error) {
	dcontext.GetLogger(oih.ctx).Debug("(*ociIndexHandler).Put")

	doi, ok := ociIndex.(*ociindex.DeserializedOCIIndex)
	if !ok {
		return "", fmt.Errorf("wrong type put to ociIndexHandler: %T", ociIndex)
	}

	if err := oih.verifyManifest(oih.ctx, *doi, skipDependencyVerification); err != nil {
		return "", err
	}

	mt, payload, err := doi.Payload()
	if err != nil {
		return "", err
	}

	revision, err := oih.blobStore.Put(ctx, mt, payload)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("error putting payload into blobstore: %v", err)
		return "", err
	}

	err = oih.linkReferrerMetadata(ctx, *doi)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("error linking referrer metadata: %v", err)
		return "", err
	}

	return revision.Digest, nil
}

// verifyManifest ensures that the manifest content is valid from the
// perspective of the registry. As a policy, the registry only tries to
// store valid content, leaving trust policies of that content up to
// consumers.
func (oih *ociIndexHandler) verifyManifest(ctx context.Context, doi ociindex.DeserializedOCIIndex, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	if doi.SchemaVersion != 2 {
		return fmt.Errorf("unrecognized manifest list schema version %d", doi.SchemaVersion)
	}

	if !skipDependencyVerification {
		// This manifest service is different from the blob service
		// returned by Blob. It uses a linked blob store to ensure that
		// only manifests are accessible.

		manifestService, err := oih.repository.Manifests(ctx)
		if err != nil {
			return err
		}

		for _, manifestDescriptor := range doi.References() {
			exists, err := manifestService.Exists(ctx, manifestDescriptor.Digest)
			if err != nil && err != distribution.ErrBlobUnknown {
				errs = append(errs, err)
			}
			if err != nil || !exists {
				// On error here, we always append unknown blob errors.
				errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: manifestDescriptor.Digest})
			}
		}

		if configDigest := doi.Config.Digest; configDigest != "" {
			blobStore := oih.repository.Blobs(ctx)
			_, err = blobStore.Stat(ctx, configDigest)
		}
	}
	if len(errs) != 0 {
		return errs
	}

	return nil
}

func (oih *ociIndexHandler) linkReferrerMetadata(ctx context.Context, doi ociindex.DeserializedOCIIndex) error {
	// Link the manifest list config object as referrer metadata to each referenced manifest.
	for _, manifestDescriptor := range doi.References() {
		if err := oih.referrerMetadataStoreFunc(manifestDescriptor.Digest, doi.Config.MediaType).linkBlob(ctx, distribution.Descriptor{Digest: doi.Config.Digest}); err != nil {
			return err
		}
	}
	return nil
}
