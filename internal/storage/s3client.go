package storage

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
	GetObjectTagging(context.Context, *s3.GetObjectTaggingInput, ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error)
	GetBucketVersioning(context.Context, *s3.GetBucketVersioningInput, ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
	GetBucketEncryption(context.Context, *s3.GetBucketEncryptionInput, ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error)
	GetBucketLifecycleConfiguration(context.Context, *s3.GetBucketLifecycleConfigurationInput, ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error)
	GetBucketReplication(context.Context, *s3.GetBucketReplicationInput, ...func(*s3.Options)) (*s3.GetBucketReplicationOutput, error)
	GetPublicAccessBlock(context.Context, *s3.GetPublicAccessBlockInput, ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error)
	GetBucketLocation(context.Context, *s3.GetBucketLocationInput, ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
	ListMultipartUploads(context.Context, *s3.ListMultipartUploadsInput, ...func(*s3.Options)) (*s3.ListMultipartUploadsOutput, error)
	ListParts(context.Context, *s3.ListPartsInput, ...func(*s3.Options)) (*s3.ListPartsOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	CopyObject(context.Context, *s3.CopyObjectInput, ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	DeleteBucket(context.Context, *s3.DeleteBucketInput, ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
}

// s3Client is the aws-sdk-go-v2 backed Storage + Mutator implementation. presigner and
// creds back PresignGet (017 US3): both are client-side facilities — nil in unit tests
// that fake the api, always set by New.
type s3Client struct {
	api       s3API
	presigner *s3.PresignClient
	creds     aws.CredentialsProvider
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
	return &s3Client{api: client, presigner: s3.NewPresignClient(client), creds: cfg.Credentials}, nil
}

// PresignGet mints a presigned GET URL client-side (017 US3/FR-015). No network call is
// made; the URL is a bearer secret and is NEVER logged — only {bucket, key, ttl} is
// (constitution V, FR-016).
func (c *s3Client) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (string, string, error) {
	if !ValidPresignTTL(ttl) {
		return "", "", fmt.Errorf("%w: presign ttl must be one of 15m/1h/24h/7d", ErrInvalidConfig)
	}
	if c.presigner == nil {
		return "", "", fmt.Errorf("%w: presigner unavailable", ErrInvalidConfig)
	}
	req, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if err != nil {
		return "", "", classify(err)
	}
	warn := ""
	if c.creds != nil {
		if cr, cerr := c.creds.Retrieve(ctx); cerr == nil && cr.CanExpire && cr.Expires.Before(time.Now().Add(ttl)) {
			warn = "credentials expire before the link (" + cr.Expires.Format("2006-01-02 15:04") + ") — the link dies with them"
		}
	}
	slog.Info("presign", "bucket", bucket, "key", key, "ttl", ttl.String())
	return req.URL, warn, nil
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

		// 016: enrich from the SAME response — no extra round-trip.
		VersionID:           aws.ToString(out.VersionId),
		DeleteMarker:        aws.ToBool(out.DeleteMarker),
		SSEAlgorithm:        string(out.ServerSideEncryption),
		SSEKMSKeyID:         aws.ToString(out.SSEKMSKeyId),
		ReplicationStatus:   string(out.ReplicationStatus),
		RestoreStatus:       parseRestore(aws.ToString(out.Restore)),
		ObjectLockMode:      string(out.ObjectLockMode),
		ObjectLockRetainTil: aws.ToTime(out.ObjectLockRetainUntilDate),
		ObjectLockLegalHold: string(out.ObjectLockLegalHoldStatus),
		LifecycleExpiration: aws.ToString(out.Expiration),
		ContentEncoding:     aws.ToString(out.ContentEncoding),
		CacheControl:        aws.ToString(out.CacheControl),
		ContentDisposition:  aws.ToString(out.ContentDisposition),
	}, nil
}

// parseRestore reduces the x-amz-restore header to a short status. The header is
// `ongoing-request="true"` while a Glacier restore runs, or
// `ongoing-request="false", expiry-date="…"` once the temporary copy is available.
// Malformed/empty input yields "" (the field is then omitted), never a panic.
func parseRestore(s string) string {
	if s == "" {
		return ""
	}
	if strings.Contains(s, `ongoing-request="true"`) {
		return "in progress"
	}
	if i := strings.Index(s, `expiry-date="`); i >= 0 {
		rest := s[i+len(`expiry-date="`):]
		if j := strings.IndexByte(rest, '"'); j >= 0 {
			return "restored until " + rest[:j]
		}
	}
	return "restored"
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
// and aggregates totals, an immediate-child breakdown ranked largest-first, and
// age/size/class distributions in the same pass. A read (005 FR-008..FR-012,
// 017 FR-001/FR-020). maxObjects > 0 stops the enumeration within one page of the
// cap and reports Bounded=true (a lower bound, nil error). onProgress (nil-safe)
// receives running totals; a cancelled ctx returns the partial report with
// Complete=false and ctx.Err().
func (c *s3Client) UsageOf(ctx context.Context, bucket, prefix string, maxObjects int, onProgress func(UsageProgress)) (UsageReport, error) {
	agg := newUsageAgg(bucket, prefix, time.Now())
	var token *string
	for {
		if err := ctx.Err(); err != nil {
			return agg.report(false, false), err
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
			return agg.report(false, false), classify(err)
		}
		for _, o := range out.Contents {
			agg.add(aws.ToString(o.Key), aws.ToInt64(o.Size), aws.ToTime(o.LastModified), string(o.StorageClass))
		}
		if onProgress != nil {
			onProgress(UsageProgress{ScannedCount: agg.totalCount, ScannedSize: agg.totalSize})
		}
		// Truncation first: a cap reached on the FINAL page is an exact result, not a
		// lower bound (017 boundary edge case).
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		if maxObjects > 0 && agg.totalCount >= maxObjects {
			return agg.report(false, true), nil
		}
		token = out.NextContinuationToken
	}
	return agg.report(true, false), nil
}

// incompleteSizingCap bounds the per-upload ListParts size enrichment: a dangling-MPU
// population is typically tens, so 100 covers the real case at ≤100 extra requests —
// an unbounded fan-out would recreate the US1 problem on a different API (research D6).
const incompleteSizingCap = 100

// ListIncompleteUploads paginates ListMultipartUploads under prefix and size-enriches
// the first incompleteSizingCap uploads via ListParts (017 US4/FR-021). Denied and
// not-implemented map to tri-states, never to a zero result (FR-022).
func (c *s3Client) ListIncompleteUploads(ctx context.Context, bucket, prefix string) (IncompleteUploads, error) {
	out := IncompleteUploads{Bucket: bucket, Prefix: prefix}
	var keyMarker, idMarker *string
	type up struct {
		key, id string
	}
	var uploads []up
	for {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		in := &s3.ListMultipartUploadsInput{
			Bucket:         aws.String(bucket),
			KeyMarker:      keyMarker,
			UploadIdMarker: idMarker,
		}
		if prefix != "" {
			in.Prefix = aws.String(prefix)
		}
		page, err := c.api.ListMultipartUploads(ctx, in)
		if err != nil {
			return classifyIncomplete(out, err)
		}
		for _, u := range page.Uploads {
			out.Count++
			if t := aws.ToTime(u.Initiated); out.OldestInitiated.IsZero() || t.Before(out.OldestInitiated) {
				out.OldestInitiated = t
			}
			if len(uploads) < incompleteSizingCap {
				uploads = append(uploads, up{key: aws.ToString(u.Key), id: aws.ToString(u.UploadId)})
			}
		}
		if !aws.ToBool(page.IsTruncated) {
			break
		}
		keyMarker = page.NextKeyMarker
		idMarker = page.NextUploadIdMarker
	}
	for _, u := range uploads {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		var partMarker *string
		for {
			parts, err := c.api.ListParts(ctx, &s3.ListPartsInput{
				Bucket: aws.String(bucket), Key: aws.String(u.key), UploadId: aws.String(u.id),
				PartNumberMarker: partMarker,
			})
			if err != nil {
				break // sizing is best-effort; counting stays exact
			}
			for _, p := range parts.Parts {
				out.TotalSize += aws.ToInt64(p.Size)
			}
			if !aws.ToBool(parts.IsTruncated) {
				break
			}
			partMarker = parts.NextPartNumberMarker
		}
		out.SizedCount++
	}
	if out.Count == 0 {
		out.State = ConfigNone
	} else {
		out.State = ConfigConfigured
	}
	return out, nil
}

// classifyIncomplete maps a listing failure to the tri-state (denied/unsupported) or
// returns the classified error for everything else — never a silent zero (FR-022).
func classifyIncomplete(out IncompleteUploads, err error) (IncompleteUploads, error) {
	c := classify(err)
	switch {
	case errors.Is(c, ErrAccessDenied):
		out.State = ConfigDenied
		return out, nil
	case errors.Is(c, ErrUnsupported):
		out.State = ConfigUnsupported
		return out, nil
	default:
		return out, c
	}
}

// GetObjectTagging returns the object's tag key/value pairs (016 US4/FR-011).
func (c *s3Client) GetObjectTagging(ctx context.Context, bucket, key string) (ObjectTags, error) {
	out, err := c.api.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return ObjectTags{}, classify(err)
	}
	tags := make(map[string]string, len(out.TagSet))
	for _, t := range out.TagSet {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return ObjectTags{ObjectKey: key, Tags: tags}, nil
}

// GetBucketConfiguration fetches each governance sub-resource independently and
// classifies it (configured/none/denied/unsupported), so one failed sub-resource does
// not fail the whole call (016 US4/FR-012/FR-013).
func (c *s3Client) GetBucketConfiguration(ctx context.Context, bucket string) (BucketConfig, error) {
	cfg := BucketConfig{Bucket: bucket}

	if out, err := c.api.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)}); err != nil {
		cfg.Versioning = classifyConfig(err)
	} else if s := string(out.Status); s == "" {
		cfg.Versioning = ConfigItem{State: ConfigNone}
	} else {
		cfg.Versioning = ConfigItem{State: ConfigConfigured, Detail: s}
	}

	if out, err := c.api.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(bucket)}); err != nil {
		cfg.Encryption = classifyConfig(err)
	} else {
		cfg.Encryption = ConfigItem{State: ConfigConfigured, Detail: encryptionSummary(out.ServerSideEncryptionConfiguration)}
	}

	if out, err := c.api.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucket)}); err != nil {
		cfg.Lifecycle = classifyConfig(err)
	} else if n := len(out.Rules); n == 0 {
		cfg.Lifecycle = ConfigItem{State: ConfigNone}
	} else {
		cfg.Lifecycle = ConfigItem{State: ConfigConfigured, Detail: fmt.Sprintf("%d rule(s)", n)}
	}

	if out, err := c.api.GetBucketReplication(ctx, &s3.GetBucketReplicationInput{Bucket: aws.String(bucket)}); err != nil {
		cfg.Replication = classifyConfig(err)
	} else if out.ReplicationConfiguration == nil || len(out.ReplicationConfiguration.Rules) == 0 {
		cfg.Replication = ConfigItem{State: ConfigNone}
	} else {
		cfg.Replication = ConfigItem{State: ConfigConfigured, Detail: fmt.Sprintf("%d rule(s)", len(out.ReplicationConfiguration.Rules))}
	}

	if out, err := c.api.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)}); err != nil {
		cfg.PublicAccessBlock = classifyConfig(err)
	} else {
		cfg.PublicAccessBlock = ConfigItem{State: ConfigConfigured, Detail: publicAccessSummary(out.PublicAccessBlockConfiguration)}
	}

	if out, err := c.api.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)}); err != nil {
		cfg.Location = classifyConfig(err)
	} else if loc := string(out.LocationConstraint); loc == "" {
		cfg.Location = ConfigItem{State: ConfigConfigured, Detail: "us-east-1"}
	} else {
		cfg.Location = ConfigItem{State: ConfigConfigured, Detail: loc}
	}

	return cfg, nil
}

// encryptionSummary describes the default SSE rule without leaking the KMS key body.
func encryptionSummary(c *types.ServerSideEncryptionConfiguration) string {
	if c == nil || len(c.Rules) == 0 || c.Rules[0].ApplyServerSideEncryptionByDefault == nil {
		return "enabled"
	}
	return string(c.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm)
}

// publicAccessSummary reduces the four flags to "blocked" / "partial" / "open".
func publicAccessSummary(c *types.PublicAccessBlockConfiguration) string {
	if c == nil {
		return "open"
	}
	allBlocked := aws.ToBool(c.BlockPublicAcls) && aws.ToBool(c.BlockPublicPolicy) &&
		aws.ToBool(c.IgnorePublicAcls) && aws.ToBool(c.RestrictPublicBuckets)
	anyBlocked := aws.ToBool(c.BlockPublicAcls) || aws.ToBool(c.BlockPublicPolicy) ||
		aws.ToBool(c.IgnorePublicAcls) || aws.ToBool(c.RestrictPublicBuckets)
	switch {
	case allBlocked:
		return "blocked"
	case anyBlocked:
		return "partial"
	default:
		return "open"
	}
}

// classifyConfig maps a bucket-config sub-resource error to a ConfigItem: the
// not-configured family → none; NotImplemented/501/405 → unsupported; denied → denied;
// anything else → denied (couldn't read), carrying the classified sentinel. The
// not-found family is handled HERE (not in the generic classify) so "no lifecycle rule"
// reads as "none", never "unsupported" (016 FR-013, data-model §4).
func classifyConfig(err error) ConfigItem {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchTagSet", "ServerSideEncryptionConfigurationNotFoundError",
			"NoSuchLifecycleConfiguration", "ReplicationConfigurationNotFoundError",
			"NoSuchPublicAccessBlockConfiguration", "NoSuchConfiguration",
			"ObjectLockConfigurationNotFoundError":
			return ConfigItem{State: ConfigNone}
		}
	}
	switch {
	case errors.Is(classify(err), ErrUnsupported):
		return ConfigItem{State: ConfigUnsupported, Reason: ErrUnsupported}
	case errors.Is(classify(err), ErrAccessDenied):
		return ConfigItem{State: ConfigDenied, Reason: ErrAccessDenied}
	default:
		return ConfigItem{State: ConfigDenied, Reason: classify(err)}
	}
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
		case "NotImplemented", "MethodNotAllowed":
			return fmt.Errorf("%w", ErrUnsupported)
		}
	}

	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		switch respErr.HTTPStatusCode() {
		case http.StatusNotFound:
			return fmt.Errorf("%w", ErrNotFound)
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("%w", ErrAccessDenied)
		case http.StatusNotImplemented, http.StatusMethodNotAllowed:
			return fmt.Errorf("%w", ErrUnsupported)
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
