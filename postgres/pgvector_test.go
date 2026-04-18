package postgres

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
)

// VectorDoc is a minimal model used by the pgvector integration tests.
type VectorDoc struct {
	gorm.Model
	Name      string
	Embedding Vector `gorm:"type:vector(3)"`
}

// SparseDoc exercises the sparsevec column type.
type SparseDoc struct {
	gorm.Model
	Name     string
	Features SparseVector `gorm:"type:sparsevec(10)"`
}

var (
	pgvectorOnce      sync.Once
	pgvectorContainer *PostgresContainer
	pgvectorErr       error
)

// pgvectorSetup starts a pgvector-capable Postgres container once per test
// process, enables the extension, and returns a *Postgres bound to it.
// Tests share the container; each test creates and drops its own tables.
func pgvectorSetup(t *testing.T) *Postgres {
	t.Helper()
	pgvectorOnce.Do(func() {
		pgvectorContainer, pgvectorErr = setupPGVectorContainer(context.Background())
	})
	require.NoError(t, pgvectorErr, "pgvector container failed to start")
	require.NotNil(t, pgvectorContainer)

	pg, err := NewPostgres(pgvectorContainer.Config)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.GracefulShutdown() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, pg.EnableVectorExtension(ctx))
	return pg
}

func setupPGVectorContainer(ctx context.Context) (*PostgresContainer, error) {
	port, err := getFreePort()
	if err != nil {
		return nil, fmt.Errorf("could not get free port: %w", err)
	}
	portStr := fmt.Sprintf("%d", port)
	portBindings := nat.PortMap{
		"5432/tcp": []nat.PortBinding{{HostPort: portStr}},
	}

	req := testcontainers.ContainerRequest{
		Image: "pgvector/pgvector:pg16",
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		ExposedPorts: []string{"5432/tcp"},
		HostConfigModifier: func(cfg *container.HostConfig) {
			cfg.PortBindings = portBindings
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start pgvector container: %w", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, fmt.Errorf("failed to get host: %w", err)
	}
	mapped, err := c.MappedPort(ctx, "5432")
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}
	portStr = mapped.Port()

	if err := waitForPostgresReady(host, portStr, "testuser", "testpass", "testdb", 30*time.Second); err != nil {
		_ = c.Terminate(ctx)
		return nil, fmt.Errorf("pgvector container not ready: %w", err)
	}

	return &PostgresContainer{
		Container: c,
		Config: Config{
			Connection: Connection{
				Host:     host,
				Port:     portStr,
				User:     "testuser",
				Password: "testpass",
				DbName:   "testdb",
				SSLMode:  "disable",
			},
		},
		Host: host,
		Port: portStr,
	}, nil
}

func TestEnableVectorExtension_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pg := pgvectorSetup(t)

	// Second call should be a no-op since pgvectorSetup already enabled it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, pg.EnableVectorExtension(ctx))

	var installed bool
	require.NoError(t, pg.DB().Raw(
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = ?)", "vector",
	).Scan(&installed).Error)
	assert.True(t, installed, "vector extension should be installed")
}

func TestVector_InsertAndNearestNeighbor(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pg := pgvectorSetup(t)
	db := pg.DB()

	require.NoError(t, db.Migrator().DropTable(&VectorDoc{}))
	require.NoError(t, db.AutoMigrate(&VectorDoc{}))
	t.Cleanup(func() { _ = db.Migrator().DropTable(&VectorDoc{}) })

	ctx := context.Background()
	docs := []VectorDoc{
		{Name: "a", Embedding: NewVector([]float32{1, 0, 0})},
		{Name: "b", Embedding: NewVector([]float32{0, 1, 0})},
		{Name: "c", Embedding: NewVector([]float32{0, 0, 1})},
	}
	for i := range docs {
		require.NoError(t, pg.Create(ctx, &docs[i]))
	}

	// Query closest to (0.9, 0.1, 0) using L2 — expect "a" first.
	target := NewVector([]float32{0.9, 0.1, 0})
	var got []VectorDoc
	err := pg.DB().WithContext(ctx).
		Order(gorm.Expr("embedding "+DistanceL2+" ?", target)).
		Limit(3).
		Find(&got).Error
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "a", got[0].Name)
}

func TestSparseVector_InsertAndNearestNeighbor(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pg := pgvectorSetup(t)
	db := pg.DB()

	require.NoError(t, db.Migrator().DropTable(&SparseDoc{}))
	require.NoError(t, db.AutoMigrate(&SparseDoc{}))
	t.Cleanup(func() { _ = db.Migrator().DropTable(&SparseDoc{}) })

	ctx := context.Background()
	docs := []SparseDoc{
		{Name: "a", Features: NewSparseVectorFromMap(map[int32]float32{0: 1, 4: 0.5}, 10)},
		{Name: "b", Features: NewSparseVectorFromMap(map[int32]float32{7: 1}, 10)},
		{Name: "c", Features: NewSparseVectorFromMap(map[int32]float32{0: 0.1, 9: 1}, 10)},
	}
	for i := range docs {
		require.NoError(t, pg.Create(ctx, &docs[i]))
	}

	target := NewSparseVectorFromMap(map[int32]float32{0: 1}, 10)
	var got []SparseDoc
	err := pg.DB().WithContext(ctx).
		Order(gorm.Expr("features "+DistanceCosine+" ?", target)).
		Limit(3).
		Find(&got).Error
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "a", got[0].Name, "doc with dimension 0 active should be closest by cosine")
}

func TestCreateHNSWIndex_Vector(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pg := pgvectorSetup(t)
	db := pg.DB()

	require.NoError(t, db.Migrator().DropTable(&VectorDoc{}))
	require.NoError(t, db.AutoMigrate(&VectorDoc{}))
	t.Cleanup(func() { _ = db.Migrator().DropTable(&VectorDoc{}) })

	ctx := context.Background()
	err := pg.CreateHNSWIndex(ctx, "vector_docs", "embedding", VectorCosineOps, HNSWOptions{
		M:              8,
		EfConstruction: 32,
		Name:           "idx_vector_docs_embedding_hnsw",
	})
	require.NoError(t, err)

	var exists bool
	require.NoError(t, db.Raw(
		"SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = ?)",
		"idx_vector_docs_embedding_hnsw",
	).Scan(&exists).Error)
	assert.True(t, exists, "HNSW index should exist")
}

func TestCreateHNSWIndex_SparseVector(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pg := pgvectorSetup(t)
	db := pg.DB()

	require.NoError(t, db.Migrator().DropTable(&SparseDoc{}))
	require.NoError(t, db.AutoMigrate(&SparseDoc{}))
	t.Cleanup(func() { _ = db.Migrator().DropTable(&SparseDoc{}) })

	ctx := context.Background()
	err := pg.CreateHNSWIndex(ctx, "sparse_docs", "features", SparseVectorInnerProductOps, HNSWOptions{
		Name: "idx_sparse_docs_features_hnsw",
	})
	require.NoError(t, err)

	var exists bool
	require.NoError(t, db.Raw(
		"SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = ?)",
		"idx_sparse_docs_features_hnsw",
	).Scan(&exists).Error)
	assert.True(t, exists)
}

func TestCreateIVFFlatIndex_Vector(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	pg := pgvectorSetup(t)
	db := pg.DB()

	require.NoError(t, db.Migrator().DropTable(&VectorDoc{}))
	require.NoError(t, db.AutoMigrate(&VectorDoc{}))
	t.Cleanup(func() { _ = db.Migrator().DropTable(&VectorDoc{}) })

	// Seed a few rows — IVFFlat requires non-empty data to build sensibly,
	// but pgvector allows building on empty tables. Test both paths would be
	// over-scope; just verify creation succeeds with explicit Lists.
	ctx := context.Background()
	err := pg.CreateIVFFlatIndex(ctx, "vector_docs", "embedding", VectorL2Ops, IVFFlatOptions{
		Lists: 1,
		Name:  "idx_vector_docs_embedding_ivf",
	})
	require.NoError(t, err)

	var exists bool
	require.NoError(t, db.Raw(
		"SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = ?)",
		"idx_vector_docs_embedding_ivf",
	).Scan(&exists).Error)
	assert.True(t, exists)
}

func TestCreateIVFFlatIndex_RejectsZeroLists(t *testing.T) {
	pg := &Postgres{} // no DB connection needed — validation runs before SQL
	err := pg.CreateIVFFlatIndex(context.Background(), "t", "c", VectorL2Ops, IVFFlatOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Lists must be > 0")
}

func TestBuildVectorIndexSQL(t *testing.T) {
	cases := []struct {
		name         string
		method       string
		table        string
		column       string
		opClass      string
		indexName    string
		concurrently bool
		with         string
		want         string
	}{
		{
			name: "hnsw_basic",
			method: "hnsw", table: "docs", column: "embedding",
			opClass: VectorCosineOps,
			want:    `CREATE INDEX ON "docs" USING hnsw ("embedding" vector_cosine_ops)`,
		},
		{
			name: "hnsw_with_name_and_opts",
			method: "hnsw", table: "docs", column: "embedding",
			opClass: VectorL2Ops, indexName: "my_idx",
			with: "m = 16, ef_construction = 64",
			want: `CREATE INDEX "my_idx" ON "docs" USING hnsw ("embedding" vector_l2_ops) WITH (m = 16, ef_construction = 64)`,
		},
		{
			name: "ivfflat_concurrently",
			method: "ivfflat", table: "docs", column: "emb",
			opClass: VectorL2Ops, concurrently: true, with: "lists = 100",
			want: `CREATE INDEX CONCURRENTLY ON "docs" USING ivfflat ("emb" vector_l2_ops) WITH (lists = 100)`,
		},
		{
			name: "quotes_escaped_in_identifiers",
			method: "hnsw", table: `weird"name`, column: "c",
			opClass: VectorL2Ops,
			want:    `CREATE INDEX ON "weird""name" USING hnsw ("c" vector_l2_ops)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := buildVectorIndexSQL(tc.method, tc.table, tc.column, tc.opClass, tc.indexName, tc.concurrently, tc.with)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHNSWWithClause(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", hnswWithClause(HNSWOptions{}))
	assert.Equal(t, "m = 32", hnswWithClause(HNSWOptions{M: 32}))
	assert.Equal(t, "ef_construction = 128", hnswWithClause(HNSWOptions{EfConstruction: 128}))
	assert.Equal(t, "m = 16, ef_construction = 64", hnswWithClause(HNSWOptions{M: 16, EfConstruction: 64}))
}
