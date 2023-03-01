#!/usr/bin/env bash
set -eo pipefail

cd -P -- "$(dirname -- "$0")"

# download SQLite
../sqlite3/download.sh

# build SQLite
zig cc --target=wasm32-wasi -flto -g0 -Os \
  -o sqlite3.wasm ../sqlite3/amalg.c \
	-mmutable-globals \
	-mbulk-memory -mreference-types \
	-mnontrapping-fptoint -msign-ext \
	-D_HAVE_SQLITE_CONFIG_H \
	-Wl,--export=free \
	-Wl,--export=malloc \
	-Wl,--export=malloc_destructor \
	-Wl,--export=sqlite3_errcode \
	-Wl,--export=sqlite3_errstr \
	-Wl,--export=sqlite3_errmsg \
	-Wl,--export=sqlite3_error_offset \
	-Wl,--export=sqlite3_open_v2 \
	-Wl,--export=sqlite3_close \
	-Wl,--export=sqlite3_prepare_v3 \
	-Wl,--export=sqlite3_finalize \
	-Wl,--export=sqlite3_reset \
	-Wl,--export=sqlite3_step \
	-Wl,--export=sqlite3_exec \
	-Wl,--export=sqlite3_clear_bindings \
	-Wl,--export=sqlite3_bind_parameter_count \
	-Wl,--export=sqlite3_bind_parameter_index \
	-Wl,--export=sqlite3_bind_parameter_name \
	-Wl,--export=sqlite3_bind_null \
	-Wl,--export=sqlite3_bind_int64 \
	-Wl,--export=sqlite3_bind_double \
	-Wl,--export=sqlite3_bind_text64 \
	-Wl,--export=sqlite3_bind_blob64 \
	-Wl,--export=sqlite3_bind_zeroblob64 \
	-Wl,--export=sqlite3_column_count \
	-Wl,--export=sqlite3_column_name \
	-Wl,--export=sqlite3_column_type \
	-Wl,--export=sqlite3_column_int64 \
	-Wl,--export=sqlite3_column_double \
	-Wl,--export=sqlite3_column_text \
	-Wl,--export=sqlite3_column_blob \
	-Wl,--export=sqlite3_column_bytes \
	-Wl,--export=sqlite3_blob_open \
	-Wl,--export=sqlite3_blob_close \
	-Wl,--export=sqlite3_blob_bytes \
	-Wl,--export=sqlite3_blob_read \
	-Wl,--export=sqlite3_blob_write \
	-Wl,--export=sqlite3_blob_reopen \
	-Wl,--export=sqlite3_get_autocommit \
	-Wl,--export=sqlite3_last_insert_rowid \
	-Wl,--export=sqlite3_changes64 \
	-Wl,--export=sqlite3_unlock_notify \
	-Wl,--export=sqlite3_backup_init \
	-Wl,--export=sqlite3_backup_step \
	-Wl,--export=sqlite3_backup_finish \
	-Wl,--export=sqlite3_backup_remaining \
	-Wl,--export=sqlite3_backup_pagecount \
	-Wl,--export=sqlite3_interrupt_offset \