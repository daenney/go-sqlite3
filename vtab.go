package sqlite3

import (
	"context"
	"reflect"

	"github.com/ncruces/go-sqlite3/internal/util"
	"github.com/tetratelabs/wazero/api"
)

// CreateModule registers a new virtual table module name.
// If create is nil, the virtual table is eponymous.
//
// https://sqlite.org/c3ref/create_module.html
func CreateModule[T VTab](db *Conn, name string, create, connect VTabConstructor[T]) error {
	var flags int

	const (
		VTAB_CREATOR     = 0x01
		VTAB_DESTROYER   = 0x02
		VTAB_UPDATER     = 0x04
		VTAB_RENAMER     = 0x08
		VTAB_OVERLOADER  = 0x10
		VTAB_CHECKER     = 0x20
		VTAB_TX          = 0x40
		VTAB_SAVEPOINTER = 0x80
	)

	if create != nil {
		flags |= VTAB_CREATOR
	}

	vtab := reflect.TypeOf(connect).Out(0)
	if implements[VTabDestroyer](vtab) {
		flags |= VTAB_DESTROYER
	}
	if implements[VTabUpdater](vtab) {
		flags |= VTAB_UPDATER
	}
	if implements[VTabRenamer](vtab) {
		flags |= VTAB_RENAMER
	}
	if implements[VTabOverloader](vtab) {
		flags |= VTAB_OVERLOADER
	}
	if implements[VTabChecker](vtab) {
		flags |= VTAB_CHECKER
	}
	if implements[VTabTx](vtab) {
		flags |= VTAB_TX
	}
	if implements[VTabSavepointer](vtab) {
		flags |= VTAB_SAVEPOINTER
	}

	defer db.arena.reset()
	namePtr := db.arena.string(name)
	modulePtr := util.AddHandle(db.ctx, module[T]{create, connect})
	r := db.call(db.api.createModule, uint64(db.handle),
		uint64(namePtr), uint64(flags), uint64(modulePtr))
	return db.error(r)
}

func implements[T any](typ reflect.Type) bool {
	var ptr *T
	return typ.Implements(reflect.TypeOf(ptr).Elem())
}

func (c *Conn) DeclareVtab(sql string) error {
	// defer c.arena.reset()
	sqlPtr := c.arena.string(sql)
	r := c.call(c.api.declareVTab, uint64(c.handle), uint64(sqlPtr))
	return c.error(r)
}

// VTabConstructor is a virtual table constructor function.
type VTabConstructor[T VTab] func(db *Conn, arg ...string) (T, error)

type module[T VTab] [2]VTabConstructor[T]

// A VTab describes a particular instance of the virtual table.
// A VTab may optionally implement [io.Closer] to free resources.
//
// https://sqlite.org/c3ref/vtab.html
type VTab interface {
	// https://sqlite.org/vtab.html#xbestindex
	BestIndex(*IndexInfo) error
	// https://sqlite.org/vtab.html#xopen
	Open() (VTabCursor, error)
}

// A VTabDestroyer allows a virtual table to drop persistent state.
type VTabDestroyer interface {
	VTab
	// https://sqlite.org/vtab.html#sqlite3_module.xDestroy
	Destroy() error
}

// A VTabUpdater allows a virtual table to be updated.
type VTabUpdater interface {
	VTab
	// https://sqlite.org/vtab.html#xupdate
	Update(arg ...Value) (rowid int64, err error)
}

// A VTabRenamer allows a virtual table to be renamed.
type VTabRenamer interface {
	VTab
	// https://sqlite.org/vtab.html#xrename
	Rename(new string) error
}

// A VTabOverloader allows a virtual table to overload SQL functions.
type VTabOverloader interface {
	VTab
	// https://sqlite.org/vtab.html#xfindfunction
	FindFunction(arg int, name string) (func(ctx Context, arg ...Value), IndexConstraintOp)
}

// A VTabChecker allows a virtual table to report errors
// to the PRAGMA integrity_check PRAGMA quick_check commands.
//
// Integrity should return an error if it finds problems in the content of the virtual table,
// but should avoid returning a (wrapped) [Error], [ErrorCode] or [ExtendedErrorCode],
// as those indicate the Integrity method itself encountered problems
// while trying to evaluate the virtual table content.
type VTabChecker interface {
	VTab
	// https://sqlite.org/vtab.html#xintegrity
	Integrity(schema, table string, flags int) error
}

// A VTabTx allows a virtual table to implement
// transactions with two-phase commit.
type VTabTx interface {
	VTab
	// https://sqlite.org/vtab.html#xBegin
	Begin() error
	// https://sqlite.org/vtab.html#xsync
	Sync() error
	// https://sqlite.org/vtab.html#xcommit
	Commit() error
	// https://sqlite.org/vtab.html#xrollback
	Rollback() error
}

// A VTabSavepointer allows a virtual table to implement
// nested transactions.
//
// https://sqlite.org/vtab.html#xsavepoint
type VTabSavepointer interface {
	VTabTx
	Savepoint(id int) error
	Release(id int) error
	RollbackTo(id int) error
}

// A VTabCursor describes cursors that point
// into the virtual table and are used
// to loop through the virtual table.
// A VTabCursor may optionally implement
// [io.Closer] to free resources.
//
// http://sqlite.org/c3ref/vtab_cursor.html
type VTabCursor interface {
	// https://sqlite.org/vtab.html#xfilter
	Filter(idxNum int, idxStr string, arg ...Value) error
	// https://sqlite.org/vtab.html#xnext
	Next() error
	// https://sqlite.org/vtab.html#xeof
	EOF() bool
	// https://sqlite.org/vtab.html#xcolumn
	Column(ctx *Context, n int) error
	// https://sqlite.org/vtab.html#xrowid
	RowID() (int64, error)
}

// An IndexInfo describes virtual table indexing information.
//
// https://sqlite.org/c3ref/index_info.html
type IndexInfo struct {
	// Inputs
	Constraint  []IndexConstraint
	OrderBy     []IndexOrderBy
	ColumnsUsed int64
	// Outputs
	ConstraintUsage []IndexConstraintUsage
	IdxNum          int
	IdxStr          string
	IdxFlags        IndexScanFlag
	OrderByConsumed bool
	EstimatedCost   float64
	EstimatedRows   int64
	// Internal
	c      *Conn
	handle uint32
}

// An IndexConstraint describes virtual table indexing constraint information.
//
// https://sqlite.org/c3ref/index_info.html
type IndexConstraint struct {
	Column int
	Op     IndexConstraintOp
	Usable bool
}

// An IndexOrderBy describes virtual table indexing order by information.
//
// https://sqlite.org/c3ref/index_info.html
type IndexOrderBy struct {
	Column int
	Desc   bool
}

// An IndexConstraintUsage describes how virtual table indexing constraints will be used.
//
// https://sqlite.org/c3ref/index_info.html
type IndexConstraintUsage struct {
	ArgvIndex int
	Omit      bool
}

// RHSValue returns the value of the right-hand operand of a constraint
// if the right-hand operand is known.
//
// https://sqlite.org/c3ref/vtab_rhs_value.html
func (idx *IndexInfo) RHSValue(column int) (*Value, error) {
	// defer idx.c.arena.reset()
	valPtr := idx.c.arena.new(ptrlen)
	r := idx.c.call(idx.c.api.vtabRHSValue,
		uint64(idx.handle), uint64(column), uint64(valPtr))
	if err := idx.c.error(r); err != nil {
		return nil, err
	}
	return &Value{
		sqlite: idx.c.sqlite,
		handle: util.ReadUint32(idx.c.mod, valPtr),
	}, nil
}

func (idx *IndexInfo) load() {
	// https://sqlite.org/c3ref/index_info.html
	mod := idx.c.mod
	ptr := idx.handle

	idx.Constraint = make([]IndexConstraint, util.ReadUint32(mod, ptr+0))
	idx.ConstraintUsage = make([]IndexConstraintUsage, util.ReadUint32(mod, ptr+0))
	idx.OrderBy = make([]IndexOrderBy, util.ReadUint32(mod, ptr+8))

	constraintPtr := util.ReadUint32(mod, ptr+4)
	for i := range idx.Constraint {
		idx.Constraint[i] = IndexConstraint{
			Column: int(int32(util.ReadUint32(mod, constraintPtr+0))),
			Op:     IndexConstraintOp(util.ReadUint8(mod, constraintPtr+4)),
			Usable: util.ReadUint8(mod, constraintPtr+5) != 0,
		}
		constraintPtr += 12
	}

	orderByPtr := util.ReadUint32(mod, ptr+12)
	for i := range idx.OrderBy {
		idx.OrderBy[i] = IndexOrderBy{
			Column: int(int32(util.ReadUint32(mod, orderByPtr+0))),
			Desc:   util.ReadUint8(mod, orderByPtr+4) != 0,
		}
		orderByPtr += 8
	}

	idx.EstimatedCost = util.ReadFloat64(mod, ptr+40)
	idx.EstimatedRows = int64(util.ReadUint64(mod, ptr+48))
	idx.ColumnsUsed = int64(util.ReadUint64(mod, ptr+64))
}

func (idx *IndexInfo) save() {
	// https://sqlite.org/c3ref/index_info.html
	mod := idx.c.mod
	ptr := idx.handle

	usagePtr := util.ReadUint32(mod, ptr+16)
	for _, usage := range idx.ConstraintUsage {
		util.WriteUint32(mod, usagePtr+0, uint32(usage.ArgvIndex))
		if usage.Omit {
			util.WriteUint8(mod, usagePtr+4, 1)
		}
		usagePtr += 8
	}

	util.WriteUint32(mod, ptr+20, uint32(idx.IdxNum))
	if idx.IdxStr != "" {
		util.WriteUint32(mod, ptr+24, idx.c.newString(idx.IdxStr))
		util.WriteUint32(mod, ptr+28, 1)
	}
	if idx.OrderByConsumed {
		util.WriteUint32(mod, ptr+32, 1)
	}
	util.WriteFloat64(mod, ptr+40, idx.EstimatedCost)
	util.WriteUint64(mod, ptr+48, uint64(idx.EstimatedRows))
	util.WriteUint32(mod, ptr+56, uint32(idx.IdxFlags))
}

// IndexConstraintOp is a virtual table constraint operator code.
//
// https://sqlite.org/c3ref/c_index_constraint_eq.html
type IndexConstraintOp uint8

const (
	INDEX_CONSTRAINT_EQ        IndexConstraintOp = 2
	INDEX_CONSTRAINT_GT        IndexConstraintOp = 4
	INDEX_CONSTRAINT_LE        IndexConstraintOp = 8
	INDEX_CONSTRAINT_LT        IndexConstraintOp = 16
	INDEX_CONSTRAINT_GE        IndexConstraintOp = 32
	INDEX_CONSTRAINT_MATCH     IndexConstraintOp = 64
	INDEX_CONSTRAINT_LIKE      IndexConstraintOp = 65
	INDEX_CONSTRAINT_GLOB      IndexConstraintOp = 66
	INDEX_CONSTRAINT_REGEXP    IndexConstraintOp = 67
	INDEX_CONSTRAINT_NE        IndexConstraintOp = 68
	INDEX_CONSTRAINT_ISNOT     IndexConstraintOp = 69
	INDEX_CONSTRAINT_ISNOTNULL IndexConstraintOp = 70
	INDEX_CONSTRAINT_ISNULL    IndexConstraintOp = 71
	INDEX_CONSTRAINT_IS        IndexConstraintOp = 72
	INDEX_CONSTRAINT_LIMIT     IndexConstraintOp = 73
	INDEX_CONSTRAINT_OFFSET    IndexConstraintOp = 74
	INDEX_CONSTRAINT_FUNCTION  IndexConstraintOp = 150
)

// IndexScanFlag is a virtual table scan flag.
//
// https://sqlite.org/c3ref/c_index_scan_unique.html
type IndexScanFlag uint32

const (
	INDEX_SCAN_UNIQUE IndexScanFlag = 1
)

func vtabModuleCallback(i int) func(_ context.Context, _ api.Module, _, _, _, _, _ uint32) uint32 {
	return func(ctx context.Context, mod api.Module, pMod, argc, argv, ppVTab, pzErr uint32) uint32 {
		arg := make([]reflect.Value, 1+argc)
		arg[0] = reflect.ValueOf(ctx.Value(connKey{}))

		for i := uint32(0); i < argc; i++ {
			ptr := util.ReadUint32(mod, argv+i*ptrlen)
			arg[i+1] = reflect.ValueOf(util.ReadString(mod, ptr, _MAX_STRING))
		}

		module := vtabGetHandle(ctx, mod, pMod)
		res := reflect.ValueOf(module).Index(i).Call(arg)
		err, _ := res[1].Interface().(error)
		if err == nil {
			vtabPutHandle(ctx, mod, ppVTab, res[0].Interface())
		}

		return vtabError(ctx, mod, pzErr, _PTR_ERROR, err)
	}
}

func vtabDisconnectCallback(ctx context.Context, mod api.Module, pVTab uint32) uint32 {
	err := vtabDelHandle(ctx, mod, pVTab)
	return vtabError(ctx, mod, 0, _PTR_ERROR, err)
}

func vtabDestroyCallback(ctx context.Context, mod api.Module, pVTab uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabDestroyer)
	err := vtab.Destroy()
	if cerr := vtabDelHandle(ctx, mod, pVTab); err == nil {
		err = cerr
	}
	return vtabError(ctx, mod, 0, _PTR_ERROR, err)
}

func vtabBestIndexCallback(ctx context.Context, mod api.Module, pVTab, pIdxInfo uint32) uint32 {
	var info IndexInfo
	info.handle = pIdxInfo
	info.c = ctx.Value(connKey{}).(*Conn)
	info.load()

	vtab := vtabGetHandle(ctx, mod, pVTab).(VTab)
	err := vtab.BestIndex(&info)

	info.save()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabUpdateCallback(ctx context.Context, mod api.Module, pVTab, argc, argv, pRowID uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabUpdater)

	db := ctx.Value(connKey{}).(*Conn)
	args := callbackArgs(db, argc, argv)
	rowID, err := vtab.Update(args...)
	if err == nil {
		util.WriteUint64(mod, pRowID, uint64(rowID))
	}

	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabRenameCallback(ctx context.Context, mod api.Module, pVTab, zNew uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabRenamer)
	err := vtab.Rename(util.ReadString(mod, zNew, _MAX_STRING))
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabFindFuncCallback(ctx context.Context, mod api.Module, pVTab, nArg, zName, pxFunc uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabOverloader)
	fn, op := vtab.FindFunction(int(nArg), util.ReadString(mod, zName, _MAX_STRING))
	if fn != nil {
		handle := util.AddHandle(ctx, fn)
		util.WriteUint32(mod, pxFunc, handle)
		if op == 0 {
			op = 1
		}
	}
	return uint32(op)
}

func vtabIntegrityCallback(ctx context.Context, mod api.Module, pVTab, zSchema, zTabName, mFlags, pzErr uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabChecker)
	schema := util.ReadString(mod, zSchema, _MAX_STRING)
	table := util.ReadString(mod, zTabName, _MAX_STRING)
	err := vtab.Integrity(schema, table, int(mFlags))
	// xIntegrity should return OK - even if it finds problems in the content of the virtual table.
	// https://sqlite.org/vtab.html#xintegrity
	vtabError(ctx, mod, pzErr, _PTR_ERROR, err)
	_, code := errorCode(err, _OK)
	return code
}

func vtabBeginCallback(ctx context.Context, mod api.Module, pVTab uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabTx)
	err := vtab.Begin()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabSyncCallback(ctx context.Context, mod api.Module, pVTab uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabTx)
	err := vtab.Sync()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabCommitCallback(ctx context.Context, mod api.Module, pVTab uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabTx)
	err := vtab.Commit()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabRollbackCallback(ctx context.Context, mod api.Module, pVTab uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabTx)
	err := vtab.Rollback()
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabSavepointCallback(ctx context.Context, mod api.Module, pVTab, id uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabSavepointer)
	err := vtab.Savepoint(int(id))
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabReleaseCallback(ctx context.Context, mod api.Module, pVTab, id uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabSavepointer)
	err := vtab.Release(int(id))
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func vtabRollbackToCallback(ctx context.Context, mod api.Module, pVTab, id uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTabSavepointer)
	err := vtab.RollbackTo(int(id))
	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func cursorOpenCallback(ctx context.Context, mod api.Module, pVTab, ppCur uint32) uint32 {
	vtab := vtabGetHandle(ctx, mod, pVTab).(VTab)

	cursor, err := vtab.Open()
	if err == nil {
		vtabPutHandle(ctx, mod, ppCur, cursor)
	}

	return vtabError(ctx, mod, pVTab, _VTAB_ERROR, err)
}

func cursorCloseCallback(ctx context.Context, mod api.Module, pCur uint32) uint32 {
	err := vtabDelHandle(ctx, mod, pCur)
	return vtabError(ctx, mod, 0, _VTAB_ERROR, err)
}

func cursorFilterCallback(ctx context.Context, mod api.Module, pCur, idxNum, idxStr, argc, argv uint32) uint32 {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)
	db := ctx.Value(connKey{}).(*Conn)
	args := callbackArgs(db, argc, argv)
	var idxName string
	if idxStr != 0 {
		idxName = util.ReadString(mod, idxStr, _MAX_STRING)
	}
	err := cursor.Filter(int(idxNum), idxName, args...)
	return vtabError(ctx, mod, pCur, _CURSOR_ERROR, err)
}

func cursorEOFCallback(ctx context.Context, mod api.Module, pCur uint32) uint32 {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)
	if cursor.EOF() {
		return 1
	}
	return 0
}

func cursorNextCallback(ctx context.Context, mod api.Module, pCur uint32) uint32 {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)
	err := cursor.Next()
	return vtabError(ctx, mod, pCur, _CURSOR_ERROR, err)
}

func cursorColumnCallback(ctx context.Context, mod api.Module, pCur, pCtx, n uint32) uint32 {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)
	db := ctx.Value(connKey{}).(*Conn)
	err := cursor.Column(&Context{db, pCtx}, int(n))
	return vtabError(ctx, mod, pCur, _CURSOR_ERROR, err)
}

func cursorRowIDCallback(ctx context.Context, mod api.Module, pCur, pRowID uint32) uint32 {
	cursor := vtabGetHandle(ctx, mod, pCur).(VTabCursor)

	rowID, err := cursor.RowID()
	if err == nil {
		util.WriteUint64(mod, pRowID, uint64(rowID))
	}

	return vtabError(ctx, mod, pCur, _CURSOR_ERROR, err)
}

const (
	_PTR_ERROR = iota
	_VTAB_ERROR
	_CURSOR_ERROR
)

func vtabError(ctx context.Context, mod api.Module, ptr, kind uint32, err error) uint32 {
	msg, code := errorCode(err, ERROR)
	if msg != "" && ptr != 0 {
		switch kind {
		case _VTAB_ERROR:
			ptr = ptr + 8
		case _CURSOR_ERROR:
			ptr = util.ReadUint32(mod, ptr) + 8
		}
		db := ctx.Value(connKey{}).(*Conn)
		util.WriteUint32(mod, ptr, db.newString(msg))
	}
	return code
}

func vtabGetHandle(ctx context.Context, mod api.Module, ptr uint32) any {
	const handleOffset = 4
	handle := util.ReadUint32(mod, ptr-handleOffset)
	return util.GetHandle(ctx, handle)
}

func vtabDelHandle(ctx context.Context, mod api.Module, ptr uint32) error {
	const handleOffset = 4
	handle := util.ReadUint32(mod, ptr-handleOffset)
	return util.DelHandle(ctx, handle)
}

func vtabPutHandle(ctx context.Context, mod api.Module, pptr uint32, val any) {
	const handleOffset = 4
	handle := util.AddHandle(ctx, val)
	ptr := util.ReadUint32(mod, pptr)
	util.WriteUint32(mod, ptr-handleOffset, handle)
}
