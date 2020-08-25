package storage

import (
	"context"
	"fmt"

	"github.com/docker/distribution"
	dcontext "github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/opencontainers/go-digest"
)

// ociIndexHandler is a ManifestHandler that covers OCI index with config.
type ociIndexHandler struct {
	repository distribution.Repository
	blobStore  distribution.BlobStore
	ctx        context.Context
	referrerMetadataStoreFunc
}

func (ms *ociIndexHandler) Unmarshal(ctx context.Context, dgst digest.Digest, content []byte) (distribution.Manifest, error) {
	dcontext.GetLogger(ms.ctx).Debug("(*ociIndexHandler).Unmarshal")

	m := &manifestlist.DeserializedManifestList{}
	if err := m.UnmarshalJSON(content); err != nil {
		return nil, err
	}

	return m, nil
}

func (ms *ociIndexHandler) Put(ctx context.Context, manifestList distribution.Manifest, skipDependencyVerification bool) (digest.Digest, error) {
	dcontext.GetLogger(ms.ctx).Debug("(*ociIndexHandler).Put")

	m, ok := manifestList.(*manifestlist.DeserializedManifestList)
	if !ok {
		return "", fmt.Errorf("wrong type put to ociIndexHandler: %T", manifestList)
	}

	if err := ms.verifyManifest(ms.ctx, *m, skipDependencyVerification); err != nil {
		return "", err
	}

	mt, payload, err := m.Payload()
	if err != nil {
		return "", err
	}

	revision, err := ms.blobStore.Put(ctx, mt, payload)
	if err != nil {
		dcontext.GetLogger(ctx).Errorf("error putting payload into blobstore: %v", err)
		return "", err
	}

	err = ms.linkReferrerMetadata(ctx, *m)
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
func (ms *ociIndexHandler) verifyManifest(ctx context.Context, mnfst manifestlist.DeserializedManifestList, skipDependencyVerification bool) error {
	var errs distribution.ErrManifestVerification

	if mnfst.SchemaVersion != 3 {
		return fmt.Errorf("unrecognized manifest list schema version %d", mnfst.SchemaVersion)
	}

	if !skipDependencyVerification {
		// This manifest service is different from the blob service
		// returned by Blob. It uses a linked blob store to ensure that
		// only manifests are accessible.

		manifestService, err := ms.repository.Manifests(ctx)
		if err != nil {
			return err
		}

		for _, manifestDescriptor := range mnfst.References() {
			exists, err := manifestService.Exists(ctx, manifestDescriptor.Digest)
			if err != nil && err != distribution.ErrBlobUnknown {
				errs = append(errs, err)
			}
			if err != nil || !exists {
				// On error here, we always append unknown blob errors.
				errs = append(errs, distribution.ErrManifestBlobUnknown{Digest: manifestDescriptor.Digest})
			}
		}

		if configDigest := mnfst.Config.Digest; configDigest != "" {
			blobStore := ms.repository.Blobs(ctx)
			_, err = blobStore.Stat(ctx, configDigest)
		}
	}
	if len(errs) != 0 {
		return errs
	}

	return nil
}

func (ms *ociIndexHandler) linkReferrerMetadata(ctx context.Context, mnfst manifestlist.DeserializedManifestList) error {
	// Link the manifest list config object as referrer metadata to each referenced manifest.
	for _, manifestDescriptor := range mnfst.References() {
		if err := ms.referrerMetadataStoreFunc(manifestDescriptor.Digest, mnfst.Config.MediaType).linkBlob(ctx, mnfst.Config); err != nil {
			return err
		}
	}
	return nil
}
