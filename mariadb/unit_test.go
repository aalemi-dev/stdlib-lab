package mariadb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// sharedContainer holds the single MariaDB container started in TestMain
// (defined in integration_test.go).
var sharedContainer *MariaDBContainer

// newTestDB returns a *MariaDB connected to the shared container.
// Each test gets its own connection pool so mutations (logger, observer,
// GracefulShutdown) don't bleed between tests.
func newTestDB(t *testing.T) *MariaDB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	require.NotNil(t, sharedContainer, "shared MariaDB container not started")
	db, err := NewMariaDB(sharedContainer.Config)
	require.NoError(t, err, "failed to connect to shared MariaDB container")
	t.Cleanup(func() { _ = db.GracefulShutdown() })
	return db
}

// ── error translation (no container needed) ───────────────────────────────────

func TestTranslateMySQLError_AllCodes(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	cases := []struct {
		number uint16
		want   error
	}{
		{1062, ErrDuplicateKey},
		{1586, ErrDuplicateKey},
		{1216, ErrForeignKey},
		{1217, ErrForeignKey},
		{1451, ErrForeignKey},
		{1452, ErrForeignKey},
		{1557, ErrForeignKey},
		{1761, ErrForeignKey},
		{1051, ErrTableNotFound},
		{1054, ErrColumnNotFound},
		{1091, ErrColumnNotFound},
		{1146, ErrTableNotFound},
		{1406, ErrDataTooLong},
		{1264, ErrNumericOverflow},
		{1048, ErrNotNullViolation},
		{1364, ErrNotNullViolation},
		{1690, ErrConstraintViolation},
		{3819, ErrCheckConstraintViolation},
		{4025, ErrCheckConstraintViolation},
		{1205, ErrLockTimeout},
		{1213, ErrDeadlock},
		{1637, ErrDeadlock},
		{1040, ErrTooManyConnections},
		{1158, ErrConnectionLost},
		{1159, ErrConnectionLost},
		{1160, ErrConnectionLost},
		{1161, ErrConnectionLost},
		{2002, ErrConnectionFailed},
		{2003, ErrConnectionFailed},
		{2006, ErrConnectionLost},
		{2013, ErrConnectionLost},
		{2055, ErrConnectionLost},
		{1064, ErrInvalidQuery},
		{1065, ErrInvalidQuery},
		{1149, ErrInvalidQuery},
		{1044, ErrPermissionDenied},
		{1045, ErrInvalidPassword},
		{1142, ErrPermissionDenied},
		{1143, ErrPermissionDenied},
		{1227, ErrInsufficientPrivileges},
		{1568, ErrTransactionFailed},
		{1792, ErrTransactionFailed},
		{1365, ErrDivisionByZero},
		{3140, ErrInvalidJSON},
		{3141, ErrInvalidJSON},
		{3143, ErrInvalidJSON},
		{1049, ErrDatabaseNotFound},
		{1008, ErrDatabaseNotFound},
		{1305, ErrFunctionNotFound},
		{1370, ErrFunctionNotFound},
		{1061, ErrDuplicateKey},
		{1176, ErrIndexNotFound},
		{3, ErrSystemError},
		{9, ErrSystemError},
		{1021, ErrDiskFull},
		{1969, ErrStatementTimeout},
		{1366, ErrInvalidDataType},
		{1582, ErrInvalidDataType},
		{9999, ErrSystemError}, // unknown → default
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("code_%d", tc.number), func(t *testing.T) {
			t.Parallel()
			got := m.translateMySQLError(&mysql.MySQLError{Number: tc.number})
			assert.ErrorIs(t, got, tc.want)
		})
	}
}

func TestTranslateByErrorMessage_AllBranches(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	sentinel := errors.New("original")
	cases := []struct {
		msg  string
		want error
	}{
		{"connection refused", ErrConnectionFailed},
		{"connection reset", ErrConnectionLost},
		{"too many connections", ErrTooManyConnections},
		{"connection timed out", ErrConnectionFailed},
		{"server has gone away", ErrConnectionLost},
		{"broken pipe", ErrConnectionLost},
		{"deadlock found", ErrDeadlock},
		{"lock wait timeout exceeded", ErrLockTimeout},
		{"try restarting transaction", ErrDeadlock},
		{"data too long for column", ErrDataTooLong},
		{"out of range value", ErrNumericOverflow},
		{"invalid json text", ErrInvalidJSON},
		{"division by zero", ErrDivisionByZero},
		{"incorrect integer value", ErrInvalidDataType},
		{"duplicate entry", ErrDuplicateKey},
		{"duplicate key value", ErrDuplicateKey},
		{"column cannot be null", ErrNotNullViolation},
		{"foreign key constraint fails", ErrForeignKey},
		{"check constraint violated", ErrCheckConstraintViolation},
		{"table `foo` doesn't exist", ErrTableNotFound},
		{"unknown column 'bar'", ErrColumnNotFound},
		{"unknown database 'baz'", ErrDatabaseNotFound},
		{"no such table: foo", ErrTableNotFound},
		{"access denied for user", ErrPermissionDenied},
		{"command denied to user", ErrInsufficientPrivileges},
		{"disk full", ErrDiskFull},
		{"out of memory", ErrSystemError},
		{"some completely unknown error", sentinel},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.msg, func(t *testing.T) {
			t.Parallel()
			got := m.translateByErrorMessage(tc.msg, sentinel)
			assert.ErrorIs(t, got, tc.want)
		})
	}
}

func TestTranslateError_GormNotFound(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	assert.ErrorIs(t, m.TranslateError(gorm.ErrRecordNotFound), ErrRecordNotFound)
}

func TestTranslateError_Nil(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	assert.NoError(t, m.TranslateError(nil))
}

func TestGetErrorCategory(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	assert.Equal(t, CategoryConnection, m.GetErrorCategory(ErrConnectionFailed))
	assert.Equal(t, CategoryQuery, m.GetErrorCategory(ErrInvalidQuery))
	assert.Equal(t, CategoryData, m.GetErrorCategory(ErrInvalidData))
	assert.Equal(t, CategoryConstraint, m.GetErrorCategory(ErrDuplicateKey))
	assert.Equal(t, CategoryPermission, m.GetErrorCategory(ErrPermissionDenied))
	assert.Equal(t, CategoryTransaction, m.GetErrorCategory(ErrDeadlock))
	assert.Equal(t, CategoryResource, m.GetErrorCategory(ErrDiskFull))
	assert.Equal(t, CategorySystem, m.GetErrorCategory(ErrSystemError))
	assert.Equal(t, CategorySchema, m.GetErrorCategory(ErrTableNotFound))
	assert.Equal(t, CategoryOperation, m.GetErrorCategory(ErrRecordNotFound))
	assert.Equal(t, CategoryUnknown, m.GetErrorCategory(errors.New("random")))
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	assert.True(t, m.IsRetryable(ErrConnectionFailed))
	assert.True(t, m.IsRetryable(ErrDeadlock))
	assert.False(t, m.IsRetryable(ErrDuplicateKey))
}

func TestIsTemporary(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	assert.True(t, m.IsTemporary(ErrConnectionLost))
	assert.True(t, m.IsTemporary(ErrLockTimeout))
	assert.False(t, m.IsTemporary(ErrPermissionDenied))
}

func TestIsCritical(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	assert.True(t, m.IsCritical(ErrSystemError))
	assert.True(t, m.IsCritical(ErrDiskFull))
	assert.False(t, m.IsCritical(ErrDuplicateKey))
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

	m := &MariaDB{}
	ups, err := m.loadMigrations(dir, UpMigration)
	require.NoError(t, err)
	require.Len(t, ups, 2)

	downs, err := m.loadMigrations(dir, DownMigration)
	require.NoError(t, err)
	require.Len(t, downs, 1)
}

func TestLoadMigrations_NonexistentDir(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	// filepath.Glob doesn't error on non-existent paths — returns empty
	ups, err := m.loadMigrations("/nonexistent/path/to/nowhere", UpMigration)
	assert.NoError(t, err)
	assert.Empty(t, ups)
}

func TestLoadMigrations_EmptyDir(t *testing.T) {
	t.Parallel()
	m := &MariaDB{}
	ups, err := m.loadMigrations(t.TempDir(), UpMigration)
	assert.NoError(t, err)
	assert.Empty(t, ups)
}

func TestLoadMigrations_InvalidFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, "not_a_migration.sql", "SELECT 1;")
	m := &MariaDB{}
	ups, err := m.loadMigrations(dir, UpMigration)
	assert.NoError(t, err)
	assert.Empty(t, ups)
}

func TestCreateMigration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := &MariaDB{}
	_, err := m.CreateMigration(dir, "add_index", SchemaType)
	require.NoError(t, err)
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 2) // up + down
}

// ── observer (no container needed) ───────────────────────────────────────────

func TestObserveOperation_Duration(t *testing.T) {
	t.Parallel()
	obs := &TestObserver{}
	m := &MariaDB{cfg: Config{Connection: Connection{DbName: "db"}}, observer: obs}
	d := 50 * time.Millisecond
	m.observeOperation("op", "tbl", "sub", d, nil, 1, nil)
	ops := obs.GetOperations()
	require.Len(t, ops, 1)
	assert.Equal(t, d, ops[0].Duration)
	assert.Equal(t, "sub", ops[0].SubResource)
}

// ── query builder modifier chain (real MariaDB) ───────────────────────────────

func TestQueryBuilder_ModifierChain(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)

	qb := db.Query(context.Background()).(*mariadbQueryBuilder)

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
	db := newTestDB(t)
	qb := db.Query(context.Background())
	// Done should not panic
	qb.Done()
}

func TestQueryBuilder_Last(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)

	// Empty table: record not found
	var item qbItem
	err := db.Query(context.Background()).Last(&item)
	assert.Error(t, err)
}

func TestQueryBuilder_Count(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "a"})
	db.DB().Create(&qbItem{Val: "b"})

	var count int64
	err := db.Query(context.Background()).Model(&qbItem{}).Count(&count)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestQueryBuilder_Updates(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "old"})

	rows, err := db.Query(context.Background()).
		Model(&qbItem{}).
		Where("val = ?", "old").
		Updates(map[string]interface{}{"val": "new"})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)
}

func TestQueryBuilder_Delete(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "todelete"})

	rows, err := db.Query(context.Background()).
		Where("val = ?", "todelete").
		Delete(&qbItem{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), rows)
}

func TestQueryBuilder_Pluck(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "alpha"})
	db.DB().Create(&qbItem{Val: "beta"})

	var vals []string
	rows, err := db.Query(context.Background()).Model(&qbItem{}).Pluck("val", &vals)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rows)
	assert.ElementsMatch(t, []string{"alpha", "beta"}, vals)
}

func TestQueryBuilder_CreateInBatches(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)

	items := []qbItem{{Val: "x"}, {Val: "y"}, {Val: "z"}}
	rows, err := db.Query(context.Background()).CreateInBatches(&items, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), rows)
}

func TestQueryBuilder_FirstOrInit(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)

	var item qbItem
	err := db.Query(context.Background()).Where("val = ?", "missing").FirstOrInit(&item)
	assert.NoError(t, err)
}

func TestQueryBuilder_FirstOrCreate(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)

	var item qbItem
	err := db.Query(context.Background()).Where(qbItem{Val: "newval"}).FirstOrCreate(&item)
	assert.NoError(t, err)
	assert.Equal(t, "newval", item.Val)
}

// ── row scanner (real MariaDB) ────────────────────────────────────────────────

func TestQueryBuilder_QueryRow(t *testing.T) {
	db := newTestDB(t)
	qb := &mariadbQueryBuilder{
		db:      db.DB().Raw("SELECT 1 + 1 AS num"),
		m:       db,
		release: func() {},
	}
	row := qb.QueryRow()
	require.NotNil(t, row)
	var n int
	assert.NoError(t, row.Scan(&n))
	assert.Equal(t, 2, n)
}

func TestQueryBuilder_QueryRows(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "r1"})
	db.DB().Create(&qbItem{Val: "r2"})

	qb := &mariadbQueryBuilder{
		db:      db.DB().Model(&qbItem{}).Select("val"),
		m:       db,
		release: func() {},
	}
	rows, err := qb.QueryRows()
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

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
	db := newTestDB(t)
	qb := &mariadbQueryBuilder{
		db:      db.DB().Raw("SELECT 42 AS num"),
		m:       db,
		release: func() {},
	}
	type Result struct{ Num int }
	var r Result
	err := qb.ScanRow(&r)
	assert.NoError(t, err)
	assert.Equal(t, 42, r.Num)
}

func TestQueryBuilder_MapRows(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "m1"})

	qb := &mariadbQueryBuilder{
		db:      db.DB().Model(&qbItem{}),
		m:       db,
		release: func() {},
	}
	var out []qbItem
	err := qb.MapRows(&out, func(d *gorm.DB) error {
		return d.Find(&out).Error
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, out)
}

// ── observer callbacks (real MariaDB) ─────────────────────────────────────────

func TestQueryBuilder_LastWithObserver(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	db.WithObserver(obs)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "obs"})

	var item qbItem
	_ = db.Query(context.Background()).Model(&qbItem{}).Last(&item)
	assertObservedOp(t, obs, "last")
}

func TestQueryBuilder_CountWithObserver(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	db.WithObserver(obs)
	setupTable(t, db)

	var count int64
	_ = db.Query(context.Background()).Model(&qbItem{}).Count(&count)
	assertObservedOp(t, obs, "count")
}

func TestQueryBuilder_UpdatesWithObserver(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	db.WithObserver(obs)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "upd"})

	_, _ = db.Query(context.Background()).
		Model(&qbItem{}).
		Where("val = ?", "upd").
		Updates(map[string]interface{}{"val": "updated"})
	assertObservedOp(t, obs, "update")
}

func TestQueryBuilder_DeleteWithObserver(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	db.WithObserver(obs)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "del"})

	_, _ = db.Query(context.Background()).
		Where("val = ?", "del").
		Delete(&qbItem{})
	assertObservedOp(t, obs, "delete")
}

func TestQueryBuilder_PluckWithObserver(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	db.WithObserver(obs)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "pluck"})

	var vals []string
	_, _ = db.Query(context.Background()).Model(&qbItem{}).Pluck("val", &vals)
	assertObservedOp(t, obs, "pluck")
}

func TestQueryBuilder_CreateInBatchesWithObserver(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	db.WithObserver(obs)
	setupTable(t, db)

	items := []qbItem{{Val: "b1"}, {Val: "b2"}}
	_, _ = db.Query(context.Background()).CreateInBatches(&items, 1)
	assertObservedOp(t, obs, "create_in_batches")
}

func TestQueryBuilder_FirstOrInitWithObserver(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	db.WithObserver(obs)
	setupTable(t, db)

	var item qbItem
	_ = db.Query(context.Background()).Where("val = ?", "missing").FirstOrInit(&item)
	assertObservedOp(t, obs, "first_or_init")
}

func TestQueryBuilder_FirstOrCreateWithObserver(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	db.WithObserver(obs)
	setupTable(t, db)

	var item qbItem
	_ = db.Query(context.Background()).Where(qbItem{Val: "fc"}).FirstOrCreate(&item)
	assertObservedOp(t, obs, "first_or_create")
}

// ── migrations (real MariaDB) ─────────────────────────────────────────────────

func TestAutoMigrate_WithMariaDB(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.DB().Migrator().DropTable(&qbItem{}))
	require.NoError(t, db.DB().Migrator().DropTable(&MigrationHistoryRecord{}))
	assert.NoError(t, db.AutoMigrate(&qbItem{}))

	var records []MigrationHistoryRecord
	require.NoError(t, db.DB().Find(&records).Error)
	assert.NotEmpty(t, records)
}

func TestGetMigrationStatus_WithMariaDB(t *testing.T) {
	db := newTestDB(t)
	setupMigrations(t, db)

	dir := t.TempDir()
	writeFile(t, dir, "20240101_schema_init.up.sql", "SELECT 1;")

	status, err := db.GetMigrationStatus(context.Background(), dir)
	require.NoError(t, err)
	require.Len(t, status, 1)
	assert.Equal(t, false, status[0]["applied"])
}

func TestMigrateUp_WithMariaDB(t *testing.T) {
	db := newTestDB(t)
	setupMigrations(t, db)

	dir := t.TempDir()
	writeFile(t, dir, "20240101_schema_create_test.up.sql", "SELECT 1;")
	writeFile(t, dir, "20240102_schema_add_col.up.sql", "SELECT 2;")

	assert.NoError(t, db.MigrateUp(context.Background(), dir))
	// Idempotent
	assert.NoError(t, db.MigrateUp(context.Background(), dir))
}

func TestMigrateDown_WithMariaDB(t *testing.T) {
	db := newTestDB(t)
	setupMigrations(t, db)

	// Insert a fake applied migration record
	record := MigrationHistoryRecord{
		ID: "20240101", Name: "create_test", Type: string(SchemaType),
		ExecutedAt: time.Now(), ExecutedBy: "test", Status: "success",
	}
	require.NoError(t, db.DB().Create(&record).Error)

	dir := t.TempDir()
	writeFile(t, dir, "20240101_schema_create_test.down.sql", "SELECT 1;")

	assert.NoError(t, db.MigrateDown(context.Background(), dir))
}

// ── fx / lifecycle (real MariaDB) ─────────────────────────────────────────────

func TestProvideClient(t *testing.T) {
	db := newTestDB(t)
	c := ProvideClient(db)
	assert.NotNil(t, c)
	assert.Implements(t, (*Client)(nil), c)
}

func TestGracefulShutdown_WithMariaDB(t *testing.T) {
	db := newTestDB(t)
	// First shutdown
	assert.NoError(t, db.GracefulShutdown())
	// Idempotent second call
	assert.NoError(t, db.GracefulShutdown())
}

// ── setup helpers (real MariaDB) ──────────────────────────────────────────────

func TestWithLogger(t *testing.T) {
	db := newTestDB(t)
	lg := &testLogger{}
	result := db.WithLogger(lg)
	assert.Same(t, db, result)
	assert.Equal(t, lg, db.logger)
}

func TestWithObserver_SetsField(t *testing.T) {
	db := newTestDB(t)
	obs := &TestObserver{}
	result := db.WithObserver(obs)
	assert.Same(t, db, result)
	assert.Equal(t, obs, db.observer)
}

func TestLogInfo_WithLogger(t *testing.T) {
	db := newTestDB(t)
	lg := &testLogger{}
	db.logger = lg
	db.logInfo(context.Background(), "info msg", nil)
	assert.Equal(t, 1, lg.infoCount)
}

func TestLogWarn_WithLogger(t *testing.T) {
	db := newTestDB(t)
	lg := &testLogger{}
	db.logger = lg
	db.logWarn(context.Background(), "warn msg", map[string]interface{}{"k": "v"})
	assert.Equal(t, 1, lg.warnCount)
}

func TestLogError_WithLogger(t *testing.T) {
	db := newTestDB(t)
	lg := &testLogger{}
	db.logger = lg
	db.logError(context.Background(), "err msg", nil)
	assert.Equal(t, 1, lg.errorCount)
}

func TestLogInfo_NoLogger(t *testing.T) {
	db := newTestDB(t)
	db.logInfo(context.Background(), "msg", nil) // must not panic
}

func TestLogWarn_NoLogger(t *testing.T) {
	db := newTestDB(t)
	db.logWarn(context.Background(), "msg", nil) // must not panic
}

func TestLogError_NoLogger(t *testing.T) {
	db := newTestDB(t)
	db.logError(context.Background(), "msg", nil) // must not panic
}

// ── locking clauses — verified on real MariaDB ────────────────────────────────

func TestQueryBuilder_LockingClauses(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "lock"})

	// FOR UPDATE
	var item qbItem
	err := db.DB().Transaction(func(tx *gorm.DB) error {
		qb := &mariadbQueryBuilder{db: tx, m: db, release: func() {}}
		return qb.Model(&qbItem{}).Where("val = ?", "lock").ForUpdate().First(&item)
	})
	assert.NoError(t, err)

	// FOR UPDATE SKIP LOCKED (MariaDB 10.6+, ok to skip if not supported)
	_ = db.DB().Transaction(func(tx *gorm.DB) error {
		qb := &mariadbQueryBuilder{db: tx, m: db, release: func() {}}
		return qb.Model(&qbItem{}).ForUpdateSkipLocked().Limit(1).Find(&[]qbItem{})
	})

	// Returning / ForNoKeyUpdate / ForKeyShare are no-ops on MariaDB — just ensure no panic
	qb := &mariadbQueryBuilder{db: db.DB(), m: db, release: func() {}}
	qb.Returning("id").ForNoKeyUpdate().ForKeyShare().Done()
}

// ── Clauses with real expression ──────────────────────────────────────────────

func TestQueryBuilder_ClausesWithExpression(t *testing.T) {
	db := newTestDB(t)
	setupTable(t, db)
	db.DB().Create(&qbItem{Val: "c1"})
	db.DB().Create(&qbItem{Val: "c2"})

	var items []qbItem
	qb := &mariadbQueryBuilder{db: db.DB(), m: db, release: func() {}}
	err := qb.Clauses(clause.OrderBy{
		Columns: []clause.OrderByColumn{{Column: clause.Column{Name: "val"}, Desc: true}},
	}).Find(&items)
	assert.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "c2", items[0].Val)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// qbItem is a named model used across query builder tests to avoid
// anonymous-struct AutoMigrate issues.
type qbItem struct {
	gorm.Model
	Val string
}

// setupTable drops and re-creates the qb_items table so each test starts clean.
func setupTable(t *testing.T, db *MariaDB) {
	t.Helper()
	require.NoError(t, db.DB().Migrator().DropTable(&qbItem{}))
	require.NoError(t, db.DB().AutoMigrate(&qbItem{}))
}

// setupMigrations drops and re-creates the migration_history_records table.
func setupMigrations(t *testing.T, db *MariaDB) {
	t.Helper()
	require.NoError(t, db.DB().Migrator().DropTable(&MigrationHistoryRecord{}))
	require.NoError(t, db.DB().AutoMigrate(&MigrationHistoryRecord{}))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)) //nolint:gosec
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
