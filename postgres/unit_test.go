package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// sharedContainer holds the single Postgres container started in TestMain
// (defined in integration_test.go).
var sharedContainer *PostgresContainer

// newTestDB returns a *Postgres connected to the shared container.
// Each test gets its own connection pool so mutations (logger, observer,
// GracefulShutdown) don't bleed between tests.
func newTestDB(t *testing.T) *Postgres {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	require.NotNil(t, sharedContainer, "shared Postgres container not started")
	pg, err := NewPostgres(sharedContainer.Config)
	require.NoError(t, err, "failed to connect to shared Postgres container")
	t.Cleanup(func() { _ = pg.GracefulShutdown() })
	return pg
}

// ── error translation (no container needed) ───────────────────────────────────

func TestTranslatePostgreSQLError_AllCodes(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	cases := []struct {
		code string
		want error
	}{
		{"01000", ErrInvalidData},
		{"02000", ErrRecordNotFound},
		{"02001", ErrRecordNotFound},
		{"03000", ErrInvalidQuery},
		{"08000", ErrConnectionFailed},
		{"08001", ErrConnectionFailed},
		{"08003", ErrConnectionLost},
		{"08004", ErrConnectionFailed},
		{"08006", ErrConnectionLost},
		{"08007", ErrTransactionFailed},
		{"08P01", ErrProtocolViolation},
		{"09000", ErrTriggerNotFound},
		{"0A000", ErrUnsupportedOperation},
		{"0B000", ErrTransactionFailed},
		{"0F000", ErrInvalidCursor},
		{"0F001", ErrInvalidCursor},
		{"0L000", ErrPermissionDenied},
		{"0LP01", ErrInsufficientPrivileges},
		{"0P000", ErrPermissionDenied},
		{"0Z000", ErrSystemError},
		{"0Z002", ErrSystemError},
		{"20000", ErrRecordNotFound},
		{"21000", ErrConstraintViolation},
		{"22000", ErrInvalidData},
		{"22001", ErrDataTooLong},
		{"22002", ErrNotNullViolation},
		{"22003", ErrNumericOverflow},
		{"22004", ErrNotNullViolation},
		{"22005", ErrInvalidDataType},
		{"22007", ErrInvalidDataType},
		{"22008", ErrNumericOverflow},
		{"22009", ErrInvalidData},
		{"22012", ErrDivisionByZero},
		{"22015", ErrNumericOverflow},
		{"22018", ErrInvalidDataType},
		{"22019", ErrInvalidData},
		{"22021", ErrInvalidDataType},
		{"22022", ErrNumericOverflow},
		{"22023", ErrInvalidData},
		{"22024", ErrInvalidData},
		{"22025", ErrInvalidData},
		{"22026", ErrDataTooLong},
		{"22027", ErrInvalidData},
		{"2202E", ErrInvalidData},
		{"2202F", ErrInvalidData},
		{"22030", ErrInvalidJSON},
		{"22032", ErrInvalidJSON},
		{"22P01", ErrNumericOverflow},
		{"22P02", ErrInvalidDataType},
		{"22P03", ErrInvalidDataType},
		{"22P04", ErrInvalidData},
		{"22P05", ErrInvalidDataType},
		{"22P06", ErrInvalidData},
		{"23000", ErrConstraintViolation},
		{"23001", ErrForeignKey},
		{"23502", ErrNotNullViolation},
		{"23503", ErrForeignKey},
		{"23505", ErrDuplicateKey},
		{"23514", ErrCheckConstraintViolation},
		{"23P01", ErrConstraintViolation},
		{"24000", ErrInvalidCursor},
		{"24001", ErrSystemError}, // not in switch → default
		{"25000", ErrTransactionFailed},
		{"25001", ErrTransactionFailed},
		{"25002", ErrTransactionFailed},
		{"25003", ErrTransactionFailed},
		{"25004", ErrTransactionFailed},
		{"25005", ErrTransactionFailed},
		{"25006", ErrTransactionFailed},
		{"25007", ErrTransactionFailed},
		{"25008", ErrTransactionFailed},
		{"25P01", ErrTransactionFailed},
		{"25P02", ErrTransactionFailed},
		{"25P03", ErrIdleInTransaction},
		{"26000", ErrInvalidQuery},
		{"27000", ErrTriggerNotFound},
		{"28000", ErrInvalidPassword},
		{"28P01", ErrInvalidPassword},
		{"2B000", ErrConstraintViolation},
		{"2BP01", ErrConstraintViolation},
		{"2D000", ErrTransactionFailed},
		{"2F000", ErrFunctionNotFound},
		{"2F002", ErrPermissionDenied},
		{"2F003", ErrPermissionDenied},
		{"2F004", ErrPermissionDenied},
		{"2F005", ErrFunctionNotFound},
		{"34000", ErrCursorNotFound},
		{"38000", ErrFunctionNotFound},
		{"38001", ErrPermissionDenied},
		{"38002", ErrPermissionDenied},
		{"38003", ErrPermissionDenied},
		{"38004", ErrPermissionDenied},
		{"39000", ErrFunctionNotFound},
		{"39001", ErrFunctionNotFound},
		{"39004", ErrNotNullViolation},
		{"39P01", ErrTriggerNotFound},
		{"39P02", ErrFunctionNotFound},
		{"39P03", ErrTriggerNotFound},
		{"3B000", ErrTransactionFailed},
		{"3B001", ErrTransactionFailed},
		{"3D000", ErrDatabaseNotFound},
		{"3F000", ErrSchemaNotFound},
		{"40000", ErrTransactionFailed},
		{"40001", ErrSerializationFailure},
		{"40002", ErrConstraintViolation},
		{"40003", ErrTransactionFailed},
		{"40P01", ErrDeadlock},
		{"42000", ErrInvalidQuery},
		{"42501", ErrInsufficientPrivileges},
		{"42601", ErrInvalidQuery},
		{"42602", ErrInvalidQuery},
		{"42611", ErrInvalidQuery},
		{"42622", ErrDataTooLong},
		{"42701", ErrDuplicateKey},
		{"42702", ErrColumnNotFound},
		{"42703", ErrColumnNotFound},
		{"42704", ErrTableNotFound},
		{"42710", ErrDuplicateKey},
		{"42712", ErrDuplicateKey},
		{"42723", ErrDuplicateKey},
		{"42725", ErrFunctionNotFound},
		{"42803", ErrInvalidQuery},
		{"42804", ErrInvalidDataType},
		{"42809", ErrInvalidQuery},
		{"42830", ErrForeignKey},
		{"42846", ErrInvalidDataType},
		{"42883", ErrFunctionNotFound},
		{"42939", ErrInvalidQuery},
		{"42P01", ErrTableNotFound},
		{"42P02", ErrInvalidQuery},
		{"42P03", ErrInvalidCursor},
		{"42P04", ErrDuplicateKey},
		{"42P05", ErrDuplicateKey},
		{"42P06", ErrDuplicateKey},
		{"42P07", ErrDuplicateKey},
		{"42P08", ErrInvalidQuery},
		{"42P09", ErrInvalidQuery},
		{"42P10", ErrColumnNotFound},
		{"42P11", ErrInvalidCursor},
		{"42P12", ErrConfigurationError},
		{"42P13", ErrFunctionNotFound},
		{"42P14", ErrInvalidQuery},
		{"42P15", ErrSchemaValidation},
		{"42P16", ErrSchemaValidation},
		{"42P17", ErrSchemaValidation},
		{"42P18", ErrInvalidDataType},
		{"42P19", ErrInvalidQuery},
		{"42P20", ErrInvalidQuery},
		{"42P21", ErrInvalidDataType},
		{"42P22", ErrInvalidDataType},
		{"44000", ErrCheckConstraintViolation},
		{"53000", ErrSystemError},
		{"53100", ErrDiskFull},
		{"53200", ErrSystemError},
		{"53300", ErrTooManyConnections},
		{"53400", ErrConfigurationError},
		{"54000", ErrSystemError},
		{"54001", ErrInvalidQuery},
		{"54011", ErrSchemaValidation},
		{"54023", ErrInvalidQuery},
		{"55000", ErrSystemError},
		{"55006", ErrSystemError},
		{"55P02", ErrConfigurationError},
		{"55P03", ErrLockTimeout},
		{"55P04", ErrUnsupportedOperation},
		{"57000", ErrSystemError},
		{"57014", ErrQueryTimeout},
		{"57P01", ErrSystemError},
		{"57P02", ErrSystemError},
		{"57P03", ErrConnectionFailed},
		{"57P04", ErrDatabaseNotFound},
		{"57P05", ErrQueryTimeout},
		{"58000", ErrSystemError},
		{"58030", ErrSystemError},
		{"58P01", ErrSystemError},
		{"58P02", ErrSystemError},
		{"72000", ErrSystemError}, // not in switch → default
		{"F0000", ErrConfigurationError},
		{"F0001", ErrConfigurationError},
		{"HV000", ErrSystemError},
		{"HV005", ErrColumnNotFound},
		{"HV007", ErrInvalidData},
		{"HV008", ErrFunctionNotFound},
		{"HV009", ErrSystemError},
		{"HV00A", ErrInvalidData},
		{"HV00B", ErrColumnNotFound},
		{"HV00C", ErrColumnNotFound},
		{"HV00D", ErrInvalidDataType},
		{"HV00J", ErrInvalidData},
		{"HV00K", ErrSystemError},
		{"HV00L", ErrInvalidData},
		{"HV00M", ErrInvalidData},
		{"HV00N", ErrDataTooLong},
		{"HV00P", ErrInvalidData},
		{"HV00Q", ErrInvalidData},
		{"HV00R", ErrSystemError},
		{"HV010", ErrSystemError},
		{"HV014", ErrSystemError},
		{"HV016", ErrSystemError},
		{"HV017", ErrConnectionFailed},
		{"HV018", ErrSchemaNotFound},
		{"HV019", ErrConfigurationError},
		{"HV01A", ErrSystemError},
		{"HV01B", ErrSchemaNotFound},
		{"HV01C", ErrTableNotFound},
		{"HV01D", ErrSystemError},
		{"P0000", ErrFunctionNotFound},
		{"P0001", ErrFunctionNotFound},
		{"P0002", ErrRecordNotFound},
		{"P0003", ErrConstraintViolation},
		{"P0004", ErrCheckConstraintViolation},
		{"XX000", ErrInternalError},
		{"XX001", ErrIndexCorruption},
		{"XX002", ErrIndexCorruption},
		{"ZZZZZ", ErrSystemError}, // unknown → default
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("code_%s", tc.code), func(t *testing.T) {
			t.Parallel()
			got := p.translatePostgreSQLError(&pgconn.PgError{Code: tc.code})
			assert.ErrorIs(t, got, tc.want, "code %s", tc.code)
		})
	}
}

func TestTranslateByErrorMessage_AllBranches(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	sentinel := errors.New("original")
	cases := []struct {
		msg  string
		want error
	}{
		{"connection refused", ErrConnectionFailed},
		{"connection reset by peer", ErrConnectionLost},
		{"too many clients already", ErrTooManyConnections},
		{"connection timed out", ErrConnectionFailed},
		{"server closed the connection", ErrConnectionLost},
		{"statement timeout exceeded", ErrStatementTimeout},
		{"canceling statement due to statement timeout", ErrStatementTimeout},
		{"lock timeout occurred", ErrLockTimeout},
		{"query timeout exceeded", ErrQueryTimeout},
		{"idle in transaction session detected", ErrIdleInTransaction},
		{"deadlock detected", ErrDeadlock},
		{"could not obtain lock on row", ErrLockTimeout},
		{"value too long for type", ErrDataTooLong},
		{"invalid input syntax for integer", ErrInvalidDataType},
		{"invalid json document", ErrInvalidJSON},
		{"division by zero", ErrDivisionByZero},
		{"numeric field overflow occurred", ErrNumericOverflow},
		{"violates check constraint", ErrCheckConstraintViolation},
		{"duplicated key not allowed", ErrDuplicateKey},
		{"duplicate key value violates unique constraint", ErrDuplicateKey},
		{"violates not-null constraint", ErrNotNullViolation},
		{"violates foreign key constraint", ErrForeignKey},
		{"violates unique constraint foo", ErrDuplicateKey},
		{"relation \"users\" does not exist", ErrTableNotFound},
		{"column \"id\" does not exist", ErrColumnNotFound},
		{"function myfn does not exist", ErrFunctionNotFound},
		{"database mydb does not exist", ErrDatabaseNotFound},
		{"schema myschema does not exist", ErrSchemaNotFound},
		{"sequence myseq does not exist", ErrSequenceNotFound},
		{"index myidx does not exist", ErrIndexNotFound},
		{"view myview does not exist", ErrViewNotFound},
		{"trigger mytrig does not exist", ErrTriggerNotFound},
		{"invalid field name", ErrColumnNotFound},
		{"permission denied for table users", ErrPermissionDenied},
		{"insufficient privilege to perform action", ErrInsufficientPrivileges},
		{"authentication failed for user", ErrInvalidPassword},
		{"role foo does not exist", ErrInvalidPassword},
		{"disk full error occurred", ErrDiskFull},
		{"out of memory allocation failed", ErrSystemError},
		{"internal error detected", ErrInternalError},
		{"index corruption found", ErrIndexCorruption},
		{"data corrupted in page", ErrIndexCorruption},
		{"cursor foo does not exist", ErrCursorNotFound},
		{"invalid cursor position", ErrInvalidCursor},
		{"migration script failed", ErrMigrationFailed},
		{"backup failed for database", ErrBackupFailed},
		{"restore operation failed", ErrRestoreFailed},
		{"some completely unknown error", sentinel},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			got := p.translateByErrorMessage(tc.msg, sentinel)
			assert.ErrorIs(t, got, tc.want)
		})
	}
}

func TestTranslateError_GormNotFound(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	assert.ErrorIs(t, p.TranslateError(gorm.ErrRecordNotFound), ErrRecordNotFound)
}

func TestTranslateError_Nil(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	assert.NoError(t, p.TranslateError(nil))
}

func TestGetErrorCategory(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	assert.Equal(t, CategoryConnection, p.GetErrorCategory(ErrConnectionFailed))
	assert.Equal(t, CategoryQuery, p.GetErrorCategory(ErrInvalidQuery))
	assert.Equal(t, CategoryData, p.GetErrorCategory(ErrInvalidData))
	assert.Equal(t, CategoryConstraint, p.GetErrorCategory(ErrDuplicateKey))
	assert.Equal(t, CategoryPermission, p.GetErrorCategory(ErrPermissionDenied))
	assert.Equal(t, CategoryTransaction, p.GetErrorCategory(ErrDeadlock))
	assert.Equal(t, CategoryResource, p.GetErrorCategory(ErrDiskFull))
	assert.Equal(t, CategorySystem, p.GetErrorCategory(ErrSystemError))
	assert.Equal(t, CategorySchema, p.GetErrorCategory(ErrTableNotFound))
	assert.Equal(t, CategoryOperation, p.GetErrorCategory(ErrRecordNotFound))
	assert.Equal(t, CategoryUnknown, p.GetErrorCategory(errors.New("random")))
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	assert.True(t, p.IsRetryable(ErrConnectionFailed))
	assert.True(t, p.IsRetryable(ErrDeadlock))
	assert.False(t, p.IsRetryable(ErrDuplicateKey))
}

func TestIsTemporary(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	assert.True(t, p.IsTemporary(ErrConnectionLost))
	assert.True(t, p.IsTemporary(ErrLockTimeout))
	assert.False(t, p.IsTemporary(ErrPermissionDenied))
}

func TestIsCritical(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	assert.True(t, p.IsCritical(ErrSystemError))
	assert.True(t, p.IsCritical(ErrDiskFull))
	assert.False(t, p.IsCritical(ErrDuplicateKey))
}

// ── migration file helpers (no container needed) ──────────────────────────────

func TestMigrationConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, MigrationType("schema"), SchemaType)
	assert.Equal(t, MigrationType("data"), DataType)
	assert.Equal(t, MigrationDirection("up"), UpMigration)
	assert.Equal(t, MigrationDirection("down"), DownMigration)
}

func TestLoadMigrations_ValidFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "20240101_schema_create_users.up.sql", "CREATE TABLE users (id INT);")
	writeFile(t, dir, "20240101_schema_create_users.down.sql", "DROP TABLE users;")
	writeFile(t, dir, "20240102_data_seed_users.up.sql", "INSERT INTO users VALUES (1);")

	p := &Postgres{}
	ups, err := p.loadMigrations(dir, UpMigration)
	require.NoError(t, err)
	require.Len(t, ups, 2)

	downs, err := p.loadMigrations(dir, DownMigration)
	require.NoError(t, err)
	require.Len(t, downs, 1)
}

func TestLoadMigrations_NonexistentDir(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	ups, err := p.loadMigrations("/nonexistent/path/to/nowhere", UpMigration)
	assert.NoError(t, err)
	assert.Empty(t, ups)
}

func TestLoadMigrations_EmptyDir(t *testing.T) {
	t.Parallel()
	p := &Postgres{}
	ups, err := p.loadMigrations(t.TempDir(), UpMigration)
	assert.NoError(t, err)
	assert.Empty(t, ups)
}

func TestLoadMigrations_InvalidFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "not_a_migration.sql", "SELECT 1;")
	p := &Postgres{}
	ups, err := p.loadMigrations(dir, UpMigration)
	assert.NoError(t, err)
	assert.Empty(t, ups)
}

func TestCreateMigration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := &Postgres{}
	_, err := p.CreateMigration(dir, "add_index", SchemaType)
	require.NoError(t, err)
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 2) // up + down
}

// ── observer (no container needed) ───────────────────────────────────────────

func TestObserveOperation_Duration(t *testing.T) {
	t.Parallel()
	obs := &TestObserver{}
	p := &Postgres{cfg: Config{Connection: Connection{DbName: "db"}}, observer: obs}
	d := 50 * time.Millisecond
	p.observeOperation("op", "tbl", "sub", d, nil, 1, nil)
	ops := obs.GetOperations()
	require.Len(t, ops, 1)
	assert.Equal(t, d, ops[0].Duration)
	assert.Equal(t, "sub", ops[0].SubResource)
}

// ── query builder modifier chain (real Postgres) ──────────────────────────────

func TestQueryBuilder_ModifierChain(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)

	qb := pg.Query(context.Background()).(*postgresQueryBuilder)
	result := qb.
		Or("status = ?", "pending").
		RightJoin("qb_items b ON b.id = qb_items.id").
		Offset(10).
		Distinct("id").
		Table("qb_items").
		Unscoped().
		Scopes(func(d *gorm.DB) *gorm.DB { return d }).
		ForUpdate().
		ForShare().
		ForUpdateSkipLocked().
		ForShareSkipLocked().
		ForUpdateNoWait().
		ForNoKeyUpdate().
		ForKeyShare().
		Returning("id").
		Clauses()

	assert.NotNil(t, result)
}

func TestQueryBuilder_Done(t *testing.T) {
	pg := newTestDB(t)
	pg.Query(context.Background()).Done() // must not panic
}

func TestQueryBuilder_Last(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)

	var item qbItem
	err := pg.Query(context.Background()).Last(&item)
	assert.Error(t, err) // empty table
}

func TestQueryBuilder_Count(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "a"})
	pg.DB().Create(&qbItem{Val: "b"})

	var count int64
	err := pg.Query(context.Background()).Model(&qbItem{}).Count(&count)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestQueryBuilder_Updates(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "old"})

	rows, err := pg.Query(context.Background()).
		Model(&qbItem{}).
		Where("val = ?", "old").
		Updates(map[string]interface{}{"val": "new"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)
}

func TestQueryBuilder_Delete(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "todelete"})

	rows, err := pg.Query(context.Background()).
		Where("val = ?", "todelete").
		Delete(&qbItem{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)
}

func TestQueryBuilder_Pluck(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "alpha"})
	pg.DB().Create(&qbItem{Val: "beta"})

	var vals []string
	rows, err := pg.Query(context.Background()).Model(&qbItem{}).Pluck("val", &vals)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rows)
	assert.ElementsMatch(t, []string{"alpha", "beta"}, vals)
}

func TestQueryBuilder_CreateInBatches(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)

	items := []qbItem{{Val: "x"}, {Val: "y"}, {Val: "z"}}
	rows, err := pg.Query(context.Background()).CreateInBatches(&items, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), rows)
}

func TestQueryBuilder_FirstOrInit(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)

	var item qbItem
	err := pg.Query(context.Background()).Where("val = ?", "missing").FirstOrInit(&item)
	assert.NoError(t, err)
}

func TestQueryBuilder_FirstOrCreate(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)

	var item qbItem
	err := pg.Query(context.Background()).Where(qbItem{Val: "newval"}).FirstOrCreate(&item)
	assert.NoError(t, err)
	assert.Equal(t, "newval", item.Val)
}

// ── row scanner (real Postgres) ───────────────────────────────────────────────

func TestQueryBuilder_QueryRow(t *testing.T) {
	pg := newTestDB(t)
	qb := &postgresQueryBuilder{
		db:      pg.DB().Raw("SELECT 1 + 1 AS num"),
		pg:      pg,
		release: func() {},
	}
	row := qb.QueryRow()
	require.NotNil(t, row)
	var n int
	assert.NoError(t, row.Scan(&n))
	assert.Equal(t, 2, n)
}

func TestQueryBuilder_QueryRows(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "r1"})
	pg.DB().Create(&qbItem{Val: "r2"})

	qb := &postgresQueryBuilder{
		db:      pg.DB().Model(&qbItem{}).Select("val"),
		pg:      pg,
		release: func() {},
	}
	rows, err := qb.QueryRows()
	require.NoError(t, err)
	defer rows.Close()

	var vals []string
	for rows.Next() {
		var v string
		require.NoError(t, rows.Scan(&v))
		vals = append(vals, v)
	}
	require.NoError(t, rows.Err())
	assert.ElementsMatch(t, []string{"r1", "r2"}, vals)
}

func TestQueryBuilder_ScanRow(t *testing.T) {
	pg := newTestDB(t)
	qb := &postgresQueryBuilder{
		db:      pg.DB().Raw("SELECT 42 AS num"),
		pg:      pg,
		release: func() {},
	}
	type Result struct{ Num int }
	var r Result
	err := qb.ScanRow(&r)
	assert.NoError(t, err)
	assert.Equal(t, 42, r.Num)
}

func TestQueryBuilder_MapRows(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "m1"})

	qb := &postgresQueryBuilder{
		db:      pg.DB().Model(&qbItem{}),
		pg:      pg,
		release: func() {},
	}
	var out []qbItem
	err := qb.MapRows(&out, func(d *gorm.DB) error {
		return d.Find(&out).Error
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}

// ── observer callbacks (real Postgres) ────────────────────────────────────────

func TestQueryBuilder_LastWithObserver(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	pg.WithObserver(obs)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "obs"})

	var item qbItem
	_ = pg.Query(context.Background()).Model(&qbItem{}).Last(&item)
	assertObservedOp(t, obs, "last")
}

func TestQueryBuilder_CountWithObserver(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	pg.WithObserver(obs)
	setupTable(t, pg)

	var count int64
	_ = pg.Query(context.Background()).Model(&qbItem{}).Count(&count)
	assertObservedOp(t, obs, "count")
}

func TestQueryBuilder_UpdatesWithObserver(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	pg.WithObserver(obs)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "upd"})

	_, _ = pg.Query(context.Background()).
		Model(&qbItem{}).
		Where("val = ?", "upd").
		Updates(map[string]interface{}{"val": "updated"})
	assertObservedOp(t, obs, "update")
}

func TestQueryBuilder_DeleteWithObserver(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	pg.WithObserver(obs)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "del"})

	_, _ = pg.Query(context.Background()).
		Where("val = ?", "del").
		Delete(&qbItem{})
	assertObservedOp(t, obs, "delete")
}

func TestQueryBuilder_PluckWithObserver(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	pg.WithObserver(obs)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "pluck"})

	var vals []string
	_, _ = pg.Query(context.Background()).Model(&qbItem{}).Pluck("val", &vals)
	assertObservedOp(t, obs, "pluck")
}

func TestQueryBuilder_CreateInBatchesWithObserver(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	pg.WithObserver(obs)
	setupTable(t, pg)

	items := []qbItem{{Val: "b1"}, {Val: "b2"}}
	_, _ = pg.Query(context.Background()).CreateInBatches(&items, 1)
	assertObservedOp(t, obs, "create_in_batches")
}

func TestQueryBuilder_FirstOrInitWithObserver(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	pg.WithObserver(obs)
	setupTable(t, pg)

	var item qbItem
	_ = pg.Query(context.Background()).Where("val = ?", "missing").FirstOrInit(&item)
	assertObservedOp(t, obs, "first_or_init")
}

func TestQueryBuilder_FirstOrCreateWithObserver(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	pg.WithObserver(obs)
	setupTable(t, pg)

	var item qbItem
	_ = pg.Query(context.Background()).Where(qbItem{Val: "fc"}).FirstOrCreate(&item)
	assertObservedOp(t, obs, "first_or_create")
}

// ── Returning / Clauses — Postgres-native features ────────────────────────────

func TestQueryBuilder_Returning_Wildcard(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)

	// Verify that Returning("*") produces a builder with the clause attached —
	// the actual SQL execution with RETURNING is covered by the integration tests.
	qb := pg.Query(context.Background()).(*postgresQueryBuilder)
	result := qb.Returning("*")
	assert.Same(t, qb, result.(*postgresQueryBuilder))
}

func TestQueryBuilder_Returning_Columns(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)

	item := qbItem{Val: "ret_col"}
	_, err := pg.Query(context.Background()).
		Returning("id").
		Create(&item)
	assert.NoError(t, err)
	assert.NotZero(t, item.ID)
}

func TestQueryBuilder_ClausesWithExpression(t *testing.T) {
	pg := newTestDB(t)
	setupTable(t, pg)
	pg.DB().Create(&qbItem{Val: "c1"})
	pg.DB().Create(&qbItem{Val: "c2"})

	var items []qbItem
	qb := &postgresQueryBuilder{db: pg.DB(), pg: pg, release: func() {}}
	err := qb.Clauses(clause.OrderBy{
		Columns: []clause.OrderByColumn{{Column: clause.Column{Name: "val"}, Desc: true}},
	}).Find(&items)
	assert.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "c2", items[0].Val)
}

// ── migrations (real Postgres) ────────────────────────────────────────────────

func TestAutoMigrate_WithPostgres(t *testing.T) {
	pg := newTestDB(t)
	require.NoError(t, pg.DB().Migrator().DropTable(&qbItem{}))
	require.NoError(t, pg.DB().Migrator().DropTable(&MigrationHistoryRecord{}))
	assert.NoError(t, pg.AutoMigrate(&qbItem{}))

	var records []MigrationHistoryRecord
	require.NoError(t, pg.DB().Find(&records).Error)
	assert.NotEmpty(t, records)
}

func TestGetMigrationStatus_WithPostgres(t *testing.T) {
	pg := newTestDB(t)
	setupMigrations(t, pg)

	dir := t.TempDir()
	writeFile(t, dir, "20240101_schema_init.up.sql", "SELECT 1;")

	status, err := pg.GetMigrationStatus(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, status, 1)
	assert.Equal(t, false, status[0]["applied"])
}

func TestMigrateUp_WithPostgres(t *testing.T) {
	pg := newTestDB(t)
	setupMigrations(t, pg)

	dir := t.TempDir()
	writeFile(t, dir, "20240101_schema_create_test.up.sql", "SELECT 1;")
	writeFile(t, dir, "20240102_schema_add_col.up.sql", "SELECT 2;")

	assert.NoError(t, pg.MigrateUp(context.Background(), dir))
	// Idempotent
	assert.NoError(t, pg.MigrateUp(context.Background(), dir))
}

func TestMigrateDown_WithPostgres(t *testing.T) {
	pg := newTestDB(t)
	setupMigrations(t, pg)

	record := MigrationHistoryRecord{
		ID: "20240101", Name: "create_test", Type: string(SchemaType),
		ExecutedAt: time.Now(), ExecutedBy: "test", Status: "success",
	}
	require.NoError(t, pg.DB().Create(&record).Error)

	dir := t.TempDir()
	writeFile(t, dir, "20240101_schema_create_test.down.sql", "SELECT 1;")

	assert.NoError(t, pg.MigrateDown(context.Background(), dir))
}

// ── fx / lifecycle (real Postgres) ────────────────────────────────────────────

func TestProvideClient(t *testing.T) {
	pg := newTestDB(t)
	c := ProvideClient(pg)
	assert.NotNil(t, c)
	assert.Implements(t, (*Client)(nil), c)
}

func TestGracefulShutdown_WithPostgres(t *testing.T) {
	pg := newTestDB(t)
	assert.NoError(t, pg.GracefulShutdown())
	assert.NoError(t, pg.GracefulShutdown()) // idempotent
}

// ── setup helpers (real Postgres) ─────────────────────────────────────────────

func TestWithLogger(t *testing.T) {
	pg := newTestDB(t)
	lg := &testLogger{}
	result := pg.WithLogger(lg)
	assert.Same(t, pg, result)
	assert.Equal(t, lg, pg.logger)
}

func TestWithObserver_Builder(t *testing.T) {
	pg := newTestDB(t)
	obs := &TestObserver{}
	result := pg.WithObserver(obs)
	assert.Same(t, pg, result)
	assert.Equal(t, obs, pg.observer)
}

func TestLogInfo_WithLogger(t *testing.T) {
	pg := newTestDB(t)
	lg := &testLogger{}
	pg.logger = lg
	pg.logInfo(context.Background(), "info msg", nil)
	assert.Equal(t, 1, lg.infoCount)
}

func TestLogWarn_WithLogger(t *testing.T) {
	pg := newTestDB(t)
	lg := &testLogger{}
	pg.logger = lg
	pg.logWarn(context.Background(), "warn msg", map[string]interface{}{"k": "v"})
	assert.Equal(t, 1, lg.warnCount)
}

func TestLogError_WithLogger(t *testing.T) {
	pg := newTestDB(t)
	lg := &testLogger{}
	pg.logger = lg
	pg.logError(context.Background(), "err msg", nil)
	assert.Equal(t, 1, lg.errorCount)
}

func TestLogInfo_NoLogger(t *testing.T) {
	pg := newTestDB(t)
	pg.logInfo(context.Background(), "msg", nil) // must not panic
}

func TestLogWarn_NoLogger(t *testing.T) {
	pg := newTestDB(t)
	pg.logWarn(context.Background(), "msg", nil) // must not panic
}

func TestLogError_NoLogger(t *testing.T) {
	pg := newTestDB(t)
	pg.logError(context.Background(), "msg", nil) // must not panic
}

// ── helpers ───────────────────────────────────────────────────────────────────

// qbItem is a named model used across query builder tests.
type qbItem struct {
	gorm.Model
	Val string
}

// setupTable drops and re-creates the qb_items table so each test starts clean.
func setupTable(t *testing.T, pg *Postgres) {
	t.Helper()
	require.NoError(t, pg.DB().Migrator().DropTable(&qbItem{}))
	require.NoError(t, pg.DB().AutoMigrate(&qbItem{}))
}

// setupMigrations drops and re-creates the migration_history_records table.
func setupMigrations(t *testing.T, pg *Postgres) {
	t.Helper()
	require.NoError(t, pg.DB().Migrator().DropTable(&MigrationHistoryRecord{}))
	require.NoError(t, pg.DB().AutoMigrate(&MigrationHistoryRecord{}))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func assertObservedOp(t *testing.T, obs *TestObserver, op string) {
	t.Helper()
	ops := obs.GetOperations()
	require.NotEmpty(t, ops, "expected at least one observed operation")
	assert.Equal(t, op, ops[len(ops)-1].Operation)
}

type testLogger struct {
	infoCount  int
	warnCount  int
	errorCount int
}

func (l *testLogger) InfoWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	l.infoCount++
}
func (l *testLogger) WarnWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	l.warnCount++
}
func (l *testLogger) ErrorWithContext(_ context.Context, _ string, _ error, _ ...map[string]interface{}) {
	l.errorCount++
}
