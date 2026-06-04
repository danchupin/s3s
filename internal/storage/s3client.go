package storage

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
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
}

// s3Client is the aws-sdk-go-v2 backed read-only Storage implementation.
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

	return fmt.Errorf("%w", ErrUnreachable)
}
