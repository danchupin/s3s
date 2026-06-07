package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// progressEvery is how often (in objects processed) the fake reports recursive-delete
// progress; the real client reports per object (non-blocking, best-effort).
const progressEvery = 100

// s3Client and Fake both satisfy Mutator.
var (
	_ Mutator = (*s3Client)(nil)
	_ Mutator = (*Fake)(nil)
)

// normalizeFolderKey validates a folder prefix and returns it normalised to
// exactly one trailing "/". It rejects empty/whitespace-only names and any name
// containing control characters, returning ErrInvalidName — all before any
// network call (FR-010). The trailing slash count is normalised so "a/b",
// "a/b/", and "a/b//" all yield "a/b/".
func normalizeFolderKey(prefix string) (string, error) {
	trimmed := strings.TrimRight(prefix, "/")
	if strings.TrimSpace(trimmed) == "" {
		return "", fmt.Errorf("%w: folder name is empty", ErrInvalidName)
	}
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("%w: folder name has a control character", ErrInvalidName)
		}
	}
	return trimmed + "/", nil
}

// CreateFolder puts a zero-length object at the normalised "<prefix>/" key. This
// is the de-facto empty-folder convention for MinIO/Ceph RGW (FR-009). It does not
// pre-check existence: PutObject is idempotent, and the UI does an advisory
// collision check from the loaded level before confirming.
func (c *s3Client) CreateFolder(ctx context.Context, bucket, prefix string) error {
	key, err := normalizeFolderKey(prefix)
	if err != nil {
		return err
	}
	_, err = c.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(nil),
		ContentLength: aws.Int64(0),
	})
	if err != nil {
		return classify(err)
	}
	return nil
}

// validateDestKey rejects an empty/whitespace/control destination key, or a
// destination identical to the source, before any network call (FR-013).
func validateDestKey(srcKey, dstKey string) error {
	if strings.TrimSpace(dstKey) == "" {
		return fmt.Errorf("%w: destination key is empty", ErrInvalidName)
	}
	for _, r := range dstKey {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: destination key has a control character", ErrInvalidName)
		}
	}
	if dstKey == srcKey {
		return fmt.Errorf("%w: destination equals source", ErrInvalidName)
	}
	return nil
}

// RemoveObject deletes a single object (FR-001). A not-found key is surfaced as
// ErrNotFound by classify; the UI treats it as already-gone.
func (c *s3Client) RemoveObject(ctx context.Context, bucket, key string) error {
	_, err := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return classify(err)
	}
	return nil
}

// UploadFile streams r (size bytes) to the object at key with a single PutObject
// (FR-002). The body MUST be seekable: SigV4 reads it to compute the
// x-amz-content-sha256 hash then rewinds to send, so a non-seekable body produces an
// XAmzContentSHA256Mismatch. The UI's countingReader satisfies io.ReadSeeker. Honors
// ctx cancellation, so a cancelled upload is never a success. (Multipart for >5 GiB
// objects is out of scope.)
func (c *s3Client) UploadFile(ctx context.Context, bucket, key string, r io.Reader, size int64) error {
	_, err := c.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return classify(err)
	}
	return nil
}

// CopyKey server-side copies srcKey to dstKey within the same bucket (FR-004). The
// CopySource is "<bucket>/<key>" URL-escaped so keys with spaces/special bytes copy
// correctly. The source is untouched.
func (c *s3Client) CopyKey(ctx context.Context, bucket, srcKey, dstKey string) error {
	if err := validateDestKey(srcKey, dstKey); err != nil {
		return err
	}
	source := url.PathEscape(bucket + "/" + srcKey)
	_, err := c.api.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(source),
	})
	if err != nil {
		return classify(err)
	}
	return nil
}

// MoveObject = CopyKey(src->dst) then RemoveObject(src) (FR-006). Ordering is fixed
// so no data is lost: a copy failure leaves the source intact and never deletes it;
// a copy success followed by a delete failure returns ErrMovePartial (data is safe
// at the destination, source remains) — FR-007.
func (c *s3Client) MoveObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	if err := c.CopyKey(ctx, bucket, srcKey, dstKey); err != nil {
		return err // source intact; nothing copied or copy failed
	}
	if err := c.RemoveObject(ctx, bucket, srcKey); err != nil {
		return ErrMovePartial // copied OK; source delete failed — no data loss
	}
	return nil
}

// RemoveBucket deletes an EMPTY bucket. It first probes for any content via
// ListObjectsV2 (MaxKeys=1); if anything is present it returns ErrBucketNotEmpty
// WITHOUT calling DeleteBucket — bucket delete never recursively purges (007 FR-024b).
// Only on an empty bucket does it issue DeleteBucket.
func (c *s3Client) RemoveBucket(ctx context.Context, bucket string) error {
	out, err := c.api.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return classify(err)
	}
	if aws.ToInt32(out.KeyCount) > 0 || len(out.Contents) > 0 {
		return ErrBucketNotEmpty
	}
	if _, err := c.api.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
		return classify(err)
	}
	return nil
}

// DeleteRecursive enumerates every object under prefix via ListObjectsV2 (paginated)
// and deletes them one at a time with DeleteObject, best-effort: a per-object failure
// is counted into Failed and the run continues (FR-009). Per-object DeleteObject (vs
// the batch DeleteObjects) is used deliberately: batch deletes require a Content-MD5
// header that current AWS SDK checksum defaults no longer emit, which older MinIO/Ceph
// RGW reject — single deletes carry no such requirement and work on every S3 backend.
// onProgress (nil-safe) gets cumulative counts; a cancelled ctx stops further work and
// returns the counts so far with ctx.Err() (FR-011, US5 AS4).
func (c *s3Client) DeleteRecursive(ctx context.Context, bucket, prefix string, onProgress func(DeleteSummary)) (DeleteSummary, error) {
	var sum DeleteSummary
	var token *string
	for {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		out, err := c.api.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return sum, classify(err)
		}
		for _, o := range out.Contents {
			if err := ctx.Err(); err != nil {
				return sum, err
			}
			_, derr := c.api.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: o.Key})
			if derr != nil {
				if cl := classify(derr); errors.Is(cl, context.Canceled) {
					return sum, cl // cancellation: stop and report partial counts
				}
				sum.Failed++ // best-effort: count and continue
			} else {
				sum.Deleted++
			}
			if onProgress != nil {
				onProgress(sum)
			}
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}
	return sum, nil
}
