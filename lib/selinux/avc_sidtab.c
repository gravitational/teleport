/*
 * Implementation of the userspace SID hashtable.
 *
 * Author : Eamon Walsh, <ewalsh@epoch.ncsc.mil>
 */
#include <errno.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include "selinux_internal.h"
#include <selinux/avc.h>
#include "avc_sidtab.h"
#include "avc_internal.h"

ignore_unsigned_overflow_
static inline unsigned sidtab_hash(const char * key)
{
	unsigned int hash = 5381;
	unsigned char c;

	while ((c = *(unsigned const char *)key++))
		hash = ((hash << 5) + hash) ^ c;

	return hash & (SIDTAB_SIZE - 1);
}

int sidtab_init(struct sidtab *s)
{
	int i, rc = 0;

	s->htable = (struct sidtab_node **)avc_malloc
	    (sizeof(struct sidtab_node *) * SIDTAB_SIZE);

	if (!s->htable) {
		rc = -1;
		goto out;
	}
	for (i = 0; i < SIDTAB_SIZE; i++)
		s->htable[i] = NULL;
	s->nel = 0;
      out:
	return rc;
}

static struct sidtab_node *
sidtab_insert(struct sidtab *s, const char * ctx)
{
	unsigned hvalue;
	struct sidtab_node *newnode;
	char * newctx;

	if (s->nel >= UINT_MAX - 1)
		return NULL;

	newnode = (struct sidtab_node *)avc_malloc(sizeof(*newnode));
	if (!newnode)
		return NULL;
	newctx = strdup(ctx);
	if (!newctx) {
		avc_free(newnode);
		return NULL;
	}

	hvalue = sidtab_hash(newctx);
	newnode->next = s->htable[hvalue];
	newnode->sid_s.ctx = newctx;
	newnode->sid_s.id = ++s->nel;
	s->htable[hvalue] = newnode;
	return newnode;
}

const struct security_id *
sidtab_context_lookup(const struct sidtab *s, const char *ctx)
{
	unsigned hvalue;
	const struct sidtab_node *cur;

	hvalue = sidtab_hash(ctx);

	cur = s->htable[hvalue];
	while (cur != NULL && strcmp(cur->sid_s.ctx, ctx))
		cur = cur->next;

	if (cur == NULL)
		return NULL;

	return &cur->sid_s;
}

int
sidtab_context_to_sid(struct sidtab *s,
		      const char * ctx, security_id_t * sid)
{
	struct sidtab_node *new;
	const struct security_id *lookup_sid = sidtab_context_lookup(s, ctx);

	if (lookup_sid) {
		/* Dropping const is fine since our sidtab parameter is non-const. */
		*sid = (struct security_id *)lookup_sid;
		return 0;
	}

	new = sidtab_insert(s, ctx);
	if (new == NULL) {
		*sid = NULL;
		return -1;
	}

	*sid = &new->sid_s;
	return 0;
}

void sidtab_sid_stats(const struct sidtab *s, char *buf, size_t buflen)
{
	size_t i, chain_len, slots_used, max_chain_len;
	const struct sidtab_node *cur;

	slots_used = 0;
	max_chain_len = 0;
	for (i = 0; i < SIDTAB_SIZE; i++) {
		cur = s->htable[i];
		if (cur) {
			slots_used++;
			chain_len = 0;
			while (cur) {
				chain_len++;
				cur = cur->next;
			}

			if (chain_len > max_chain_len)
				max_chain_len = chain_len;
		}
	}

	snprintf(buf, buflen,
		 "%s:  %u SID entries and %zu/%d buckets used, longest "
		 "chain length %zu\n", avc_prefix, s->nel, slots_used,
		 SIDTAB_SIZE, max_chain_len);
}

void sidtab_destroy(struct sidtab *s)
{
	int i;
	struct sidtab_node *cur, *temp;

	if (!s || !s->htable)
		return;

	for (i = 0; i < SIDTAB_SIZE; i++) {
		cur = s->htable[i];
		while (cur != NULL) {
			temp = cur;
			cur = cur->next;
			freecon(temp->sid_s.ctx);
			avc_free(temp);
		}
	}
	avc_free(s->htable);
	s->htable = NULL;
}
