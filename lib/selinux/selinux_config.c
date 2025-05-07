#include <stdio.h>
#include <stdio_ext.h>
#include <string.h>
#include <ctype.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <limits.h>
#include <unistd.h>
#include <pthread.h>
#include <errno.h>
#include "policy.h"
#include "selinux_internal.h"
#include "get_default_type_internal.h"

#define SELINUXDEFAULT "targeted"
#define SELINUXTYPETAG "SELINUXTYPE="
#define SELINUXTAG "SELINUX="
#define REQUIRESEUSERS "REQUIRESEUSERS="

/* Indices for file paths arrays. */
#define BINPOLICY         0
#define CONTEXTS_DIR      1
#define FILE_CONTEXTS     2
#define HOMEDIR_CONTEXTS  3
#define DEFAULT_CONTEXTS  4
#define USER_CONTEXTS     5
#define FAILSAFE_CONTEXT  6
#define DEFAULT_TYPE      7
/* BOOLEANS is deprecated */
#define BOOLEANS          8
#define MEDIA_CONTEXTS    9
#define REMOVABLE_CONTEXT 10
#define CUSTOMIZABLE_TYPES    11
/* USERS_DIR is deprecated */
#define USERS_DIR         12
#define SEUSERS           13
#define TRANSLATIONS      14
#define NETFILTER_CONTEXTS    15
#define FILE_CONTEXTS_HOMEDIR 16
#define FILE_CONTEXTS_LOCAL 17
#define SECURETTY_TYPES   18
#define X_CONTEXTS        19
#define COLORS            20
#define VIRTUAL_DOMAIN    21
#define VIRTUAL_IMAGE     22
#define FILE_CONTEXT_SUBS 23
#define SEPGSQL_CONTEXTS  24
#define FILE_CONTEXT_SUBS_DIST 25
#define LXC_CONTEXTS      26
#define BOOLEAN_SUBS      27
#define OPENSSH_CONTEXTS  28
#define SYSTEMD_CONTEXTS  29
#define SNAPPERD_CONTEXTS 30
#define OPENRC_CONTEXTS   31
#define NEL               32

/* Part of one-time lazy init */
static pthread_once_t once = PTHREAD_ONCE_INIT;
static void init_selinux_config(void);

/* New layout is relative to SELINUXDIR/policytype. */
static char *file_paths[NEL];
#define L1(l) L2(l)
#define L2(l)str##l
static const union file_path_suffixes_data {
	struct {
#define S_(n, s) char L1(__LINE__)[sizeof(s)];
#include "file_path_suffixes.h"
#undef S_
	};
	char str[0];
} file_path_suffixes_data = {
	{
#define S_(n, s) s,
#include "file_path_suffixes.h"
#undef S_
	}
};
static const uint16_t file_path_suffixes_idx[NEL] = {
#define S_(n, s) [n] = offsetof(union file_path_suffixes_data, L1(__LINE__)),
#include "file_path_suffixes.h"
#undef S_
};

#undef L1
#undef L2

int selinux_getenforcemode(int *enforce)
{
	int ret = -1;
	FILE *cfg = fopen(SELINUXCONFIG, "re");
	if (cfg) {
		char *buf;
		char *tag;
		int len = sizeof(SELINUXTAG) - 1;
		buf = malloc(selinux_page_size);
		if (!buf) {
			fclose(cfg);
			return -1;
		}
		while (fgets_unlocked(buf, selinux_page_size, cfg)) {
			if (strncmp(buf, SELINUXTAG, len))
				continue;
			tag = buf+len;
			while (isspace((unsigned char)*tag))
				tag++;
			if (!strncasecmp
			    (tag, "enforcing", sizeof("enforcing") - 1)) {
				*enforce = 1;
				ret = 0;
				break;
			} else
			    if (!strncasecmp
				(tag, "permissive",
				 sizeof("permissive") - 1)) {
				*enforce = 0;
				ret = 0;
				break;
			} else
			    if (!strncasecmp
				(tag, "disabled",
				 sizeof("disabled") - 1)) {
				*enforce = -1;
				ret = 0;
				break;
			}
		}
		fclose(cfg);
		free(buf);
	}
	return ret;
}


static char *selinux_policytype;

int selinux_getpolicytype(char **type)
{
	__selinux_once(once, init_selinux_config);
	if (!selinux_policytype)
		return -1;
	*type = strdup(selinux_policytype);
	return *type ? 0 : -1;
}


static int setpolicytype(const char *type)
{
	free(selinux_policytype);
	selinux_policytype = strdup(type);
	return selinux_policytype ? 0 : -1;
}

static char *selinux_policyroot = NULL;

static void init_selinux_config(void)
{
	int i, *intptr;
	size_t line_len;
	ssize_t len;
	char *line_buf = NULL, *buf_p, *value, *type = NULL, *end;
	FILE *fp;

	if (selinux_policyroot)
		return;

	fp = fopen(SELINUXCONFIG, "re");
	if (fp) {
		__fsetlocking(fp, FSETLOCKING_BYCALLER);
		while ((len = getline(&line_buf, &line_len, fp)) > 0) {
			if (line_buf[len - 1] == '\n')
				line_buf[len - 1] = 0;
			buf_p = line_buf;
			while (isspace((unsigned char)*buf_p))
				buf_p++;
			if (*buf_p == '#' || *buf_p == 0)
				continue;

			if (!strncasecmp(buf_p, SELINUXTYPETAG,
					 sizeof(SELINUXTYPETAG) - 1)) {
				buf_p += sizeof(SELINUXTYPETAG) - 1;
				while (isspace((unsigned char)*buf_p))
					buf_p++;
				type = strdup(buf_p);
				if (!type) {
					free(line_buf);
					fclose(fp);
					return;
				}
				end = type + strlen(type) - 1;
				while ((end > type) &&
				       (isspace((unsigned char)*end) || iscntrl((unsigned char)*end))) {
					*end = 0;
					end--;
				}
				if (setpolicytype(type) != 0) {
					free(type);
					free(line_buf);
					fclose(fp);
					return;
				}
				free(type);
				continue;
			} else if (!strncmp(buf_p, REQUIRESEUSERS,
					    sizeof(REQUIRESEUSERS) - 1)) {
				value = buf_p + sizeof(REQUIRESEUSERS) - 1;
				while (isspace((unsigned char)*value))
					value++;
				intptr = &require_seusers;
			} else {
				continue;
			}

			if (isdigit((unsigned char)*value))
				*intptr = atoi(value);
			else if (strncasecmp(value, "true", sizeof("true") - 1))
				*intptr = 1;
			else if (strncasecmp
				 (value, "false", sizeof("false") - 1))
				*intptr = 0;
		}
		free(line_buf);
		fclose(fp);
	}

	if (!selinux_policytype && setpolicytype(SELINUXDEFAULT) != 0)
		return;

	if (asprintf(&selinux_policyroot, "%s%s", SELINUXDIR, selinux_policytype) == -1)
		return;

	for (i = 0; i < NEL; i++)
		if (asprintf(&file_paths[i], "%s%s",
			     selinux_policyroot,
			     file_path_suffixes_data.str +
			     file_path_suffixes_idx[i])
		    == -1)
			return;
}

static void fini_selinux_policyroot(void) __attribute__ ((destructor));

static void fini_selinux_policyroot(void)
{
	int i;
	free(selinux_policyroot);
	selinux_policyroot = NULL;
	for (i = 0; i < NEL; i++) {
		free(file_paths[i]);
		file_paths[i] = NULL;
	}
	free(selinux_policytype);
	selinux_policytype = NULL;
}

void selinux_reset_config(void)
{
	fini_selinux_policyroot();
	init_selinux_config();
}


static const char *get_path(int idx)
{
	__selinux_once(once, init_selinux_config);
	return file_paths[idx];
}

const char *selinux_default_type_path(void)
{
	return get_path(DEFAULT_TYPE);
}


const char *selinux_policy_root(void)
{
	__selinux_once(once, init_selinux_config);
	return selinux_policyroot;
}

int selinux_set_policy_root(const char *path)
{
	int i;
	char *policy_type = strrchr(path, '/');
	if (!policy_type) {
		errno = EINVAL;
		return -1;
	}
	policy_type++;

	fini_selinux_policyroot();

	selinux_policyroot = strdup(path);
	if (! selinux_policyroot)
		return -1;

	if (setpolicytype(policy_type) != 0)
		return -1;

	for (i = 0; i < NEL; i++)
		if (asprintf(&file_paths[i], "%s%s",
			     selinux_policyroot,
			     file_path_suffixes_data.str +
			     file_path_suffixes_idx[i])
		    == -1)
			return -1;

	return 0;
}

const char *selinux_path(void)
{
	return SELINUXDIR;
}


const char *selinux_default_context_path(void)
{
	return get_path(DEFAULT_CONTEXTS);
}


const char *selinux_securetty_types_path(void)
{
	return get_path(SECURETTY_TYPES);
}


const char *selinux_failsafe_context_path(void)
{
	return get_path(FAILSAFE_CONTEXT);
}


const char *selinux_removable_context_path(void)
{
	return get_path(REMOVABLE_CONTEXT);
}


const char *selinux_binary_policy_path(void)
{
	return get_path(BINPOLICY);
}


const char *selinux_current_policy_path(void)
{
	int rc = 0;
	int vers = 0;
	static char policy_path[PATH_MAX];

	if (selinux_mnt) {
		snprintf(policy_path, sizeof(policy_path), "%s/policy", selinux_mnt);
		if (access(policy_path, F_OK) == 0 ) {
			return policy_path;
		}
	}
	vers = security_policyvers();
	do {
		/* Check prior versions to see if old policy is available */
		snprintf(policy_path, sizeof(policy_path), "%s.%d",
			 selinux_binary_policy_path(), vers);
	} while ((rc = access(policy_path, F_OK)) && --vers > 0);

	if (rc) return NULL;
	return policy_path;
}


const char *selinux_file_context_path(void)
{
	return get_path(FILE_CONTEXTS);
}


const char *selinux_homedir_context_path(void)
{
	return get_path(HOMEDIR_CONTEXTS);
}


const char *selinux_media_context_path(void)
{
	return get_path(MEDIA_CONTEXTS);
}


const char *selinux_customizable_types_path(void)
{
	return get_path(CUSTOMIZABLE_TYPES);
}


const char *selinux_contexts_path(void)
{
	return get_path(CONTEXTS_DIR);
}

const char *selinux_user_contexts_path(void)
{
	return get_path(USER_CONTEXTS);
}


/* Deprecated as local policy booleans no longer supported. */
const char *selinux_booleans_path(void)
{
	return get_path(BOOLEANS);
}


/* Deprecated as no longer supported. */
const char *selinux_users_path(void)
{
	return get_path(USERS_DIR);
}


const char *selinux_usersconf_path(void)
{
	return get_path(SEUSERS);
}


const char *selinux_translations_path(void)
{
	return get_path(TRANSLATIONS);
}


const char *selinux_colors_path(void)
{
	return get_path(COLORS);
}


const char *selinux_netfilter_context_path(void)
{
	return get_path(NETFILTER_CONTEXTS);
}


const char *selinux_file_context_homedir_path(void)
{
	return get_path(FILE_CONTEXTS_HOMEDIR);
}


const char *selinux_file_context_local_path(void)
{
	return get_path(FILE_CONTEXTS_LOCAL);
}


const char *selinux_x_context_path(void)
{
	return get_path(X_CONTEXTS);
}


const char *selinux_virtual_domain_context_path(void)
{
	return get_path(VIRTUAL_DOMAIN);
}


const char *selinux_virtual_image_context_path(void)
{
	return get_path(VIRTUAL_IMAGE);
}


const char *selinux_lxc_contexts_path(void)
{
	return get_path(LXC_CONTEXTS);
}


const char *selinux_openrc_contexts_path(void)
{
    return get_path(OPENRC_CONTEXTS);
}


const char *selinux_openssh_contexts_path(void)
{
    return get_path(OPENSSH_CONTEXTS);
}


const char *selinux_snapperd_contexts_path(void)
{
    return get_path(SNAPPERD_CONTEXTS);
}


const char *selinux_systemd_contexts_path(void)
{
	return get_path(SYSTEMD_CONTEXTS);
}


const char * selinux_booleans_subs_path(void) {
	return get_path(BOOLEAN_SUBS);
}


const char * selinux_file_context_subs_path(void) {
	return get_path(FILE_CONTEXT_SUBS);
}


const char * selinux_file_context_subs_dist_path(void) {
	return get_path(FILE_CONTEXT_SUBS_DIST);
}


const char *selinux_sepgsql_context_path(void)
{
	return get_path(SEPGSQL_CONTEXTS);
}

