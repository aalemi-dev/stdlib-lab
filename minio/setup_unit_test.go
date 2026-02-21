package minio

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── DefaultConnectionPoolConfig ───────────────────────────────────────────────

func TestDefaultConnectionPoolConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConnectionPoolConfig()
	assert.Equal(t, 50, cfg.MaxIdleConnections)
	assert.Equal(t, 100, cfg.MaxConnectionsPerHost)
	assert.Equal(t, 90*time.Second, cfg.IdleConnectionTimeout)
	assert.Equal(t, 30*time.Second, cfg.ConnectionTimeout)
}

// ── DefaultBufferPoolConfig ───────────────────────────────────────────────────

func TestDefaultBufferPoolConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultBufferPoolConfig()
	assert.Equal(t, 32*1024*1024, cfg.MaxBufferSize)
	assert.Equal(t, 100, cfg.MaxPoolSize)
	assert.Equal(t, 64*1024, cfg.InitialBufferSize)
}

// ── BufferPool ────────────────────────────────────────────────────────────────

func TestBufferPool_GetPut(t *testing.T) {
	t.Parallel()
	bp := NewBufferPool()

	// Get allocates a new buffer
	buf := bp.Get()
	require.NotNil(t, buf)
	buf.WriteString("hello")
	bp.Put(buf)

	// A second Get may return the recycled buffer
	buf2 := bp.Get()
	require.NotNil(t, buf2)
	bp.Put(buf2)
}

func TestBufferPool_PutNil(t *testing.T) {
	t.Parallel()
	bp := NewBufferPool()
	// Should not panic when given nil
	bp.Put(nil)
}

func TestBufferPool_PutOversizedBuffer(t *testing.T) {
	t.Parallel()
	cfg := BufferPoolConfig{MaxBufferSize: 10, MaxPoolSize: 5, InitialBufferSize: 4}
	bp := NewBufferPoolWithConfig(cfg)

	big := bytes.NewBuffer(make([]byte, 0, 100))
	bp.Put(big) // should be discarded silently

	stats := bp.GetStats()
	assert.Equal(t, int64(1), stats.TotalBuffersDiscarded)
}

func TestBufferPool_PutWhenFull(t *testing.T) {
	t.Parallel()
	cfg := BufferPoolConfig{MaxBufferSize: 1024, MaxPoolSize: 1, InitialBufferSize: 8}
	bp := NewBufferPoolWithConfig(cfg)

	b1 := bp.Get()
	b2 := bp.Get()
	bp.Put(b1) // pool now "full" (size=1 after this)
	bp.Put(b2) // should be discarded
}

func TestBufferPool_GetStats(t *testing.T) {
	t.Parallel()
	bp := NewBufferPool()

	buf := bp.Get()
	bp.Put(buf)
	_ = bp.Get()

	stats := bp.GetStats()
	assert.GreaterOrEqual(t, stats.TotalBuffersCreated, int64(1))
}

func TestBufferPool_GetStats_ReuseRatio_Zero(t *testing.T) {
	t.Parallel()
	// Created=0 means reuse ratio should be 0 (no division by zero)
	bp := NewBufferPoolWithConfig(BufferPoolConfig{
		MaxBufferSize:     1024 * 1024,
		MaxPoolSize:       10,
		InitialBufferSize: 64,
	})
	stats := bp.GetStats()
	assert.Equal(t, float64(0), stats.ReuseRatio)
}

func TestBufferPool_Cleanup(t *testing.T) {
	t.Parallel()
	bp := NewBufferPool()
	buf := bp.Get()
	bp.Put(buf)
	bp.Cleanup() // should not panic
}

// ── WithLogger ────────────────────────────────────────────────────────────────

type noopLogger struct{}

func (n *noopLogger) InfoWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
}
func (n *noopLogger) WarnWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
}
func (n *noopLogger) ErrorWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
}

func TestWithLogger(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	logger := &noopLogger{}
	out := m.WithLogger(logger)
	assert.Equal(t, m, out)
	assert.Equal(t, logger, m.logger)
}

// ── GetBufferPoolStats ────────────────────────────────────────────────────────

func TestGetBufferPoolStats_NilPool(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	stats := m.GetBufferPoolStats()
	assert.Equal(t, BufferPoolStats{}, stats)
}

func TestGetBufferPoolStats_WithPool(t *testing.T) {
	t.Parallel()
	bp := NewBufferPool()
	buf := bp.Get()
	bp.Put(buf)

	m := &MinioClient{bufferPool: bp}
	stats := m.GetBufferPoolStats()
	assert.GreaterOrEqual(t, stats.TotalBuffersCreated, int64(1))
}

// ── CleanupResources ──────────────────────────────────────────────────────────

func TestCleanupResources_NilPool(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	// Must not panic
	m.CleanupResources()
}

func TestCleanupResources_WithPool(t *testing.T) {
	t.Parallel()
	bp := NewBufferPool()
	m := &MinioClient{bufferPool: bp}
	m.CleanupResources() // must not panic
}

// ── logInfo / logWarn / logError ──────────────────────────────────────────────

type captureLogger struct {
	infoCalled  bool
	warnCalled  bool
	errorCalled bool
}

func (c *captureLogger) InfoWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	c.infoCalled = true
}
func (c *captureLogger) WarnWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	c.warnCalled = true
}
func (c *captureLogger) ErrorWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	c.errorCalled = true
}

func TestLogMethods_WithLogger(t *testing.T) {
	t.Parallel()
	logger := &captureLogger{}
	m := &MinioClient{logger: logger}

	m.logInfo(context.Background(), "info msg", nil)
	m.logWarn(context.Background(), "warn msg", nil)
	m.logError(context.Background(), "error msg", nil)

	assert.True(t, logger.infoCalled)
	assert.True(t, logger.warnCalled)
	assert.True(t, logger.errorCalled)
}

func TestLogMethods_NoLogger(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	// must not panic
	m.logInfo(context.Background(), "info msg", nil)
	m.logWarn(context.Background(), "warn msg", nil)
	m.logError(context.Background(), "error msg", nil)
}

// ── calculateOptimalPartSize ──────────────────────────────────────────────────

func TestCalculateOptimalPartSize(t *testing.T) {
	t.Parallel()
	m := &MinioClient{cfg: Config{}}

	cases := []struct {
		name      string
		fileSize  int64
		wantParts int
	}{
		{"small file 1MB", 1 * 1024 * 1024, 1},
		{"medium file 100MB", 100 * 1024 * 1024, 20},
		{"large file 1GB", 1024 * 1024 * 1024, 200},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			partSize, partCount := m.calculateOptimalPartSize(tc.fileSize)
			assert.Greater(t, partSize, int64(0))
			assert.Greater(t, partCount, 0)
			// All parts together must cover the full file
			assert.GreaterOrEqual(t, partSize*int64(partCount), tc.fileSize)
		})
	}
}

func TestCalculateOptimalPartSize_CustomMinPartSize(t *testing.T) {
	t.Parallel()
	customMin := uint64(10 * 1024 * 1024) // 10 MiB
	m := &MinioClient{
		cfg: Config{UploadConfig: UploadConfig{MinPartSize: customMin}},
	}
	partSize, partCount := m.calculateOptimalPartSize(50 * 1024 * 1024)
	assert.GreaterOrEqual(t, partSize, int64(customMin))
	assert.Greater(t, partCount, 0)
}

// ── BucketClient (delegation wiring) ─────────────────────────────────────────

func TestMinioClient_Bucket(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	bc := m.Bucket("my-bucket")
	require.NotNil(t, bc)
	assert.Equal(t, "my-bucket", bc.bucket)
	assert.Equal(t, m, bc.client)
}

// ── BucketExists early return (nil client) ────────────────────────────────────

func TestBucketExists_EmptyName(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	_, err := m.BucketExists(context.Background(), "")
	assert.Error(t, err)
}

func TestBucketExists_NilClient(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	_, err := m.BucketExists(context.Background(), "test-bucket")
	assert.ErrorIs(t, err, ErrConnectionFailed)
}

// ── ListBuckets early return (nil client) ─────────────────────────────────────

func TestListBuckets_NilClient(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	_, err := m.ListBuckets(context.Background())
	assert.ErrorIs(t, err, ErrConnectionFailed)
}

// ── DeleteBucket early returns ────────────────────────────────────────────────

func TestDeleteBucket_EmptyName(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	err := m.DeleteBucket(context.Background(), "")
	assert.Error(t, err)
}

func TestDeleteBucket_NilClient(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	err := m.DeleteBucket(context.Background(), "my-bucket")
	assert.ErrorIs(t, err, ErrConnectionFailed)
}

// ── BucketClient.CompleteMultipartUpload delegation ───────────────────────────

func TestBucketClient_CompleteMultipartUpload_NoParts(t *testing.T) {
	t.Parallel()
	m := &MinioClient{}
	bc := m.Bucket("test-bucket")
	err := bc.CompleteMultipartUpload(context.Background(), "key", "uid", nil, nil)
	assert.Error(t, err)
}
