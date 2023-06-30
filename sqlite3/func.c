#include <string.h>

#include "sqlite3.h"

int go_compare(void *, int, const void *, int, const void *);
void go_func(sqlite3_context *, int, sqlite3_value **);
void go_step(sqlite3_context *, int, sqlite3_value **);
void go_final(sqlite3_context *);
void go_value(sqlite3_context *);
void go_inverse(sqlite3_context *, int, sqlite3_value **);
void go_destroy(void *);

int sqlite3_create_go_collation(sqlite3 *db, const char *zName, void *pApp) {
  return sqlite3_create_collation_v2(db, zName, SQLITE_UTF8, pApp, go_compare,
                                     go_destroy);
}

int sqlite3_create_go_function(sqlite3 *db, const char *zName, int nArg,
                               int flags, void *pApp) {
  return sqlite3_create_function_v2(db, zName, nArg, SQLITE_UTF8 | flags, pApp,
                                    go_func, NULL, NULL, go_destroy);
}

int sqlite3_create_go_window_function(sqlite3 *db, const char *zName, int nArg,
                                      int flags, void *pApp) {
  return sqlite3_create_window_function(db, zName, nArg, SQLITE_UTF8 | flags,
                                        pApp, go_step, go_final, NULL, NULL,
                                        go_destroy);
}

int sqlite3_create_go_aggregate_function(sqlite3 *db, const char *zName,
                                         int nArg, int flags, void *pApp) {
  return sqlite3_create_window_function(db, zName, nArg, SQLITE_UTF8 | flags,
                                        pApp, go_step, go_final, go_value,
                                        go_inverse, go_destroy);
}