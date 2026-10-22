/* Minimal, binary-safe JSON support for the engine line protocol: parsing a
 * flat request object of string/number fields and building response text.
 * Strings keep their length, so embedded NUL bytes survive (PHP allows them
 * in both patterns and subjects). */
#ifndef LAZYREGEX_JSON_H
#define LAZYREGEX_JSON_H

#include <stddef.h>

/* A byte string; p is always NUL-terminated (p[n] == 0). */
typedef struct {
	char *p;
	size_t n;
} jstr;

/* One protocol request. Absent string fields are empty (never NULL). */
typedef struct {
	long id;
	jstr op, pattern, flags, text, mode, replacement;
} jreq;

/* Parses one request line. Returns 0 on success, -1 on malformed input. */
int jreq_parse(const char *line, size_t len, jreq *r);
void jreq_free(jreq *r);

/* Growable output buffer. */
typedef struct {
	char *p;
	size_t n, cap;
} jbuf;

void jb_bytes(jbuf *b, const char *s, size_t n);
void jb_str(jbuf *b, const char *s); /* raw text, not quoted */
void jb_long(jbuf *b, long v);
void jb_quote(jbuf *b, const char *s, size_t n); /* JSON string literal */

#endif
