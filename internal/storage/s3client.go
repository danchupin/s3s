package storage

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// s3API is the subset of *s3.Client used here — keeps the impl testable and the
// surface explicitly read-only.
type s3API interface {
	ListBuckets(context.Context, *s3.ListBucketsInput, ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	CopyObject(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
}

// s3Client is the aws-sdk-go-v2 backed Storage + Mutator implementation.
type s3Client struct {
	api s3API
}

// New builds a Storage from a resolved (cluster + user) ClientConfig. Anonymous
// when cc.Anonymous; otherwise static credentials. Endpoint and path-style are
// applied per-client (the deprecated global resolver is avoided).
func New(cc ClientConfig) (Storage, error) {
	region := cc.Region
	if region == "" {
		region = "us-east-1"
	}

	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if cc.Anonymous {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(aws.AnonymousCredentials{}))
	} else {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cc.AccessKeyID, cc.SecretKey, cc.SessionToken),
		))
	}
	if cc.TLSSkipVerify {
		hc := awshttp.NewBuildableClient().WithTransportOptions(func(tr *http.Transport) {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit per-cluster opt-in (FR-004)
		})
		loadOpts = append(loadOpts, awsconfig.WithHTTPClient(hc))
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if cc.Endpoint != "" {
			o.BaseEndpoint = aws.String(cc.Endpoint)
		}
		o.UsePathStyle = cc.PathStyle
		// aws-sdk-go-v2 (Jan 2025+) defaults to WhenSupported, which adds a CRC32
		// data-integrity checksum and an aws-chunked trailer on writes. Many
		// S3-compatible backends (older MinIO, Ceph RGW) reject that trailer with a
		// 400, surfacing as an opaque write failure. WhenRequired adds a checksum only
		// when the API actually requires one, restoring compatibility for PutObject /
		// upload-part on non-AWS backends.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &s3Client{api: client}, nil
}

func (c *s3Client) ListBuckets(ctx context.Context) ([]Bucket, error) {
	out, err := c.api.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, classify(err)
	}
	buckets := make([]Bucket, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		buckets = append(buckets, Bucket{
			Name:         aws.ToString(b.Name),
			CreationDate: aws.ToTime(b.CreationDate),
		})
	}
	return buckets, nil
}

func (c *s3Client) ListLevel(ctx context.Context, q LevelQuery) (Page, error) {
	maxKeys := q.MaxKeys
	if maxKeys <= 0 {
		maxKeys = DefaultMaxKeys
	}
	effPrefix := q.Prefix + q.Search

	in := &s3.ListObjectsV2Input{
		Bucket:    aws.String(q.Bucket),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(maxKeys),
	}
	if effPrefix != "" {
		in.Prefix = aws.String(effPrefix)
	}
	if q.Token != nil && *q.Token != "" {
		in.ContinuationToken = q.Token
	}

	out, err := c.api.ListObjectsV2(ctx, in)
	if err != nil {
		return Page{}, classify(err)
	}

	dirs := make([]string, 0, len(out.CommonPrefixes))
	for _, cp := range out.CommonPrefixes {
		dirs = append(dirs, aws.ToString(cp.Prefix))
	}
	objs := make([]ObjectRef, 0, len(out.Contents))
	for _, o := range out.Contents {
		key := aws.ToString(o.Key)
		if key == effPrefix {
			continue // the "directory placeholder" key for this prefix, not a child
		}
		objs = append(objs, ObjectRef{
			Key:          key,
			Size:         aws.ToInt64(o.Size),
			LastModified: aws.ToTime(o.LastModified),
			StorageClass: string(o.StorageClass),
		})
	}

	var next *string
	if aws.ToBool(out.IsTruncated) {
		next = out.NextContinuationToken
	}
	return Page{Dirs: dirs, Objects: objs, NextToken: next}, nil
}

func (c *s3Client) HeadObject(ctx context.Context, bucket, key string) (ObjectMetadata, error) {
	out, err := c.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return ObjectMetadata{}, classify(err)
	}
	return ObjectMetadata{
		Key:          key,
		Size:         aws.ToInt64(out.ContentLength),
		LastModified: aws.ToTime(out.LastModified),
		ContentType:  aws.ToString(out.ContentType),
		StorageClass: string(out.StorageClass),
		ETag:         aws.ToString(out.ETag),
		UserMetadata: out.Metadata,
	}, nil
}

func (c *s3Client) GetObjectRange(ctx context.Context, bucket, key string, start, end int64) (io.ReadCloser, error) {
	out, err := c.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
	})
	if err != nil {
		return nil, classify(err)
	}
	return out.Body, nil
}

// GetObject streams the full object body (no Range). A read — the caller closes the
// reader. Used by download (005 FR-001/FR-002).
func (c *s3Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	out, err := c.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, classify(err)
	}
	return out.Body, nil
}

// UsageOf recursively lists every object under prefix (delimiter-less, paginated)
// and aggregates totals plus an immediate-child breakdown ranked largest-first. A
// read (005 FR-008..FR-012). onProgress (nil-safe) receives running totals; a
// cancelled ctx returns the partial report with Complete=false and ctx.Err().
func (c *s3Client) UsageOf(ctx context.Context, bucket, prefix string, onProgress func(UsageProgress)) (UsageReport, error) {
	agg := newUsageAgg(bucket, prefix)
	var token *string
	for {
		if err := ctx.Err(); err != nil {
			return agg.report(false), err
		}
		in := &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			ContinuationToken: token,
		}
		if prefix != "" {
			in.Prefix = aws.String(prefix)
		}
		out, err := c.api.ListObjectsV2(ctx, in)
		if err != nil {
			return agg.report(false), classify(err)
		}
		for _, o := range out.Contents {
			agg.add(aws.ToString(o.Key), aws.ToInt64(o.Size))
		}
		if onProgress != nil {
			onProgress(UsageProgress{ScannedCount: agg.totalCount, ScannedSize: agg.totalSize})
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}
	return agg.report(true), nil
}

// classify maps SDK/transport errors onto the package's error sentinels. It never
// embeds secret values — only error codes and HTTP statuses (FR-005, FR-021).
func classify(err error) error {
	if err == nil {
		return nil
	}
	// Cancellation/timeout pass through untouched so the UI can drop stale loads.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	var nsk *types.NoSuchKey
	var nsb *types.NoSuchBucket
	var nf *types.NotFound
	if errors.As(err, &nsk) || errors.As(err, &nsb) || errors.As(err, &nf) {
		return fmt.Errorf("%w", ErrNotFound)
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NoSuchBucket", "NotFound", "404":
			return fmt.Errorf("%w", ErrNotFound)
		case "AccessDenied", "Forbidden", "403", "401", "Unauthorized",
			"InvalidAccessKeyId", "SignatureDoesNotMatch":
			return fmt.Errorf("%w", ErrAccessDenied)
		}
	}

	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.HTTPStatusCode() {
		case http.StatusNotFound:
			return fmt.Errorf("%w", ErrNotFound)
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("%w", ErrAccessDenied)
		}
	}

	// Fell through: an error we couldn't map. If it carries an API error code or an
	// HTTP status, the backend WAS reached and rejected the request — log the raw
	// detail (code/status/message, never credentials) so an opaque "unreachable" can
	// be diagnosed from the log file. A bare transport failure (no response) stays a
	// genuine unreachable.
	if errors.As(err, &apiErr) {
		slog.Warn("s3 request rejected", "code", apiErr.ErrorCode(), "message", apiErr.ErrorMessage())
	} else if errors.As(err, &respErr) {
		slog.Warn("s3 request rejected", "status", respErr.HTTPStatusCode(), "err", respErr.Error())
	} else {
		slog.Debug("s3 transport error", "err", err.Error())
	}
	return fmt.Errorf("%w", ErrUnreachable)
}
