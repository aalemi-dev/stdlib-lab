package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pgvector/pgvector-go"
)

// Vector is a dense float32 vector mapped to pgvector's `vector` column type.
// It implements database/sql.Scanner and driver.Valuer, so GORM reads and
// writes it transparently when the struct field is tagged with the column type.
//
// Example:
//
//	type Document struct {
//	    ID        uint
//	    Embedding postgres.Vector `gorm:"type:vector(1536)"`
//	}
//
//	doc := Document{Embedding: postgres.NewVector([]float32{0.1, 0.2, 0.3, ...})}
type Vector = pgvector.Vector

// SparseVector is a sparse float32 vector mapped to pgvector's `sparsevec`
// column type. Use it when most dimensions are zero (e.g. SPLADE-style
// hybrid-search models over large vocabularies).
//
// Example:
//
//	type Document struct {
//	    ID       uint
//	    Features postgres.SparseVector `gorm:"type:sparsevec(30000)"`
//	}
//
//	feats := postgres.NewSparseVectorFromMap(map[int32]float32{42: 0.8, 1337: 0.5}, 30000)
type SparseVector = pgvector.SparseVector

// NewVector constructs a dense Vector from a []float32.
func NewVector(v []float32) Vector { return pgvector.NewVector(v) }

// NewSparseVector constructs a SparseVector by compressing a dense []float32,
// keeping only non-zero entries. Dimension is inferred from len(v).
func NewSparseVector(v []float32) SparseVector { return pgvector.NewSparseVector(v) }

// NewSparseVectorFromMap constructs a SparseVector from a map of index to value.
// Use this when the input is already sparse and you know the total dimension.
func NewSparseVectorFromMap(elements map[int32]float32, dim int32) SparseVector {
	return pgvector.NewSparseVectorFromMap(elements, dim)
}

// Distance operators used in similarity queries: ORDER BY column <op> target.
//
// The operator you choose must match the operator class of any index on the
// column — otherwise Postgres silently falls back to a sequential scan and
// the index provides no benefit.
const (
	// DistanceL2 is Euclidean distance. Pairs with VectorL2Ops / SparseVectorL2Ops.
	DistanceL2 = "<->"
	// DistanceInnerProduct is negative inner product. Pairs with VectorInnerProductOps /
	// SparseVectorInnerProductOps. Fastest when vectors are unit-normalized.
	DistanceInnerProduct = "<#>"
	// DistanceCosine is cosine distance. Pairs with VectorCosineOps / SparseVectorCosineOps.
	// Common default for unnormalized LLM embeddings.
	DistanceCosine = "<=>"
	// DistanceL1 is Manhattan distance. Pairs with VectorL1Ops.
	// Requires pgvector 0.7+. Not supported for sparsevec.
	DistanceL1 = "<+>"
)

// Operator classes for indexing dense `vector` columns.
// The class must match the distance operator used at query time.
const (
	VectorL2Ops           = "vector_l2_ops"
	VectorInnerProductOps = "vector_ip_ops"
	VectorCosineOps       = "vector_cosine_ops"
	VectorL1Ops           = "vector_l1_ops" // HNSW only; pgvector 0.7+
)

// Operator classes for indexing `sparsevec` columns.
// Only HNSW indexes support sparsevec — IVFFlat does not.
const (
	SparseVectorL2Ops           = "sparsevec_l2_ops"
	SparseVectorInnerProductOps = "sparsevec_ip_ops"
	SparseVectorCosineOps       = "sparsevec_cosine_ops"
)

// EnableVectorExtension installs the pgvector extension in the connected
// database (CREATE EXTENSION IF NOT EXISTS vector).
//
// The connecting role must hold CREATE privilege on the database. In managed
// Postgres this is typically a one-off DBA task, not something the application
// runs at startup — call this from a setup script or migration, not from a
// request handler.
func (p *Postgres) EnableVectorExtension(ctx context.Context) error {
	start := time.Now()
	err := p.DB().WithContext(ctx).Exec("CREATE EXTENSION IF NOT EXISTS vector").Error
	p.observeOperation("enable_extension", "", "", time.Since(start), err, 0, map[string]interface{}{
		"extension": "vector",
	})
	return err
}

// HNSWOptions configures an HNSW index build. Zero values defer to pgvector's
// own defaults (m=16, ef_construction=64).
type HNSWOptions struct {
	// M is the max number of connections per layer. Higher improves recall
	// at the cost of memory and build time.
	M int
	// EfConstruction is the size of the dynamic candidate list during build.
	// Higher improves recall at the cost of build time.
	EfConstruction int
	// Name overrides the auto-generated index name.
	Name string
	// Concurrently issues CREATE INDEX CONCURRENTLY. Cannot run inside a
	// transaction but avoids blocking writes during the build.
	Concurrently bool
}

// IVFFlatOptions configures an IVFFlat index build. Lists is required.
type IVFFlatOptions struct {
	// Lists is the number of inverted lists to partition vectors into.
	// pgvector guidance: rows/1000 up to 1M rows, sqrt(rows) above that.
	Lists int
	// Name overrides the auto-generated index name.
	Name string
	// Concurrently issues CREATE INDEX CONCURRENTLY.
	Concurrently bool
}

// CreateHNSWIndex creates an HNSW index on (table, column) using the given
// operator class. The class must match the distance operator used at query
// time — e.g. VectorCosineOps for `ORDER BY col <=> ?`, or
// SparseVectorInnerProductOps for sparsevec with `<#>`.
//
// HNSW supports both `vector` and `sparsevec` columns (with their respective
// op classes). For pure dense workloads IVFFlat is also an option.
func (p *Postgres) CreateHNSWIndex(ctx context.Context, table, column, opClass string, opts HNSWOptions) error {
	start := time.Now()
	sql := buildVectorIndexSQL("hnsw", table, column, opClass, opts.Name, opts.Concurrently, hnswWithClause(opts))
	err := p.DB().WithContext(ctx).Exec(sql).Error
	p.observeOperation("create_hnsw_index", table, column, time.Since(start), err, 0, map[string]interface{}{
		"op_class":        opClass,
		"m":               opts.M,
		"ef_construction": opts.EfConstruction,
		"concurrently":    opts.Concurrently,
	})
	return err
}

// CreateIVFFlatIndex creates an IVFFlat index on (table, column) using the
// given operator class. IVFFlat does not support sparsevec — use
// CreateHNSWIndex for sparse columns.
func (p *Postgres) CreateIVFFlatIndex(ctx context.Context, table, column, opClass string, opts IVFFlatOptions) error {
	if opts.Lists <= 0 {
		return fmt.Errorf("postgres: IVFFlatOptions.Lists must be > 0")
	}
	start := time.Now()
	sql := buildVectorIndexSQL("ivfflat", table, column, opClass, opts.Name, opts.Concurrently,
		fmt.Sprintf("lists = %d", opts.Lists))
	err := p.DB().WithContext(ctx).Exec(sql).Error
	p.observeOperation("create_ivfflat_index", table, column, time.Since(start), err, 0, map[string]interface{}{
		"op_class":     opClass,
		"lists":        opts.Lists,
		"concurrently": opts.Concurrently,
	})
	return err
}

func hnswWithClause(opts HNSWOptions) string {
	var parts []string
	if opts.M > 0 {
		parts = append(parts, fmt.Sprintf("m = %d", opts.M))
	}
	if opts.EfConstruction > 0 {
		parts = append(parts, fmt.Sprintf("ef_construction = %d", opts.EfConstruction))
	}
	return strings.Join(parts, ", ")
}

func buildVectorIndexSQL(method, table, column, opClass, name string, concurrently bool, withClause string) string {
	var b strings.Builder
	b.WriteString("CREATE INDEX ")
	if concurrently {
		b.WriteString("CONCURRENTLY ")
	}
	if name != "" {
		b.WriteString(quoteIdent(name))
		b.WriteByte(' ')
	}
	b.WriteString("ON ")
	b.WriteString(quoteIdent(table))
	b.WriteString(" USING ")
	b.WriteString(method)
	b.WriteString(" (")
	b.WriteString(quoteIdent(column))
	b.WriteByte(' ')
	b.WriteString(opClass)
	b.WriteByte(')')
	if withClause != "" {
		b.WriteString(" WITH (")
		b.WriteString(withClause)
		b.WriteByte(')')
	}
	return b.String()
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
