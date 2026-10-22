/* lazyregex PCRE engine: a WASI command speaking the NDJSON line protocol
 * (one request object per stdin line, one response object per stdout line).
 *
 * It reproduces PHP 8.4's ext/pcre/php_pcre.c, which is the referee for the
 * PCRE flavor: the user's pattern is wrapped as PHP code would write it,
 * '/' . addcslashes($pattern, '/') . '/' . $modifiers, and then
 *   match:   preg_match / preg_match_all (PREG_SET_ORDER | PREG_OFFSET_CAPTURE
 *            | PREG_UNMATCHED_AS_NULL)
 *   replace: preg_replace($re, $replacement, $text, g ? -1 : 1)
 *   split:   preg_split($re, $text)            (limit -1, no flags)
 * Errors carry PHP's warning text without the "preg_xxx(): " prefix, or
 * preg_last_error_msg() for match-time failures. Function names below refer
 * to their php_pcre.c counterparts. */
#define PCRE2_CODE_UNIT_WIDTH 8
#include <pcre2.h>

#include <ctype.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "json.h"

/* PHP INI defaults: pcre.backtrack_limit, pcre.recursion_limit. */
#define PHP_BACKTRACK_LIMIT 1000000
#define PHP_RECURSION_LIMIT 100000

static pcre2_compile_context *cctx;
static pcre2_match_context *mctx;

/* A compiled pattern (php's pcre_cache_entry). */
typedef struct {
	pcre2_code *re;
	uint32_t compile_options;
	uint32_t capture_count;
} pce_t;

/* The outcome of an evaluation: either a result body or an error. */
typedef struct {
	jbuf body;        /* JSON members after "ok":true, each prefixed by ',' */
	char err[256];    /* non-empty on failure */
	long err_offset;  /* offset in the user's pattern, -1 if none */
} outcome;

static void fail(outcome *o, const char *msg, long offset) {
	snprintf(o->err, sizeof o->err, "%s", msg);
	o->err_offset = offset;
}

/* pcre_handle_exec_error + php_pcre_get_error_msg */
static const char *exec_error_msg(int rc) {
	switch (rc) {
	case PCRE2_ERROR_MATCHLIMIT:
		return "Backtrack limit exhausted";
	case PCRE2_ERROR_DEPTHLIMIT:
		return "Recursion limit exhausted";
	case PCRE2_ERROR_BADUTFOFFSET:
		return "The offset did not correspond to the beginning of a valid UTF-8 code point";
	}
	if (rc <= PCRE2_ERROR_UTF8_ERR1 && rc >= PCRE2_ERROR_UTF8_ERR21)
		return "Malformed UTF-8 characters, possibly incorrectly encoded";
	return "Internal error";
}

/* calculate_unit_length: s is NUL-terminated, so the scan stops at the end. */
static size_t unit_length(const pce_t *pce, const char *start) {
	if (!(pce->compile_options & PCRE2_UTF)) return 1;
	const char *end = start;
	while ((*++end & 0xC0) == 0x80)
		;
	return (size_t)(end - start);
}

/* pcre_get_compiled_regex_cache_ex for the regex PHP would see. Returns 0 on
 * success. */
static int compile(const jstr *pattern, const jstr *flags, pce_t *pce, outcome *o) {
	/* $rx = '/' . addcslashes($pattern, '/') . '/' . str_replace('g', '', $flags);
	 * rawpos[i] maps byte i of the escaped pattern back to the user's pattern. */
	size_t cap = 2 * pattern->n + flags->n + 3;
	char *regex = malloc(cap);
	size_t *rawpos = malloc((2 * pattern->n + 1) * sizeof *rawpos);
	if (!regex || !rawpos) {
		free(regex);
		free(rawpos);
		fail(o, "Internal error", -1);
		return -1;
	}
	size_t n = 0, e = 0;
	regex[n++] = '/';
	for (size_t i = 0; i < pattern->n; i++) {
		if (pattern->p[i] == '/') {
			rawpos[e++] = i;
			regex[n++] = '\\';
		}
		rawpos[e++] = i;
		regex[n++] = pattern->p[i];
	}
	rawpos[e] = pattern->n;
	regex[n++] = '/';
	for (size_t i = 0; i < flags->n; i++)
		if (flags->p[i] != 'g') regex[n++] = flags->p[i];
	regex[n] = 0;

	int rc = -1;
	char msg[256];
	const char *p = regex, *end_p = regex + n;
	while (isspace((unsigned char)*p)) p++;
	if (p >= end_p) {
		fail(o, "Empty regular expression", -1);
		goto out;
	}
	char delimiter = *p++;
	if (isalnum((unsigned char)delimiter) || delimiter == '\\' || delimiter == '\0') {
		fail(o, "Delimiter must not be alphanumeric, backslash, or NUL byte", -1);
		goto out;
	}
	/* The delimiter is always '/', so start and end delimiters are equal. */
	const char *pp = p;
	while (pp < end_p) {
		if (*pp == '\\' && pp + 1 < end_p) pp++;
		else if (*pp == delimiter) break;
		pp++;
	}
	if (pp >= end_p) {
		snprintf(msg, sizeof msg, "No ending delimiter '%c' found", delimiter);
		fail(o, msg, -1);
		goto out;
	}
	const char *pat = p;
	size_t pat_len = (size_t)(pp - p);
	pp++;

	uint32_t coptions = 0;
	uint32_t eoptions = PCRE2_EXTRA_ALLOW_LOOKAROUND_BSK;
	while (pp < end_p) {
		switch (*pp++) {
		case 'i': coptions |= PCRE2_CASELESS; break;
		case 'm': coptions |= PCRE2_MULTILINE; break;
		case 'n': coptions |= PCRE2_NO_AUTO_CAPTURE; break;
		case 's': coptions |= PCRE2_DOTALL; break;
		case 'x': coptions |= PCRE2_EXTENDED; break;
		case 'A': coptions |= PCRE2_ANCHORED; break;
		case 'D': coptions |= PCRE2_DOLLAR_ENDONLY; break;
		case 'r': eoptions |= PCRE2_EXTRA_CASELESS_RESTRICT; break;
		case 'S': break;
		case 'X': break;
		case 'U': coptions |= PCRE2_UNGREEDY; break;
		case 'u': coptions |= PCRE2_UTF | PCRE2_UCP | PCRE2_NEVER_BACKSLASH_C; break;
		case 'J': coptions |= PCRE2_DUPNAMES; break;
		case ' ':
		case '\n':
		case '\r':
			break;
		default:
			if (pp[-1]) snprintf(msg, sizeof msg, "Unknown modifier '%c'", pp[-1]);
			else snprintf(msg, sizeof msg, "NUL byte is not a valid modifier");
			fail(o, msg, -1);
			goto out;
		}
	}

	pcre2_set_compile_extra_options(cctx, eoptions);
	int errnumber;
	PCRE2_SIZE erroffset;
	pcre2_code *re = pcre2_compile((PCRE2_SPTR)pat, pat_len, coptions, &errnumber, &erroffset, cctx);
	if (!re) {
		char error[128];
		const char *err_msg = error;
		if (errnumber == PCRE2_ERROR_BACKSLASH_C_CALLER_DISABLED)
			err_msg = "using \\C is incompatible with the 'u' modifier";
		else
			pcre2_get_error_message(errnumber, (PCRE2_UCHAR *)error, sizeof error);
		snprintf(msg, sizeof msg, "Compilation failed: %s at offset %zu", err_msg, (size_t)erroffset);
		/* The compiled text is exactly the escaped pattern (otherwise the
		 * delimiter scan or a modifier would have failed first). */
		fail(o, msg, erroffset <= e ? (long)rawpos[erroffset] : -1);
		goto out;
	}
	pce->re = re;
	pce->compile_options = coptions;
	pcre2_pattern_info(re, PCRE2_INFO_CAPTURECOUNT, &pce->capture_count);
	rc = 0;
out:
	free(regex);
	free(rawpos);
	return rc;
}

/* Group names as gen_php.php reads them off a PHP match array: the name key
 * precedes its group's index key, and with duplicate names (J) PHP keeps
 * only the first position of the key, so later groups appear unnamed. */
static char **group_names(const pce_t *pce) {
	uint32_t count, size;
	PCRE2_SPTR table;
	char **names = calloc(pce->capture_count + 1, sizeof *names);
	pcre2_pattern_info(pce->re, PCRE2_INFO_NAMECOUNT, &count);
	pcre2_pattern_info(pce->re, PCRE2_INFO_NAMEENTRYSIZE, &size);
	pcre2_pattern_info(pce->re, PCRE2_INFO_NAMETABLE, &table);
	for (uint32_t i = 0; i < count; i++, table += size) {
		uint32_t idx = (uint32_t)table[0] << 8 | table[1];
		names[idx] = (char *)(table + 2);
	}
	for (uint32_t i = 1; i <= pce->capture_count; i++)
		for (uint32_t j = 1; j < i && names[i]; j++)
			if (names[j] && strcmp(names[j], names[i]) == 0) names[i] = NULL;
	return names;
}

static void emit_span(jbuf *b, long s, long e) {
	jb_str(b, "[");
	jb_long(b, s);
	jb_str(b, ",");
	jb_long(b, e);
	jb_str(b, "]");
}

/* populate_subpat_array with PREG_OFFSET_CAPTURE | PREG_UNMATCHED_AS_NULL:
 * every group is present, unset ones as null. */
static void emit_match(jbuf *b, const pce_t *pce, char **names, const PCRE2_SIZE *ov, int count, int first) {
	jb_str(b, first ? "{\"span\":" : ",{\"span\":");
	emit_span(b, (long)ov[0], (long)ov[1]);
	jb_str(b, ",\"groups\":[");
	for (uint32_t i = 1; i <= pce->capture_count; i++) {
		int matched = (int)i < count && ov[2 * i] != PCRE2_UNSET;
		if (i > 1) jb_str(b, ",");
		jb_str(b, "{\"index\":");
		jb_long(b, (long)i);
		jb_str(b, ",\"name\":");
		const char *nm = names[i] ? names[i] : "";
		jb_quote(b, nm, strlen(nm));
		jb_str(b, matched ? ",\"matched\":true,\"span\":" : ",\"matched\":false,\"span\":");
		if (matched) emit_span(b, (long)ov[2 * i], (long)ov[2 * i + 1]);
		else emit_span(b, -1, -1);
		jb_str(b, "}");
	}
	jb_str(b, "]}");
}

/* php_pcre_match_impl (start offset 0). */
static void do_match(const pce_t *pce, const jstr *subj, int global, pcre2_match_data *md, outcome *o) {
	PCRE2_SPTR subject = (PCRE2_SPTR)subj->p;
	size_t subject_len = subj->n;
	PCRE2_SIZE *ov = pcre2_get_ovector_pointer(md);
	char **names = group_names(pce);
	jbuf *b = &o->body;
	int nmatches = 0;
	PCRE2_SIZE start_offset2 = 0;
	uint32_t options = (pce->compile_options & PCRE2_UTF) ? 0 : PCRE2_NO_UTF_CHECK;

	jb_str(b, ",\"matches\":[");
	int count = pcre2_match(pce->re, subject, subject_len, start_offset2, options, md, mctx);
	for (;;) {
		if (count >= 0) {
		matched:
			if (ov[1] < ov[0]) {
				fail(o, "Get subpatterns list failed", -1);
				goto done;
			}
			emit_match(b, pce, names, ov, count, nmatches == 0);
			nmatches++;
			if (!global) break;

			start_offset2 = ov[1];
			/* Empty match: retry at the same place, non-empty and anchored;
			 * if that fails, advance one character. */
			if (start_offset2 == ov[0]) {
				count = pcre2_match(pce->re, subject, subject_len, start_offset2,
				                    PCRE2_NO_UTF_CHECK | PCRE2_NOTEMPTY_ATSTART | PCRE2_ANCHORED, md, mctx);
				if (count >= 0) {
					goto matched;
				} else if (count == PCRE2_ERROR_NOMATCH) {
					if (start_offset2 < subject_len)
						start_offset2 += unit_length(pce, subj->p + start_offset2);
					else
						break;
				} else {
					goto error;
				}
			}
		} else if (count == PCRE2_ERROR_NOMATCH) {
			break;
		} else {
		error:
			fail(o, exec_error_msg(count), -1);
			goto done;
		}
		count = pcre2_match(pce->re, subject, subject_len, start_offset2, PCRE2_NO_UTF_CHECK, md, mctx);
	}
	jb_str(b, "]");
done:
	free(names);
}

/* preg_get_backref: \n, $n, ${n} with one or two digits. str is
 * NUL-terminated. */
static int preg_get_backref(const char **str, int *backref) {
	char in_brace = 0;
	const char *walk = *str;

	if (walk[1] == 0) return 0;
	if (*walk == '$' && walk[1] == '{') {
		in_brace = 1;
		walk++;
	}
	walk++;
	if (*walk >= '0' && *walk <= '9') {
		*backref = *walk - '0';
		walk++;
	} else {
		return 0;
	}
	if (*walk && *walk >= '0' && *walk <= '9') {
		*backref = *backref * 10 + *walk - '0';
		walk++;
	}
	if (in_brace) {
		if (*walk != '}') return 0;
		walk++;
	}
	*str = walk;
	return 1;
}

/* Appends the expansion of the replacement for the current match. */
static void expand(jbuf *out, const jstr *repl, const char *subject, const PCRE2_SIZE *ov, int count) {
	const char *walk = repl->p, *replace_end = repl->p + repl->n;
	char walk_last = 0;
	int backref;
	while (walk < replace_end) {
		if ('\\' == *walk || '$' == *walk) {
			if (walk_last == '\\') {
				out->p[out->n - 1] = *walk++; /* "\\" -> "\", "\$" -> "$" */
				walk_last = 0;
				continue;
			}
			if (preg_get_backref(&walk, &backref)) {
				if (backref < count && ov[backref << 1] != PCRE2_UNSET)
					jb_bytes(out, subject + ov[backref << 1], ov[(backref << 1) + 1] - ov[backref << 1]);
				continue;
			}
		}
		jb_bytes(out, walk, 1);
		walk++;
		walk_last = walk[-1];
	}
}

/* php_pcre_replace_impl. */
static void do_replace(const pce_t *pce, const jstr *subj, const jstr *repl, size_t limit, pcre2_match_data *md, outcome *o) {
	PCRE2_SPTR subject = (PCRE2_SPTR)subj->p;
	size_t subject_len = subj->n;
	PCRE2_SIZE *ov = pcre2_get_ovector_pointer(md);
	jbuf out = {0};
	PCRE2_SIZE start_offset = 0, last_end_offset = 0, piece;
	uint32_t options = (pce->compile_options & PCRE2_UTF) ? 0 : PCRE2_NO_UTF_CHECK;

	jb_bytes(&out, "", 0);
	int count = pcre2_match(pce->re, subject, subject_len, start_offset, options, md, mctx);
	for (;;) {
		piece = last_end_offset;
		if (count >= 0 && limit > 0) {
		matched:
			if (ov[1] < ov[0]) {
				fail(o, "Internal error", -1);
				goto done;
			}
			jb_bytes(&out, subj->p + piece, ov[0] - piece); /* part before the match */
			expand(&out, repl, subj->p, ov, count);
			limit--;
			start_offset = last_end_offset = ov[1];
			if (start_offset == ov[0]) {
				count = pcre2_match(pce->re, subject, subject_len, start_offset,
				                    PCRE2_NO_UTF_CHECK | PCRE2_NOTEMPTY_ATSTART | PCRE2_ANCHORED, md, mctx);
				piece = start_offset;
				if (count >= 0 && limit > 0) {
					goto matched;
				} else if (count == PCRE2_ERROR_NOMATCH || limit == 0) {
					if (start_offset < subject_len)
						start_offset += unit_length(pce, subj->p + piece);
					else
						goto not_matched;
				} else {
					goto error;
				}
			}
		} else if (count == PCRE2_ERROR_NOMATCH || limit == 0) {
		not_matched:
			jb_bytes(&out, subj->p + last_end_offset, subject_len - last_end_offset);
			jb_str(&o->body, ",\"output\":");
			jb_quote(&o->body, out.p, out.n);
			goto done;
		} else {
		error:
			fail(o, exec_error_msg(count), -1);
			goto done;
		}
		count = pcre2_match(pce->re, subject, subject_len, start_offset, PCRE2_NO_UTF_CHECK, md, mctx);
	}
done:
	free(out.p);
}

/* php_pcre_split_impl with limit -1 and no flags. */
static void do_split(const pce_t *pce, const jstr *subj, pcre2_match_data *md, outcome *o) {
	PCRE2_SPTR subject = (PCRE2_SPTR)subj->p;
	size_t subject_len = subj->n;
	PCRE2_SIZE *ov = pcre2_get_ovector_pointer(md);
	PCRE2_SIZE start_offset = 0, last_match_offset = 0;
	uint32_t options = (pce->compile_options & PCRE2_UTF) ? 0 : PCRE2_NO_UTF_CHECK;
	jbuf parts = {0};

	int count = pcre2_match(pce->re, subject, subject_len, start_offset, options, md, mctx);
	for (;;) {
		if (count >= 0) {
		matched:
			if (ov[1] < ov[0]) {
				fail(o, "Internal error", -1);
				goto done;
			}
			jb_str(&parts, ",");
			jb_quote(&parts, subj->p + last_match_offset, ov[0] - last_match_offset);
			start_offset = last_match_offset = ov[1];
			if (start_offset == ov[0]) {
				count = pcre2_match(pce->re, subject, subject_len, start_offset,
				                    PCRE2_NO_UTF_CHECK | PCRE2_NOTEMPTY_ATSTART | PCRE2_ANCHORED, md, mctx);
				if (count >= 0) {
					goto matched;
				} else if (count == PCRE2_ERROR_NOMATCH) {
					if (start_offset < subject_len)
						start_offset += unit_length(pce, subj->p + start_offset);
					else
						break;
				} else {
					goto error;
				}
			}
		} else if (count == PCRE2_ERROR_NOMATCH) {
			break;
		} else {
		error:
			fail(o, exec_error_msg(count), -1);
			goto done;
		}
		count = pcre2_match(pce->re, subject, subject_len, start_offset, PCRE2_NO_UTF_CHECK, md, mctx);
	}
	jb_str(&parts, ",");
	jb_quote(&parts, subj->p + last_match_offset, subject_len - last_match_offset);
	jb_str(&o->body, ",\"parts\":[");
	jb_str(&o->body, parts.p + 1); /* drop the leading ',' */
	jb_str(&o->body, "]");
done:
	free(parts.p);
}

static void evaluate(const jreq *r, outcome *o) {
	pce_t pce = {0};
	if (compile(&r->pattern, &r->flags, &pce, o)) return;
	pcre2_match_data *md = pcre2_match_data_create_from_pattern(pce.re, NULL);
	if (!md) {
		fail(o, "Internal error", -1);
	} else {
		int global = memchr(r->flags.p, 'g', r->flags.n) != NULL;
		if (strcmp(r->mode.p, "replace") == 0)
			do_replace(&pce, &r->text, &r->replacement, global ? SIZE_MAX : 1, md, o);
		else if (strcmp(r->mode.p, "split") == 0)
			do_split(&pce, &r->text, md, o);
		else
			do_match(&pce, &r->text, global, md, o);
		pcre2_match_data_free(md);
	}
	pcre2_code_free(pce.re);
}

static void respond(const jreq *r) {
	jbuf b = {0};
	jb_str(&b, "{\"id\":");
	jb_long(&b, r->id);
	if (strcmp(r->op.p, "ping") == 0) {
		char version[64];
		pcre2_config(PCRE2_CONFIG_VERSION, version);
		jb_str(&b, ",\"ok\":true,\"version\":");
		jb_quote(&b, version, strlen(version));
	} else if (strcmp(r->op.p, "eval") == 0) {
		outcome o = {0};
		evaluate(r, &o);
		if (o.err[0]) {
			jb_str(&b, ",\"ok\":false,\"error\":{\"message\":");
			jb_quote(&b, o.err, strlen(o.err));
			jb_str(&b, ",\"offset\":");
			jb_long(&b, o.err_offset);
			jb_str(&b, "}");
		} else {
			jb_str(&b, ",\"ok\":true");
			if (o.body.n) jb_bytes(&b, o.body.p, o.body.n);
		}
		free(o.body.p);
	} else {
		jb_str(&b, ",\"ok\":false,\"error\":{\"message\":\"unknown op\",\"offset\":-1}");
	}
	jb_str(&b, "}\n");
	fwrite(b.p, 1, b.n, stdout);
	fflush(stdout);
	free(b.p);
}

int main(void) {
	cctx = pcre2_compile_context_create(NULL);
	mctx = pcre2_match_context_create(NULL);
	if (!cctx || !mctx) {
		fputs("pcre shim: cannot create PCRE2 contexts\n", stderr);
		return 70;
	}
	pcre2_set_match_limit(mctx, PHP_BACKTRACK_LIMIT);
	pcre2_set_depth_limit(mctx, PHP_RECURSION_LIMIT);

	char *line = NULL;
	size_t cap = 0;
	ssize_t len;
	while ((len = getline(&line, &cap, stdin)) > 0) {
		jreq r;
		if (jreq_parse(line, (size_t)len, &r)) {
			fputs("pcre shim: malformed request\n", stderr);
			return 2;
		}
		respond(&r);
		jreq_free(&r);
	}
	free(line);
	return 0;
}
