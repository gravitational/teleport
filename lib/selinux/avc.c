/*
 * Implementation of the userspace access vector cache (AVC).
 *
 * Author : Eamon Walsh <ewalsh@epoch.ncsc.mil>
 *
 * Derived from the kernel AVC implementation by
 * Stephen Smalley <stephen.smalley.work@gmail.com> and
 * James Morris <jmorris@redhat.com>.
 */
#include <selinux/avc.h>
#include "selinux_internal.h"
#include <assert.h>
#include "avc_sidtab.h"
#include "avc_internal.h"

#define AVC_CACHE_SLOTS		512
#define AVC_CACHE_MAXNODES	410

struct avc_entry {
	security_id_t ssid;
	security_id_t tsid;
	security_class_t tclass;
	struct av_decision avd;
	security_id_t	create_sid;
	int used;		/* used recently */
};

struct avc_node {
	struct avc_entry ae;
	struct avc_node *next;
};

struct avc_cache {
	struct avc_node *slots[AVC_CACHE_SLOTS];
	uint32_t lru_hint;	/* LRU hint for reclaim scan */
	uint32_t active_nodes;
	uint32_t latest_notif;	/* latest revocation notification */
};

struct avc_callback_node {
	int (*callback) (uint32_t event, security_id_t ssid,
			 security_id_t tsid,
			 security_class_t tclass, access_vector_t perms,
			 access_vector_t * out_retained);
	uint32_t events;
	security_id_t ssid;
	security_id_t tsid;
	security_class_t tclass;
	access_vector_t perms;
	struct avc_callback_node *next;
};

static void *avc_lock = NULL;
static void *avc_log_lock = NULL;
static struct avc_node *avc_node_freelist = NULL;
static struct avc_cache avc_cache;
static char *avc_audit_buf = NULL;
static struct avc_cache_stats cache_stats;
static struct avc_callback_node *avc_callbacks = NULL;
static struct sidtab avc_sidtab;

static inline int avc_hash(security_id_t ssid,
			   security_id_t tsid, security_class_t tclass)
{
	return ((uintptr_t) ssid ^ ((uintptr_t) tsid << 2) ^ tclass)
	    & (AVC_CACHE_SLOTS - 1);
}

int avc_context_to_sid_raw(const char * ctx, security_id_t * sid)
{
	int rc;
	/* avc_init needs to be called before this function */
	assert(avc_running);

	avc_get_lock(avc_lock);
	rc = sidtab_context_to_sid(&avc_sidtab, ctx, sid);
	avc_release_lock(avc_lock);
	return rc;
}

int avc_context_to_sid(const char * ctx, security_id_t * sid)
{
	int ret;
	char * rctx;

	if (selinux_trans_to_raw_context(ctx, &rctx))
		return -1;

	ret = avc_context_to_sid_raw(rctx, sid);

	freecon(rctx);

	return ret;
}

int avc_sid_to_context_raw(security_id_t sid, char ** ctx)
{
	int rc;
	*ctx = NULL;
	avc_get_lock(avc_lock);
	*ctx = strdup(sid->ctx);	/* caller must free via freecon */
	rc = *ctx ? 0 : -1;
	avc_release_lock(avc_lock);
	return rc;
}

int avc_sid_to_context(security_id_t sid, char ** ctx)
{
	int ret;
	char * rctx;

	ret = avc_sid_to_context_raw(sid, &rctx);

	if (ret == 0) {
		ret = selinux_raw_to_trans_context(rctx, ctx);
		freecon(rctx);
	}

	return ret;
}

int sidget(security_id_t sid __attribute__((unused)))
{
	return 1;
}

int sidput(security_id_t sid __attribute__((unused)))
{
	return 1;
}

int avc_get_initial_sid(const char * name, security_id_t * sid)
{
	int rc;
	char * con;

	rc = security_get_initial_context_raw(name, &con);
	if (rc < 0)
		return rc;
	rc = avc_context_to_sid_raw(con, sid);

	freecon(con);

	return rc;
}

static int avc_init_internal(const char *prefix,
	     const struct avc_memory_callback *mem_cb,
	     const struct avc_log_callback *log_cb,
	     const struct avc_thread_callback *thread_cb,
	     const struct avc_lock_callback *lock_cb)
{
	struct avc_node *new;
	int i, rc = 0;

	if (avc_running)
		return 0;

	if (prefix)
		strncpy(avc_prefix, prefix, AVC_PREFIX_SIZE - 1);

	set_callbacks(mem_cb, log_cb, thread_cb, lock_cb);

	avc_lock = avc_alloc_lock();
	avc_log_lock = avc_alloc_lock();

	memset(&cache_stats, 0, sizeof(cache_stats));

	for (i = 0; i < AVC_CACHE_SLOTS; i++)
		avc_cache.slots[i] = 0;
	avc_cache.lru_hint = 0;
	avc_cache.active_nodes = 0;
	avc_cache.latest_notif = 0;

	rc = sidtab_init(&avc_sidtab);
	if (rc) {
		avc_log(SELINUX_ERROR,
			"%s:  unable to initialize SID table\n",
			avc_prefix);
		goto out;
	}

	avc_audit_buf = (char *)avc_malloc(AVC_AUDIT_BUFSIZE);
	if (!avc_audit_buf) {
		avc_log(SELINUX_ERROR,
			"%s:  unable to allocate audit buffer\n",
			avc_prefix);
		rc = -1;
		goto out;
	}

	for (i = 0; i < AVC_CACHE_MAXNODES; i++) {
		new = avc_malloc(sizeof(*new));
		if (!new) {
			avc_log(SELINUX_WARNING,
				"%s:  warning: only got %d av entries\n",
				avc_prefix, i);
			break;
		}
		memset(new, 0, sizeof(*new));
		new->next = avc_node_freelist;
		avc_node_freelist = new;
	}

	if (!avc_setenforce) {
		rc = security_getenforce();
		if (rc < 0) {
			avc_log(SELINUX_ERROR,
				"%s:  could not determine enforcing mode: %m\n",
				avc_prefix);
			goto out;
		}
		avc_enforcing = rc;
	}

	rc = selinux_status_open(0);
	if (rc < 0) {
		avc_log(SELINUX_ERROR,
			"%s: could not open selinux status page: %d (%m)\n",
			avc_prefix, errno);
		goto out;
	}
	avc_running = 1;
      out:
	return rc;
}

int avc_open(const struct selinux_opt *opts, unsigned nopts)
{
	avc_setenforce = 0;

	while (nopts) {
		nopts--;
		switch(opts[nopts].type) {
		case AVC_OPT_SETENFORCE:
			avc_setenforce = 1;
			avc_enforcing = !!opts[nopts].value;
			break;
		}
	}

	return avc_init_internal("avc", NULL, NULL, NULL, NULL);
}

int avc_init(const char *prefix,
	     const struct avc_memory_callback *mem_cb,
	     const struct avc_log_callback *log_cb,
	     const struct avc_thread_callback *thread_cb,
	     const struct avc_lock_callback *lock_cb)
{
	return avc_init_internal(prefix, mem_cb, log_cb, thread_cb, lock_cb);
}

void avc_cache_stats(struct avc_cache_stats *p)
{
	memcpy(p, &cache_stats, sizeof(cache_stats));
}

void avc_sid_stats(void)
{
	/* avc_init needs to be called before this function */
	assert(avc_running);
	avc_get_lock(avc_log_lock);
	avc_get_lock(avc_lock);
	sidtab_sid_stats(&avc_sidtab, avc_audit_buf, AVC_AUDIT_BUFSIZE);
	avc_release_lock(avc_lock);
	avc_log(SELINUX_INFO, "%s", avc_audit_buf);
	avc_release_lock(avc_log_lock);
}

void avc_av_stats(void)
{
	int i, chain_len, max_chain_len, slots_used;
	struct avc_node *node;

	avc_get_lock(avc_lock);

	slots_used = 0;
	max_chain_len = 0;
	for (i = 0; i < AVC_CACHE_SLOTS; i++) {
		node = avc_cache.slots[i];
		if (node) {
			slots_used++;
			chain_len = 0;
			while (node) {
				chain_len++;
				node = node->next;
			}
			if (chain_len > max_chain_len)
				max_chain_len = chain_len;
		}
	}

	avc_release_lock(avc_lock);

	avc_log(SELINUX_INFO, "%s:  %u AV entries and %d/%d buckets used, "
		"longest chain length %d\n", avc_prefix,
		avc_cache.active_nodes,
		slots_used, AVC_CACHE_SLOTS, max_chain_len);
}


static inline struct avc_node *avc_reclaim_node(void)
{
	struct avc_node *prev, *cur;
	int try;
	uint32_t hvalue;

	hvalue = avc_cache.lru_hint;
	for (try = 0; try < 2; try++) {
		do {
			prev = NULL;
			cur = avc_cache.slots[hvalue];
			while (cur) {
				if (!cur->ae.used)
					goto found;

				cur->ae.used = 0;

				prev = cur;
				cur = cur->next;
			}
			hvalue = (hvalue + 1) & (AVC_CACHE_SLOTS - 1);
		} while (hvalue != avc_cache.lru_hint);
	}

	errno = ENOMEM;		/* this was a panic in the kernel... */
	return NULL;

      found:
	avc_cache.lru_hint = hvalue;

	if (prev == NULL)
		avc_cache.slots[hvalue] = cur->next;
	else
		prev->next = cur->next;

	return cur;
}

static inline void avc_clear_avc_entry(struct avc_entry *ae)
{
	memset(ae, 0, sizeof(*ae));
}

static inline struct avc_node *avc_claim_node(security_id_t ssid,
					      security_id_t tsid,
					      security_class_t tclass)
{
	struct avc_node *new;
	int hvalue;

	if (!avc_node_freelist)
		avc_cleanup();

	if (avc_node_freelist) {
		new = avc_node_freelist;
		avc_node_freelist = avc_node_freelist->next;
		avc_cache.active_nodes++;
	} else {
		new = avc_reclaim_node();
		if (!new)
			goto out;
	}

	hvalue = avc_hash(ssid, tsid, tclass);
	avc_clear_avc_entry(&new->ae);
	new->ae.used = 1;
	new->ae.ssid = ssid;
	new->ae.tsid = tsid;
	new->ae.tclass = tclass;
	new->next = avc_cache.slots[hvalue];
	avc_cache.slots[hvalue] = new;

      out:
	return new;
}

static inline struct avc_node *avc_search_node(security_id_t ssid,
					       security_id_t tsid,
					       security_class_t tclass,
					       int *probes)
{
	struct avc_node *cur;
	int hvalue;
	int tprobes = 1;

	hvalue = avc_hash(ssid, tsid, tclass);
	cur = avc_cache.slots[hvalue];
	while (cur != NULL &&
	       (ssid != cur->ae.ssid ||
		tclass != cur->ae.tclass || tsid != cur->ae.tsid)) {
		tprobes++;
		cur = cur->next;
	}

	if (cur == NULL) {
		/* cache miss */
		goto out;
	}

	/* cache hit */
	if (probes)
		*probes = tprobes;

	cur->ae.used = 1;

      out:
	return cur;
}

/**
 * avc_lookup - Look up an AVC entry.
 * @ssid: source security identifier
 * @tsid: target security identifier
 * @tclass: target security class
 * @requested: requested permissions, interpreted based on @tclass
 * @aeref:  AVC entry reference
 *
 * Look up an AVC entry that is valid for the
 * @requested permissions between the SID pair
 * (@ssid, @tsid), interpreting the permissions
 * based on @tclass.  If a valid AVC entry exists,
 * then this function updates @aeref to refer to the
 * entry and returns %0.  Otherwise, -1 is returned.
 */
static int avc_lookup(security_id_t ssid, security_id_t tsid,
		      security_class_t tclass,
		      access_vector_t requested, struct avc_entry_ref *aeref)
{
	struct avc_node *node;
	int probes, rc = 0;

	avc_cache_stats_incr(cav_lookups);
	node = avc_search_node(ssid, tsid, tclass, &probes);

	if (node && ((node->ae.avd.decided & requested) == requested)) {
		avc_cache_stats_incr(cav_hits);
		avc_cache_stats_add(cav_probes, probes);
		aeref->ae = &node->ae;
		goto out;
	}

	avc_cache_stats_incr(cav_misses);
	rc = -1;
      out:
	return rc;
}

/**
 * avc_insert - Insert an AVC entry.
 * @ssid: source security identifier
 * @tsid: target security identifier
 * @tclass: target security class
 * @ae: AVC entry
 * @aeref:  AVC entry reference
 *
 * Insert an AVC entry for the SID pair
 * (@ssid, @tsid) and class @tclass.
 * The access vectors and the sequence number are
 * normally provided by the security server in
 * response to a security_compute_av() call.  If the
 * sequence number @ae->avd.seqno is not less than the latest
 * revocation notification, then the function copies
 * the access vectors into a cache entry, updates
 * @aeref to refer to the entry, and returns %0.
 * Otherwise, this function returns -%1 with @errno set to %EAGAIN.
 */
static int avc_insert(security_id_t ssid, security_id_t tsid,
		      security_class_t tclass,
		      struct avc_entry *ae, struct avc_entry_ref *aeref)
{
	struct avc_node *node;
	int rc = 0;

	if (ae->avd.seqno < avc_cache.latest_notif) {
		avc_log(SELINUX_WARNING,
			"%s:  seqno %u < latest_notif %u\n", avc_prefix,
			ae->avd.seqno, avc_cache.latest_notif);
		errno = EAGAIN;
		rc = -1;
		goto out;
	}

	node = avc_claim_node(ssid, tsid, tclass);
	if (!node) {
		rc = -1;
		goto out;
	}

	memcpy(&node->ae.avd, &ae->avd, sizeof(ae->avd));
	aeref->ae = &node->ae;
      out:
	return rc;
}

void avc_cleanup(void)
{
}


int avc_reset(void)
{
	struct avc_callback_node *c;
	int i, ret, rc = 0, errsave = 0;
	struct avc_node *node, *tmp;
	errno = 0;

	if (!avc_running)
		return 0;

	avc_get_lock(avc_lock);

	for (i = 0; i < AVC_CACHE_SLOTS; i++) {
		node = avc_cache.slots[i];
		while (node) {
			tmp = node;
			node = node->next;
			avc_clear_avc_entry(&tmp->ae);
			tmp->next = avc_node_freelist;
			avc_node_freelist = tmp;
			avc_cache.active_nodes--;
		}
		avc_cache.slots[i] = 0;
	}
	avc_cache.lru_hint = 0;

	avc_release_lock(avc_lock);

	memset(&cache_stats, 0, sizeof(cache_stats));

	for (c = avc_callbacks; c; c = c->next) {
		if (c->events & AVC_CALLBACK_RESET) {
			ret = c->callback(AVC_CALLBACK_RESET, 0, 0, 0, 0, 0);
			if (ret && !rc) {
				rc = ret;
				errsave = errno;
			}
		}
	}
	errno = errsave;
	return rc;
}


void avc_destroy(void)
{
	struct avc_callback_node *c;
	struct avc_node *node, *tmp;
	int i;
	/* avc_init needs to be called before this function */
	assert(avc_running);

	avc_get_lock(avc_lock);

	selinux_status_close();

	for (i = 0; i < AVC_CACHE_SLOTS; i++) {
		node = avc_cache.slots[i];
		while (node) {
			tmp = node;
			node = node->next;
			avc_free(tmp);
		}
	}
	while (avc_node_freelist) {
		tmp = avc_node_freelist;
		avc_node_freelist = tmp->next;
		avc_free(tmp);
	}
	avc_release_lock(avc_lock);

	while (avc_callbacks) {
		c = avc_callbacks;
		avc_callbacks = c->next;
		avc_free(c);
	}
	sidtab_destroy(&avc_sidtab);
	avc_free_lock(avc_lock);
	avc_free_lock(avc_log_lock);
	avc_free(avc_audit_buf);
	avc_running = 0;
}

/* ratelimit stuff put aside for now --EFW */
#if 0
/*
 * Copied from net/core/utils.c:net_ratelimit and modified for
 * use by the AVC audit facility.
 */
#define AVC_MSG_COST	5*HZ
#define AVC_MSG_BURST	10*5*HZ

/*
 * This enforces a rate limit: not more than one kernel message
 * every 5secs to make a denial-of-service attack impossible.
 */
static int avc_ratelimit(void)
{
	static unsigned long toks = 10 * 5 * HZ;
	static unsigned long last_msg;
	static int missed, rc = 0;
	unsigned long now = jiffies;
	void *ratelimit_lock = avc_alloc_lock();

	avc_get_lock(ratelimit_lock);
	toks += now - last_msg;
	last_msg = now;
	if (toks > AVC_MSG_BURST)
		toks = AVC_MSG_BURST;
	if (toks >= AVC_MSG_COST) {
		int lost = missed;
		missed = 0;
		toks -= AVC_MSG_COST;
		avc_release_lock(ratelimit_lock);
		if (lost) {
			avc_log(SELINUX_WARNING,
				"%s:  %d messages suppressed.\n", avc_prefix,
				lost);
		}
		rc = 1;
		goto out;
	}
	missed++;
	avc_release_lock(ratelimit_lock);
      out:
	avc_free_lock(ratelimit_lock);
	return rc;
}

static inline int check_avc_ratelimit(void)
{
	if (avc_enforcing)
		return avc_ratelimit();
	else {
		/* If permissive, then never suppress messages. */
		return 1;
	}
}
#endif				/* ratelimit stuff */

/**
 * avc_dump_av - Display an access vector in human-readable form.
 * @tclass: target security class
 * @av: access vector
 */
static void avc_dump_av(security_class_t tclass, access_vector_t av)
{
	const char *permstr;
	access_vector_t bit = 1;

	if (av == 0) {
		log_append(avc_audit_buf, " null");
		return;
	}

	log_append(avc_audit_buf, " {");

	while (av) {
		if (av & bit) {
			permstr = security_av_perm_to_string(tclass, bit);
			if (!permstr)
				break;
			log_append(avc_audit_buf, " %s", permstr);
			av &= ~bit;
		}
		bit <<= 1;
	}

	if (av)
		log_append(avc_audit_buf, " 0x%x", av);
	log_append(avc_audit_buf, " }");
}

/**
 * avc_dump_query - Display a SID pair and a class in human-readable form.
 * @ssid: source security identifier
 * @tsid: target security identifier
 * @tclass: target security class
 */
static void avc_dump_query(security_id_t ssid, security_id_t tsid,
			   security_class_t tclass)
{
	avc_get_lock(avc_lock);

	log_append(avc_audit_buf, "scontext=%s tcontext=%s",
		   ssid->ctx, tsid->ctx);

	avc_release_lock(avc_lock);
	log_append(avc_audit_buf, " tclass=%s",
		   security_class_to_string(tclass));
}

void avc_audit(security_id_t ssid, security_id_t tsid,
	       security_class_t tclass, access_vector_t requested,
	       struct av_decision *avd, int result, void *a)
{
	access_vector_t denied, audited;

	denied = requested & ~avd->allowed;
	if (denied)
		audited = denied & avd->auditdeny;
	else if (!requested || result)
		audited = denied = requested;
	else
		audited = requested & avd->auditallow;
	if (!audited)
		return;
#if 0
	if (!check_avc_ratelimit())
		return;
#endif
	/* prevent overlapping buffer writes */
	avc_get_lock(avc_log_lock);
	snprintf(avc_audit_buf, AVC_AUDIT_BUFSIZE,
		 "%s:  %s ", avc_prefix, (denied || !requested) ? "denied" : "granted");
	avc_dump_av(tclass, audited);
	log_append(avc_audit_buf, " for ");

	/* get any extra information printed by the callback */
	avc_suppl_audit(a, tclass, avc_audit_buf + strlen(avc_audit_buf),
			AVC_AUDIT_BUFSIZE - strlen(avc_audit_buf));

	log_append(avc_audit_buf, " ");
	avc_dump_query(ssid, tsid, tclass);

	if (denied)
		log_append(avc_audit_buf, " permissive=%u", result ? 0 : 1);

	avc_log(SELINUX_AVC, "%s", avc_audit_buf);

	avc_release_lock(avc_log_lock);
}



static void avd_init(struct av_decision *avd)
{
	avd->allowed = 0;
	avd->auditallow = 0;
	avd->auditdeny = 0xffffffff;
	avd->seqno = avc_cache.latest_notif;
	avd->flags = 0;
}

int avc_has_perm_noaudit(security_id_t ssid,
			 security_id_t tsid,
			 security_class_t tclass,
			 access_vector_t requested,
			 struct avc_entry_ref *aeref, struct av_decision *avd)
{
	struct avc_entry *ae;
	int rc = 0;
	struct avc_entry entry;
	access_vector_t denied;
	struct avc_entry_ref ref;

	if (avd)
		avd_init(avd);

	if (!avc_using_threads && !avc_app_main_loop) {
		(void) selinux_status_updated();
	}

	if (!aeref) {
		avc_entry_ref_init(&ref);
		aeref = &ref;
	}

	avc_get_lock(avc_lock);
	avc_cache_stats_incr(entry_lookups);
	ae = aeref->ae;
	if (ae) {
		if (ae->ssid == ssid &&
		    ae->tsid == tsid &&
		    ae->tclass == tclass &&
		    ((ae->avd.decided & requested) == requested)) {
			avc_cache_stats_incr(entry_hits);
			ae->used = 1;
		} else {
			avc_cache_stats_incr(entry_discards);
			ae = 0;
		}
	}

	if (!ae) {
		avc_cache_stats_incr(entry_misses);
		rc = avc_lookup(ssid, tsid, tclass, requested, aeref);
		if (rc) {
			rc = security_compute_av_flags_raw(ssid->ctx, tsid->ctx,
							   tclass, requested,
							   &entry.avd);
			if (rc && errno == EINVAL && !avc_enforcing) {
				rc = errno = 0;
				goto out;
			}
			if (rc)
				goto out;
			rc = avc_insert(ssid, tsid, tclass, &entry, aeref);
			if (rc)
				goto out;
		}
		ae = aeref->ae;
	}

	if (avd)
		memcpy(avd, &ae->avd, sizeof(*avd));

	denied = requested & ~(ae->avd.allowed);

	if (!requested || denied) {
		if (!avc_enforcing ||
		    (ae->avd.flags & SELINUX_AVD_FLAGS_PERMISSIVE))
			ae->avd.allowed |= requested;
		else {
			errno = EACCES;
			rc = -1;
		}
	}

      out:
	avc_release_lock(avc_lock);
	return rc;
}


int avc_has_perm(security_id_t ssid, security_id_t tsid,
		 security_class_t tclass, access_vector_t requested,
		 struct avc_entry_ref *aeref, void *auditdata)
{
	struct av_decision avd;
	int errsave, rc;

	rc = avc_has_perm_noaudit(ssid, tsid, tclass, requested, aeref, &avd);
	errsave = errno;
	avc_audit(ssid, tsid, tclass, requested, &avd, rc, auditdata);
	errno = errsave;
	return rc;
}

int avc_compute_create(security_id_t ssid,  security_id_t tsid,
		       security_class_t tclass, security_id_t *newsid)
{
	int rc;
	struct avc_entry_ref aeref;
	struct avc_entry entry;
	char * ctx;

	*newsid = NULL;
	avc_entry_ref_init(&aeref);

	avc_get_lock(avc_lock);

	/* check for a cached entry */
	rc = avc_lookup(ssid, tsid, tclass, 0, &aeref);
	if (rc) {
		/* need to make a cache entry for this tuple */
		rc = security_compute_av_flags_raw(ssid->ctx, tsid->ctx,
						   tclass, 0, &entry.avd);
		if (rc)
			goto out;
		rc = avc_insert(ssid, tsid, tclass, &entry, &aeref);
		if (rc)
			goto out;
	}

	/* check for a saved compute_create value */
	if (!aeref.ae->create_sid) {
		/* need to query the kernel policy */
		rc = security_compute_create_raw(ssid->ctx, tsid->ctx, tclass,
						 &ctx);
		if (rc)
			goto out;
		rc = sidtab_context_to_sid(&avc_sidtab, ctx, newsid);
		freecon(ctx);
		if (rc)
			goto out;

		aeref.ae->create_sid = *newsid;
	} else {
		/* found saved value */
		*newsid = aeref.ae->create_sid;
	}

	rc = 0;
out:
	avc_release_lock(avc_lock);
	return rc;
}

int avc_compute_member(security_id_t ssid,  security_id_t tsid,
		       security_class_t tclass, security_id_t *newsid)
{
	int rc;
	char * ctx = NULL;
	*newsid = NULL;
	/* avc_init needs to be called before this function */
	assert(avc_running);
	avc_get_lock(avc_lock);

	rc = security_compute_member_raw(ssid->ctx, tsid->ctx, tclass, &ctx);
	if (rc)
		goto out;
	rc = sidtab_context_to_sid(&avc_sidtab, ctx, newsid);
	freecon(ctx);
out:
	avc_release_lock(avc_lock);
	return rc;
}

int avc_add_callback(int (*callback) (uint32_t event, security_id_t ssid,
				      security_id_t tsid,
				      security_class_t tclass,
				      access_vector_t perms,
				      access_vector_t * out_retained),
		     uint32_t events, security_id_t ssid,
		     security_id_t tsid,
		     security_class_t tclass, access_vector_t perms)
{
	struct avc_callback_node *c;
	int rc = 0;

	c = avc_malloc(sizeof(*c));
	if (!c) {
		rc = -1;
		goto out;
	}

	c->callback = callback;
	c->events = events;
	c->ssid = ssid;
	c->tsid = tsid;
	c->tclass = tclass;
	c->perms = perms;
	c->next = avc_callbacks;
	avc_callbacks = c;
      out:
	return rc;
}

static inline int avc_sidcmp(security_id_t x, security_id_t y)
{
	return (x == y || x == SECSID_WILD || y == SECSID_WILD);
}

static inline void avc_update_node(uint32_t event, struct avc_node *node,
				   access_vector_t perms)
{
	switch (event) {
	case AVC_CALLBACK_GRANT:
		node->ae.avd.allowed |= perms;
		break;
	case AVC_CALLBACK_TRY_REVOKE:
	case AVC_CALLBACK_REVOKE:
		node->ae.avd.allowed &= ~perms;
		break;
	case AVC_CALLBACK_AUDITALLOW_ENABLE:
		node->ae.avd.auditallow |= perms;
		break;
	case AVC_CALLBACK_AUDITALLOW_DISABLE:
		node->ae.avd.auditallow &= ~perms;
		break;
	case AVC_CALLBACK_AUDITDENY_ENABLE:
		node->ae.avd.auditdeny |= perms;
		break;
	case AVC_CALLBACK_AUDITDENY_DISABLE:
		node->ae.avd.auditdeny &= ~perms;
		break;
	}
}

static int avc_update_cache(uint32_t event, security_id_t ssid,
			    security_id_t tsid, security_class_t tclass,
			    access_vector_t perms)
{
	struct avc_node *node;
	int i;

	avc_get_lock(avc_lock);

	if (ssid == SECSID_WILD || tsid == SECSID_WILD) {
		/* apply to all matching nodes */
		for (i = 0; i < AVC_CACHE_SLOTS; i++) {
			for (node = avc_cache.slots[i]; node; node = node->next) {
				if (avc_sidcmp(ssid, node->ae.ssid) &&
				    avc_sidcmp(tsid, node->ae.tsid) &&
				    tclass == node->ae.tclass) {
					avc_update_node(event, node, perms);
				}
			}
		}
	} else {
		/* apply to one node */
		node = avc_search_node(ssid, tsid, tclass, 0);
		if (node) {
			avc_update_node(event, node, perms);
		}
	}

	avc_release_lock(avc_lock);

	return 0;
}

/* avc_control - update cache and call callbacks
 *
 * This should not be called directly; use the individual event
 * functions instead.
 */
static int avc_control(uint32_t event, security_id_t ssid,
		       security_id_t tsid, security_class_t tclass,
		       access_vector_t perms,
		       uint32_t seqno, access_vector_t * out_retained)
{
	struct avc_callback_node *c;
	access_vector_t tretained = 0, cretained = 0;
	int ret, rc = 0, errsave = 0;
	errno = 0;

	/*
	 * try_revoke only removes permissions from the cache
	 * state if they are not retained by the object manager.
	 * Hence, try_revoke must wait until after the callbacks have
	 * been invoked to update the cache state.
	 */
	if (event != AVC_CALLBACK_TRY_REVOKE)
		avc_update_cache(event, ssid, tsid, tclass, perms);

	for (c = avc_callbacks; c; c = c->next) {
		if ((c->events & event) &&
		    avc_sidcmp(c->ssid, ssid) &&
		    avc_sidcmp(c->tsid, tsid) &&
		    c->tclass == tclass && (c->perms & perms)) {
			cretained = 0;
			ret = c->callback(event, ssid, tsid, tclass,
					  (c->perms & perms), &cretained);
			if (ret && !rc) {
				rc = ret;
				errsave = errno;
			}
			if (!ret)
				tretained |= cretained;
		}
	}

	if (event == AVC_CALLBACK_TRY_REVOKE) {
		/* revoke any unretained permissions */
		perms &= ~tretained;
		avc_update_cache(event, ssid, tsid, tclass, perms);
		*out_retained = tretained;
	}

	avc_get_lock(avc_lock);
	if (seqno > avc_cache.latest_notif)
		avc_cache.latest_notif = seqno;
	avc_release_lock(avc_lock);

	errno = errsave;
	return rc;
}

/**
 * avc_ss_grant - Grant previously denied permissions.
 * @ssid: source security identifier or %SECSID_WILD
 * @tsid: target security identifier or %SECSID_WILD
 * @tclass: target security class
 * @perms: permissions to grant
 * @seqno: policy sequence number
 */
int avc_ss_grant(security_id_t ssid, security_id_t tsid,
		 security_class_t tclass, access_vector_t perms,
		 uint32_t seqno)
{
	return avc_control(AVC_CALLBACK_GRANT,
			   ssid, tsid, tclass, perms, seqno, 0);
}

/**
 * avc_ss_try_revoke - Try to revoke previously granted permissions.
 * @ssid: source security identifier or %SECSID_WILD
 * @tsid: target security identifier or %SECSID_WILD
 * @tclass: target security class
 * @perms: permissions to grant
 * @seqno: policy sequence number
 * @out_retained: subset of @perms that are retained
 *
 * Try to revoke previously granted permissions, but
 * only if they are not retained as migrated permissions.
 * Return the subset of permissions that are retained via @out_retained.
 */
int avc_ss_try_revoke(security_id_t ssid, security_id_t tsid,
		      security_class_t tclass,
		      access_vector_t perms, uint32_t seqno,
		      access_vector_t * out_retained)
{
	return avc_control(AVC_CALLBACK_TRY_REVOKE,
			   ssid, tsid, tclass, perms, seqno, out_retained);
}

/**
 * avc_ss_revoke - Revoke previously granted permissions.
 * @ssid: source security identifier or %SECSID_WILD
 * @tsid: target security identifier or %SECSID_WILD
 * @tclass: target security class
 * @perms: permissions to grant
 * @seqno: policy sequence number
 *
 * Revoke previously granted permissions, even if
 * they are retained as migrated permissions.
 */
int avc_ss_revoke(security_id_t ssid, security_id_t tsid,
		  security_class_t tclass, access_vector_t perms,
		  uint32_t seqno)
{
	return avc_control(AVC_CALLBACK_REVOKE,
			   ssid, tsid, tclass, perms, seqno, 0);
}

/**
 * avc_ss_reset - Flush the cache and revalidate migrated permissions.
 * @seqno: policy sequence number
 */
int avc_ss_reset(uint32_t seqno)
{
	int rc;

	rc = avc_reset();

	avc_get_lock(avc_lock);
	if (seqno > avc_cache.latest_notif)
		avc_cache.latest_notif = seqno;
	avc_release_lock(avc_lock);

	return rc;
}

/**
 * avc_ss_set_auditallow - Enable or disable auditing of granted permissions.
 * @ssid: source security identifier or %SECSID_WILD
 * @tsid: target security identifier or %SECSID_WILD
 * @tclass: target security class
 * @perms: permissions to grant
 * @seqno: policy sequence number
 * @enable: enable flag.
 */
int avc_ss_set_auditallow(security_id_t ssid, security_id_t tsid,
			  security_class_t tclass, access_vector_t perms,
			  uint32_t seqno, uint32_t enable)
{
	if (enable)
		return avc_control(AVC_CALLBACK_AUDITALLOW_ENABLE,
				   ssid, tsid, tclass, perms, seqno, 0);
	else
		return avc_control(AVC_CALLBACK_AUDITALLOW_DISABLE,
				   ssid, tsid, tclass, perms, seqno, 0);
}

/**
 * avc_ss_set_auditdeny - Enable or disable auditing of denied permissions.
 * @ssid: source security identifier or %SECSID_WILD
 * @tsid: target security identifier or %SECSID_WILD
 * @tclass: target security class
 * @perms: permissions to grant
 * @seqno: policy sequence number
 * @enable: enable flag.
 */
int avc_ss_set_auditdeny(security_id_t ssid, security_id_t tsid,
			 security_class_t tclass, access_vector_t perms,
			 uint32_t seqno, uint32_t enable)
{
	if (enable)
		return avc_control(AVC_CALLBACK_AUDITDENY_ENABLE,
				   ssid, tsid, tclass, perms, seqno, 0);
	else
		return avc_control(AVC_CALLBACK_AUDITDENY_DISABLE,
				   ssid, tsid, tclass, perms, seqno, 0);
}
