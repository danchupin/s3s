package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

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
