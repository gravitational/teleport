#include <unistd.h>
#include <fcntl.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <stdio_ext.h>
#include <ctype.h>
#include <errno.h>
#include <limits.h>

#include <selinux/selinux.h>
#include <selinux/context.h>

#include "selinux_internal.h"
#include "callbacks.h"

/* Process line from seusers.conf and split into its fields.
   Returns 0 on success, -1 on comments, and -2 on error. */
static int process_seusers(const char *buffer,
			   char **luserp,
			   char **seuserp, char **levelp, int mls_enabled)
{
	char *newbuf = strdup(buffer);
	char *luser = NULL, *seuser = NULL, *level = NULL;
	char *start, *end;
	int mls_found = 1;

	if (!newbuf)
		goto err;

	start = newbuf;
	while (isspace((unsigned char)*start))
		start++;
	if (*start == '#' || *start == 0) {
		free(newbuf);
		return -1;	/* Comment or empty line, skip over */
	}
	end = strchr(start, ':');
	if (!end)
		goto err;
	*end = 0;

	luser = strdup(start);
	if (!luser)
		goto err;

	start = end + 1;
	end = strchr(start, ':');
	if (!end) {
		mls_found = 0;

		end = start;
		while (*end && !isspace((unsigned char)*end))
			end++;
	}
	*end = 0;

	seuser = strdup(start);
	if (!seuser)
		goto err;

	if (!strcmp(seuser, ""))
		goto err;

	/* Skip MLS if disabled, or missing. */
	if (!mls_enabled || !mls_found)
		goto out;

	start = ++end;
	while (*end && !isspace((unsigned char)*end))
		end++;
	*end = 0;

	level = strdup(start);
	if (!level)
		goto err;

	if (!strcmp(level, ""))
		goto err;

      out:
	free(newbuf);
	*luserp = luser;
	*seuserp = seuser;
	*levelp = level;
	return 0;
      err:
	free(newbuf);
	free(luser);
	free(seuser);
	free(level);
	return -2;		/* error */
}

int require_seusers  = 0;

#include <pwd.h>
#include <grp.h>

static gid_t get_default_gid(const char *name) {
	struct passwd pwstorage, *pwent = NULL;
	gid_t gid = (gid_t)-1;
	/* Allocate space for the getpwnam_r buffer */
	char *rbuf = NULL;
	long rbuflen = sysconf(_SC_GETPW_R_SIZE_MAX);
	if (rbuflen <= 0)
		rbuflen = 1024;

	for (;;) {
		int rc;

		rbuf = malloc(rbuflen);
		if (rbuf == NULL)
			break;

		rc = getpwnam_r(name, &pwstorage, rbuf, rbuflen, &pwent);
		if (rc == ERANGE && rbuflen < LONG_MAX / 2) {
			free(rbuf);
			rbuflen *= 2;
			continue;
		}
		if (rc == 0 && pwent)
			gid = pwent->pw_gid;

		break;
	}

	free(rbuf);
	return gid;
}

static int check_group(const char *group, const char *name, const gid_t gid) {
	int match = 0;
	int i, ng = 0;
	gid_t *groups = NULL;
	struct group gbuf, *grent = NULL;

	long rbuflen = sysconf(_SC_GETGR_R_SIZE_MAX);
	if (rbuflen <= 0)
		rbuflen = 1024;
	char *rbuf;

	while(1) {
		rbuf = malloc(rbuflen);
		if (rbuf == NULL)
			return 0;
		int retval = getgrnam_r(group, &gbuf, rbuf, 
				rbuflen, &grent);
		if (retval == ERANGE && rbuflen < LONG_MAX / 2)
		{
			free(rbuf);
			rbuflen = rbuflen * 2;
		} else if ( retval != 0 || grent == NULL )
		{
			goto done;
		} else
		{
			break;
		}
	}

	if (getgrouplist(name, gid, NULL, &ng) < 0) {
		if (ng == 0)
			goto done;
		groups = calloc(ng, sizeof(*groups));
		if (!groups)
			goto done;
		if (getgrouplist(name, gid, groups, &ng) < 0)
			goto done;
	} else {
		/* WTF?  ng was 0 and we didn't fail? Are we in 0 groups? */
		goto done;
	}

	for (i = 0; i < ng; i++) {
		if (grent->gr_gid == groups[i]) {
			match = 1;
			goto done;
		}
	}

 done:
	free(groups);
	free(rbuf);
	return match;
}

int getseuserbyname(const char *name, char **r_seuser, char **r_level)
{
	FILE *cfg = NULL;
	size_t size = 0;
	char *buffer = NULL;
	int rc;
	unsigned long lineno = 0;
	int mls_enabled = is_selinux_mls_enabled();

	char *username = NULL;
	char *seuser = NULL;
	char *level = NULL;
	char *groupseuser = NULL;
	char *grouplevel = NULL;
	char *defaultseuser = NULL;
	char *defaultlevel = NULL;

	gid_t gid = get_default_gid(name);

	cfg = fopen(selinux_usersconf_path(), "re");
	if (!cfg)
		goto nomatch;

	__fsetlocking(cfg, FSETLOCKING_BYCALLER);
	while (getline(&buffer, &size, cfg) > 0) {
		++lineno;
		rc = process_seusers(buffer, &username, &seuser, &level,
				     mls_enabled);
		if (rc == -1)
			continue;	/* comment, skip */
		if (rc == -2) {
			selinux_log(SELINUX_ERROR, "%s:  error on line %lu, skipping...\n",
						   selinux_usersconf_path(), lineno);
			continue;
		}

		if (!strcmp(username, name))
			break;

		if (username[0] == '%' && 
		    !groupseuser && 
		    check_group(&username[1], name, gid)) {
				groupseuser = seuser;
				grouplevel = level;
		} else {
			if (!defaultseuser && 
			    !strcmp(username, "__default__")) {
				defaultseuser = seuser;
				defaultlevel = level;
			} else {
				free(seuser);
				free(level);
			}
		}
		free(username);
		username = NULL;
		seuser = NULL;
	}

	free(buffer);
	fclose(cfg);

	if (seuser) {
		free(username);
		free(defaultseuser);
		free(defaultlevel);
		free(groupseuser);
		free(grouplevel);
		*r_seuser = seuser;
		*r_level = level;
		return 0;
	}

	if (groupseuser) {
		free(defaultseuser);
		free(defaultlevel);
		*r_seuser = groupseuser;
		*r_level = grouplevel;
		return 0;
	}

	if (defaultseuser) {
		*r_seuser = defaultseuser;
		*r_level = defaultlevel;
		return 0;
	}

      nomatch:
	if (require_seusers)
		return -1;

	/* Fall back to the Linux username and no level. */
	*r_seuser = strdup(name);
	if (!(*r_seuser))
		return -1;
	*r_level = NULL;
	return 0;
}

int getseuser(const char *username, const char *service, 
	      char **r_seuser, char **r_level) {
	int ret = -1;
	int len = 0;
	char *seuser = NULL;
	char *level = NULL;
	char *buffer = NULL;
	size_t size = 0;
	char *rec = NULL;
	char *path = NULL;
	FILE *fp = NULL;
	if (asprintf(&path,"%s/logins/%s", selinux_policy_root(), username) <  0)
		goto err;
	fp = fopen(path, "re");
	free(path);
	if (fp == NULL) goto err;
	__fsetlocking(fp, FSETLOCKING_BYCALLER);
	while (getline(&buffer, &size, fp) > 0) {
		if (strncmp(buffer, "*:", 2) == 0) {
			free(rec);
			rec = strdup(buffer);
			continue;
		}
		if (!service)
			continue;
		len = strlen(service);
		if ((strncmp(buffer, service, len) == 0) &&
		    (buffer[len] == ':')) {
			free(rec);
			rec = strdup(buffer);
			break;
		}
	}

	if (! rec)  goto err;
	seuser = strchr(rec, ':');
	if (! seuser) goto err;

	seuser++;
	level = strchr(seuser, ':');
	if (! level) goto err;
	*level = 0;
	level++;
	*r_seuser = strdup(seuser);
	if (! *r_seuser) goto err;

	len = strlen(level);
	if (len && level[len-1] == '\n')
		level[len-1] = 0;

	*r_level = strdup(level);
	if (! *r_level) {
		free(*r_seuser);
		goto err;
	}
	ret = 0;

	err:
	free(buffer);
	if (fp) fclose(fp);
	free(rec);

	return (ret ? getseuserbyname(username, r_seuser, r_level) : ret);
}
