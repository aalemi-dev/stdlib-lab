package minio

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	miniogo "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newClient() *MinioClient {
	return &MinioClient{cfg: Config{}}
}

// ── TranslateError ────────────────────────────────────────────────────────────

func TestTranslateError_Nil(t *testing.T) {
	t.Parallel()
	assert.NoError(t, newClient().TranslateError(nil))
}

func TestTranslateError_PlainError(t *testing.T) {
	t.Parallel()
	err := newClient().TranslateError(errors.New("connection refused"))
	assert.ErrorIs(t, err, ErrConnectionFailed)
}

func TestTranslateMinIOError_AllCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code string
		want error
	}{
		{"NoSuchBucket", ErrBucketNotFound},
		{"NoSuchKey", ErrObjectNotFound},
		{"BucketAlreadyExists", ErrBucketAlreadyExists},
		{"BucketAlreadyOwnedByYou", ErrBucketAlreadyExists},
		{"InvalidBucketName", ErrInvalidBucketName},
		{"InvalidObjectName", ErrInvalidObjectName},
		{"AccessDenied", ErrAccessDenied},
		{"InvalidAccessKeyId", ErrInvalidCredentials},
		{"SignatureDoesNotMatch", ErrInvalidCredentials},
		{"TokenRefreshRequired", ErrCredentialsExpired},
		{"ExpiredToken", ErrCredentialsExpired},
		{"InvalidRange", ErrInvalidRange},
		{"EntityTooLarge", ErrObjectTooLarge},
		{"InvalidDigest", ErrInvalidChecksum},
		{"BadDigest", ErrInvalidChecksum},
		{"InvalidArgument", ErrInvalidArgument},
		{"MalformedXML", ErrMalformedXML},
		{"InvalidStorageClass", ErrInvalidStorageClass},
		{"InvalidPart", ErrInvalidPart},
		{"InvalidPartOrder", ErrInvalidPart},
		{"NoSuchUpload", ErrInvalidUploadID},
		{"PreconditionFailed", ErrPreconditionFailed},
		{"BucketNotEmpty", ErrBucketNotEmpty},
		{"TooManyBuckets", ErrQuotaExceeded},
		{"InvalidRequest", ErrInvalidArgument},
		{"MetadataTooLarge", ErrInvalidMetadata},
		{"InvalidEncryptionAlgorithm", ErrInvalidEncryption},
		{"InvalidObjectState", ErrObjectLocked},
		{"ObjectLockConfigurationNotFoundError", ErrLockConfigurationError},
		{"InvalidRetentionDate", ErrRetentionPolicyError},
		{"InvalidVersionId", ErrInvalidVersion},
		{"NoSuchVersion", ErrInvalidVersion},
		{"InvalidLocationConstraint", ErrConfigurationError},
		{"CrossLocationLoggingProhibited", ErrLoggingError},
		{"InvalidTargetBucketForLogging", ErrLoggingError},
		{"InvalidPolicyDocument", ErrPolicyError},
		{"MalformedPolicy", ErrPolicyError},
		{"InvalidWebsiteConfiguration", ErrWebsiteError},
		{"InvalidCORSConfiguration", ErrCORSError},
		{"InvalidNotificationConfiguration", ErrNotificationError},
		{"InvalidLifecycleConfiguration", ErrLifecycleError},
		{"InvalidReplicationConfiguration", ErrReplicationError},
		{"ReplicationConfigurationNotFoundError", ErrReplicationError},
		{"InvalidTagError", ErrTaggingError},
		{"BadRequest", ErrInvalidArgument},
		{"RequestTimeout", ErrTimeout},
		{"EntityTooSmall", ErrPartTooSmall},
		{"IncompleteBody", ErrInvalidArgument},
		{"InvalidContentType", ErrInvalidContentType},
		{"KeyTooLong", ErrInvalidObjectName},
		{"MissingContentLength", ErrInvalidArgument},
		{"MissingRequestBodyError", ErrInvalidArgument},
		{"RequestTimeTooSkewed", ErrInvalidArgument},
		{"SlowDown", ErrTooManyRequests},
		{"TemporaryRedirect", ErrNetworkError},
		{"RequestHeaderSectionTooLarge", ErrInvalidArgument},
		{"TooManyRequests", ErrTooManyRequests},
		{"InternalError", ErrServerError},
		{"ServiceUnavailable", ErrServiceUnavailable},
		{"NotImplemented", ErrNotImplemented},
		{"BandwidthLimitExceeded", ErrTooManyRequests},
		{"ReducedRedundancyLostFraction", ErrServerError},
		{"InsufficientStorage", ErrQuotaExceeded},
		{"XNotImplemented", ErrNotImplemented},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()
			got := newClient().translateMinIOError(miniogo.ErrorResponse{Code: tc.code})
			assert.ErrorIs(t, got, tc.want, "code %s", tc.code)
		})
	}
}

func TestTranslateMinIOError_StatusCodeFallback(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status int
		want   error
	}{
		{400, ErrInvalidArgument},
		{401, ErrInvalidCredentials},
		{403, ErrAccessDenied},
		{404, ErrObjectNotFound},
		{409, ErrBucketAlreadyExists},
		{412, ErrPreconditionFailed},
		{413, ErrObjectTooLarge},
		{416, ErrInvalidRange},
		{429, ErrTooManyRequests},
		{500, ErrServerError},
		{501, ErrNotImplemented},
		{503, ErrServiceUnavailable},
		{999, ErrUnknownError},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			t.Parallel()
			got := newClient().translateMinIOError(miniogo.ErrorResponse{Code: "UnknownCode", StatusCode: tc.status})
			assert.ErrorIs(t, got, tc.want)
		})
	}
}

func TestTranslateURLError_AllOps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		op     string
		errMsg string
		want   error
	}{
		{"dial", "connection error", ErrConnectionFailed},
		{"read", "read error", ErrConnectionLost},
		{"write", "write error", ErrConnectionLost},
		{"other", "timeout occurred", ErrTimeout},
		{"other", "connection refused", ErrConnectionFailed},
		{"other", "connection reset by peer", ErrConnectionLost},
		{"other", "some network error", ErrNetworkError},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.op+"_"+tc.errMsg, func(t *testing.T) {
			t.Parallel()
			urlErr := &url.Error{Op: tc.op, URL: "http://test", Err: errors.New(tc.errMsg)}
			got := newClient().translateURLError(urlErr)
			assert.ErrorIs(t, got, tc.want)
		})
	}
}

func TestTranslateByErrorMessage_AllBranches(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("original")
	cases := []struct {
		msg  string
		want error
	}{
		{"connection refused", ErrConnectionFailed},
		{"connection reset by peer", ErrConnectionLost},
		{"connection timeout", ErrTimeout},
		{"connection failed", ErrConnectionFailed},
		{"network is unreachable", ErrNetworkError},
		{"no route to host", ErrNetworkError},
		{"host is down", ErrNetworkError},
		{"timeout exceeded", ErrTimeout},
		{"deadline exceeded", ErrTimeout},
		{"request timeout", ErrTimeout},
		{"client timeout", ErrTimeout},
		{"bucket does not exist", ErrBucketNotFound},
		{"bucket not found", ErrBucketNotFound},
		{"key does not exist", ErrObjectNotFound},
		{"object not found", ErrObjectNotFound},
		{"no such bucket", ErrBucketNotFound},
		{"no such key", ErrObjectNotFound},
		{"bucket already exists", ErrBucketAlreadyExists},
		{"bucket not empty", ErrBucketNotEmpty},
		{"access denied by policy", ErrAccessDenied},
		{"forbidden action", ErrAccessDenied},
		{"invalid credentials provided", ErrInvalidCredentials},
		{"unauthorized request", ErrInvalidCredentials},
		{"quota exceeded for account", ErrQuotaExceeded},
		{"insufficient storage available", ErrQuotaExceeded},
		{"entity too large to upload", ErrObjectTooLarge},
		{"file too large to upload", ErrObjectTooLarge},
		{"invalid bucket name provided", ErrInvalidBucketName},
		{"invalid object name provided", ErrInvalidObjectName},
		{"invalid argument provided", ErrInvalidArgument},
		{"bad request from client", ErrInvalidArgument},
		{"server error in processing", ErrServerError},
		{"internal server error occurred", ErrServerError},
		{"service unavailable at this time", ErrServiceUnavailable},
		{"object locked by retention", ErrObjectLocked},
		{"versioning not enabled on bucket", ErrVersioningNotEnabled},
		{"invalid version id", ErrInvalidVersion},
		{"no such version present", ErrInvalidVersion},
		{"operation cancelled by user", ErrCancelled},
		{"context canceled by caller", ErrCancelled},
		{"operation was canceled manually", ErrCancelled},
		{"context was cancelled by caller", ErrCancelled},
		{"signature mismatch error", ErrInvalidCredentials},
		{"invalid access key provided", ErrInvalidCredentials},
		{"expired token found", ErrCredentialsExpired},
		{"token expired at midnight", ErrCredentialsExpired},
		{"invalid checksum found", ErrInvalidChecksum},
		{"bad digest value", ErrInvalidChecksum},
		{"invalid range specified", ErrInvalidRange},
		{"invalid part number", ErrInvalidPart},
		{"part too small to upload", ErrPartTooSmall},
		{"invalid upload id provided", ErrInvalidUploadID},
		{"malformed xml body", ErrMalformedXML},
		{"invalid xml syntax", ErrMalformedXML},
		{"bad gateway error", ErrServerError},
		{"gateway timeout occurred", ErrTimeout},
		{"too many requests rate limit", ErrTooManyRequests},
		{"rate limit exceeded", ErrTooManyRequests},
		{"slow down rate limit", ErrTooManyRequests},
		{"invalid key name given", ErrInvalidObjectName},
		{"invalid request body", ErrInvalidArgument},
		{"storage quota exceeded", ErrQuotaExceeded},
		{"retention policy violation", ErrRetentionPolicyError},
		{"lock configuration error found", ErrLockConfigurationError},
		{"some completely unknown error message xyz", sentinel},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			got := newClient().translateByErrorMessage(tc.msg, sentinel)
			assert.ErrorIs(t, got, tc.want)
		})
	}
}

// ── GetErrorCategory ──────────────────────────────────────────────────────────

func TestGetErrorCategory(t *testing.T) {
	t.Parallel()
	m := newClient()
	cases := []struct {
		err  error
		want ErrorCategory
	}{
		{ErrConnectionFailed, CategoryConnection},
		{ErrConnectionLost, CategoryConnection},
		{ErrInvalidCredentials, CategoryAuthentication},
		{ErrCredentialsExpired, CategoryAuthentication},
		{ErrAccessDenied, CategoryPermission},
		{ErrInsufficientPermissions, CategoryPermission},
		{ErrObjectNotFound, CategoryNotFound},
		{ErrBucketNotFound, CategoryNotFound},
		{ErrBucketAlreadyExists, CategoryConflict},
		{ErrBucketNotEmpty, CategoryConflict},
		{ErrInvalidBucketName, CategoryValidation},
		{ErrInvalidObjectName, CategoryValidation},
		{ErrInvalidArgument, CategoryValidation},
		{ErrInvalidRange, CategoryValidation},
		{ErrInvalidChecksum, CategoryValidation},
		{ErrInvalidMetadata, CategoryValidation},
		{ErrInvalidContentType, CategoryValidation},
		{ErrMalformedXML, CategoryValidation},
		{ErrServerError, CategoryServer},
		{ErrServiceUnavailable, CategoryServer},
		{ErrInvalidResponse, CategoryServer},
		{ErrNetworkError, CategoryNetwork},
		{ErrInvalidURL, CategoryNetwork},
		{ErrTimeout, CategoryTimeout},
		{ErrQuotaExceeded, CategoryQuota},
		{ErrObjectTooLarge, CategoryQuota},
		{ErrConfigurationError, CategoryConfiguration},
		{ErrInvalidStorageClass, CategoryConfiguration},
		{ErrVersioningNotEnabled, CategoryVersioning},
		{ErrInvalidVersion, CategoryVersioning},
		{ErrObjectLocked, CategoryLocking},
		{ErrLockConfigurationError, CategoryLocking},
		{ErrRetentionPolicyError, CategoryLocking},
		{ErrNotImplemented, CategoryOperation},
		{ErrCancelled, CategoryOperation},
		{errors.New("random"), CategoryUnknown},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, m.GetErrorCategory(tc.err))
		})
	}
}

// ── IsRetryableError / IsTemporaryError / IsPermanentError ───────────────────

func TestIsRetryableError(t *testing.T) {
	t.Parallel()
	m := newClient()
	assert.True(t, m.IsRetryableError(ErrConnectionFailed))
	assert.True(t, m.IsRetryableError(ErrConnectionLost))
	assert.True(t, m.IsRetryableError(ErrTimeout))
	assert.True(t, m.IsRetryableError(ErrNetworkError))
	assert.True(t, m.IsRetryableError(ErrServerError))
	assert.True(t, m.IsRetryableError(ErrServiceUnavailable))
	assert.True(t, m.IsRetryableError(ErrTooManyRequests))
	assert.True(t, m.IsRetryableError(ErrInvalidResponse))
	assert.False(t, m.IsRetryableError(ErrObjectNotFound))
	assert.False(t, m.IsRetryableError(ErrAccessDenied))
}

func TestIsTemporaryError(t *testing.T) {
	t.Parallel()
	m := newClient()
	assert.True(t, m.IsTemporaryError(ErrConnectionFailed))
	assert.True(t, m.IsTemporaryError(ErrCredentialsExpired))
	assert.False(t, m.IsTemporaryError(ErrObjectNotFound))
}

func TestIsPermanentError(t *testing.T) {
	t.Parallel()
	m := newClient()
	assert.True(t, m.IsPermanentError(ErrObjectNotFound))
	assert.True(t, m.IsPermanentError(ErrBucketNotFound))
	assert.True(t, m.IsPermanentError(ErrInvalidCredentials))
	assert.True(t, m.IsPermanentError(ErrAccessDenied))
	assert.False(t, m.IsPermanentError(ErrConnectionFailed))
	assert.False(t, m.IsPermanentError(errors.New("unknown")))
}

// ── options ───────────────────────────────────────────────────────────────────

func TestPutOptions(t *testing.T) {
	t.Parallel()
	opts := &PutOptions{}
	WithSize(1234)(opts)
	WithContentType("application/json")(opts)
	WithMetadata(map[string]string{"k": "v"})(opts)
	WithPartSize(10 * 1024 * 1024)(opts)
	assert.Equal(t, int64(1234), opts.Size)
	assert.Equal(t, "application/json", opts.ContentType)
	assert.Equal(t, map[string]string{"k": "v"}, opts.Metadata)
	assert.Equal(t, uint64(10*1024*1024), opts.PartSize)
}

func TestGetOptions(t *testing.T) {
	t.Parallel()
	opts := &GetOptions{}
	WithVersionID("v123")(opts)
	assert.Equal(t, "v123", opts.VersionID)

	WithByteRange(0, 1023)(opts)
	require.NotNil(t, opts.Range)
	assert.Equal(t, int64(0), opts.Range.Start)
	assert.Equal(t, int64(1023), opts.Range.End)
}

func TestBucketOptions(t *testing.T) {
	t.Parallel()
	opts := &BucketOptions{}
	WithRegion("us-east-1")(opts)
	WithObjectLocking(true)(opts)
	assert.Equal(t, "us-east-1", opts.Region)
	assert.True(t, opts.ObjectLocking)
}

// ── multipartPresignedGetImpl getters ─────────────────────────────────────────

func TestMultipartPresignedGetImpl_Getters(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(time.Hour).Unix()
	info := &MultipartPresignedGetInfo{
		ObjectKey:     "my/key",
		PresignedUrls: []string{"https://url1", "https://url2"},
		PartRanges:    []string{"bytes=0-999", "bytes=1000-1999"},
		ExpiresAt:     future,
		TotalSize:     2000,
		ContentType:   "image/png",
		ETag:          "abc123",
	}
	m := newMultipartPresignedGet(info)

	assert.Equal(t, "my/key", m.GetObjectKey())
	assert.Equal(t, []string{"https://url1", "https://url2"}, m.GetPresignedURLs())
	assert.Equal(t, []string{"bytes=0-999", "bytes=1000-1999"}, m.GetPartRanges())
	assert.Equal(t, future, m.GetExpiryTimestamp())
	assert.Equal(t, int64(2000), m.GetTotalSize())
	assert.Equal(t, "image/png", m.GetContentType())
	assert.Equal(t, "abc123", m.GetETag())
	assert.False(t, m.IsExpired())

	// Expired
	info2 := &MultipartPresignedGetInfo{ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	m2 := newMultipartPresignedGet(info2)
	assert.True(t, m2.IsExpired())

	// URLs/ranges are copies
	urls := m.GetPresignedURLs()
	urls[0] = "modified"
	assert.Equal(t, "https://url1", m.GetPresignedURLs()[0])
}

// ── CompleteMultipartUpload early returns ─────────────────────────────────────

func TestCompleteMultipartUpload_NoParts(t *testing.T) {
	t.Parallel()
	err := newClient().CompleteMultipartUpload(context.Background(), "bucket", "key", "uid", nil, nil)
	assert.Error(t, err)
}

func TestCompleteMultipartUpload_MismatchedParts(t *testing.T) {
	t.Parallel()
	err := newClient().CompleteMultipartUpload(context.Background(), "bucket", "key", "uid", []int{1, 2}, []string{"etag1"})
	assert.Error(t, err)
}

func TestCompleteMultipartUpload_NilCoreClient(t *testing.T) {
	t.Parallel()
	err := newClient().CompleteMultipartUpload(context.Background(), "bucket", "key", "uid", []int{1}, []string{"etag1"})
	assert.ErrorIs(t, err, ErrConnectionFailed)
}

// ── multipartUploadImpl getters ───────────────────────────────────────────────

func TestMultipartUploadImpl_Getters(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(time.Hour).Unix()
	info := &MultipartUploadInfo{
		ExpiresAt:           future,
		RecommendedPartSize: 5 * 1024 * 1024,
		MaxParts:            10000,
		ContentType:         "application/octet-stream",
		TotalSize:           100 * 1024 * 1024,
	}
	m := newMultipartUpload(info)

	assert.Equal(t, future, m.GetExpiryTimestamp())
	assert.Equal(t, int64(5*1024*1024), m.GetRecommendedPartSize())
	assert.Equal(t, 10000, m.GetMaxParts())
	assert.Equal(t, "application/octet-stream", m.GetContentType())
	assert.Equal(t, int64(100*1024*1024), m.GetTotalSize())
	assert.False(t, m.IsExpired())

	info2 := &MultipartUploadInfo{ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	m2 := newMultipartUpload(info2)
	assert.True(t, m2.IsExpired())
}
