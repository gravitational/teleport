#include "context_internal.h"
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>

#define COMP_USER  0
#define COMP_ROLE  1
#define COMP_TYPE  2
#define COMP_RANGE 3

typedef struct {
	char *current_str;	/* This is made up-to-date only when needed */
	char *(component[4]);
} context_private_t;

/*
 * Allocate a new context, initialized from str.  There must be 3 or
 * 4 colon-separated components and no whitespace in any component other
 * than the MLS component.
 */
context_t context_new(const char *str)
{
	int i, count;
	errno = 0;
	context_private_t *n =
	    (context_private_t *) malloc(sizeof(context_private_t));
	context_t result = (context_t) malloc(sizeof(context_s_t));
	const char *p, *tok;

	if (result)
		result->ptr = n;
	else
		free(n);
	if (n == 0 || result == 0) {
		goto err;
	}
	n->current_str = n->component[0] = n->component[1] = n->component[2] =
	    n->component[3] = 0;
	for (count = 0, p = str; *p; p++) {
		switch (*p) {
		case ':':
			count++;
			break;
		case '\n':
		case '\t':
		case '\r':
			goto err;	/* sanity check */
		case ' ':
			if (count < 3)
				goto err;	/* sanity check */
		}
	}
	/*
	 * Could be anywhere from 2 - 5
	 * e.g user:role:type to user:role:type:sens1:cata-sens2:catb
	 */
	if (count < 2 || count > 5) {	/* might not have a range */
		goto err;
	}

	n->component[3] = 0;
	for (i = 0, tok = str; *tok; i++) {
		if (i < 3)
			for (p = tok; *p && *p != ':'; p++) {	/* empty */
		} else {
			/* MLS range is one component */
			for (p = tok; *p; p++) {	/* empty */
			}
		}
		n->component[i] = strndup(tok, p - tok);
		if (n->component[i] == 0)
			goto err;
		tok = *p ? p + 1 : p;
	}
	return result;
      err:
	if (errno == 0) errno = EINVAL;
	context_free(result);
	return 0;
}


static void conditional_free(char **v)
{
	if (*v) {
		free(*v);
	}
	*v = 0;
}

/*
 * free all storage used by a context.  Safe to call with
 * null pointer. 
 */
void context_free(context_t context)
{
	context_private_t *n;
	int i;
	if (context) {
		n = context->ptr;
		if (n) {
			conditional_free(&n->current_str);
			for (i = 0; i < 4; i++) {
				conditional_free(&n->component[i]);
			}
			free(n);
		}
		free(context);
	}
}


/*
 * Return a pointer to the string value of the context.
 */
const char *context_str(context_t context)
{
	context_private_t *n = context->ptr;
	int i;
	size_t total = 0;
	conditional_free(&n->current_str);
	for (i = 0; i < 4; i++) {
		if (n->component[i]) {
			total += strlen(n->component[i]) + 1;
		}
	}
	n->current_str = malloc(total);
	if (n->current_str != 0) {
		char *cp = n->current_str;

		cp = stpcpy(cp, n->component[0]);
		for (i = 1; i < 4; i++) {
			if (n->component[i]) {
				*cp++ = ':';
				cp = stpcpy(cp, n->component[i]);
			}
		}
	}
	return n->current_str;
}


/*
 * Return a new string value of the context.
 */
char *context_to_str(context_t context)
{
	const context_private_t *n = context->ptr;
	char *buf;
	size_t total = 0;

	for (int i = 0; i < 4; i++) {
		if (n->component[i]) {
			total += strlen(n->component[i]) + 1;
		}
	}
	buf = malloc(total);
	if (buf != NULL) {
		char *cp = buf;

		cp = stpcpy(cp, n->component[0]);
		for (int i = 1; i < 4; i++) {
			if (n->component[i]) {
				*cp++ = ':';
				cp = stpcpy(cp, n->component[i]);
			}
		}
	}
	return buf;
}


/* Returns nonzero iff failed */
static int set_comp(context_private_t * n, int idx, const char *str)
{
	char *t = NULL;
	const char *p;
	if (str) {
		for (p = str; *p; p++) {
			if (*p == '\t' || *p == '\n' || *p == '\r' ||
			    ((*p == ':' || *p == ' ') && idx != COMP_RANGE)) {
				errno = EINVAL;
				return -1;
			}
		}

		t = strdup(str);
		if (!t) {
			return -1;
		}
	}
	conditional_free(&n->component[idx]);
	n->component[idx] = t;
	return 0;
}

#define def_get(name,tag) \
const char * context_ ## name ## _get(context_t context) \
{ \
        context_private_t *n = context->ptr; \
        return n->component[tag]; \
}

def_get(type, COMP_TYPE)
    def_get(user, COMP_USER)
    def_get(range, COMP_RANGE)
    def_get(role, COMP_ROLE)
#define def_set(name,tag) \
int context_ ## name ## _set(context_t context, const char* str) \
{ \
        return set_comp(context->ptr,tag,str);\
}
    def_set(type, COMP_TYPE)
    def_set(role, COMP_ROLE)
    def_set(user, COMP_USER)
    def_set(range, COMP_RANGE)
