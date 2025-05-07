/*
 * A security identifier table (sidtab) is a hash table
 * of security context structures indexed by SID value.
 */
#ifndef _SELINUX_AVC_SIDTAB_H_
#define _SELINUX_AVC_SIDTAB_H_

#include <selinux/selinux.h>
#include <selinux/avc.h>

struct sidtab_node {
	struct security_id sid_s;
	struct sidtab_node *next;
};

#define SIDTAB_HASH_BITS 7
#define SIDTAB_HASH_BUCKETS (1 << SIDTAB_HASH_BITS)
#define SIDTAB_HASH_MASK (SIDTAB_HASH_BUCKETS-1)
#define SIDTAB_SIZE SIDTAB_HASH_BUCKETS

struct sidtab {
	struct sidtab_node **htable;
	unsigned nel;
};

int sidtab_init(struct sidtab *s) ;

const struct security_id * sidtab_context_lookup(const struct sidtab *s, const char *ctx);
int sidtab_context_to_sid(struct sidtab *s,
			  const char * ctx, security_id_t * sid) ;

void sidtab_sid_stats(const struct sidtab *s, char *buf, size_t buflen) ;
void sidtab_destroy(struct sidtab *s) ;

#endif				/* _SELINUX_AVC_SIDTAB_H_ */
