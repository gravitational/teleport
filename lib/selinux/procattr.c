#include <assert.h>
#include <sys/syscall.h>
#include <unistd.h>
#include <fcntl.h>
#include <pthread.h>
#include <string.h>
#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
#include <errno.h>
#include "selinux_internal.h"
#include "policy.h"

#define UNSET (char *) -1

/* Cached values so that when a thread calls set*con() then gen*con(), the value
 * which was set is directly returned.
 */
static __thread char *prev_current = UNSET;
static __thread char *prev_exec = UNSET;
static __thread char *prev_fscreate = UNSET;
static __thread char *prev_keycreate = UNSET;
static __thread char *prev_sockcreate = UNSET;

static pthread_once_t once = PTHREAD_ONCE_INIT;
static pthread_key_t destructor_key;
static int destructor_key_initialized = 0;
static __thread char destructor_initialized;

/* Bionic and glibc >= 2.30 declare gettid() system call wrapper in unistd.h and
 * has a definition for it */
#ifdef __BIONIC__
  #define HAVE_GETTID 1
#elif !defined(__GLIBC_PREREQ)
  #define HAVE_GETTID 0
#elif !__GLIBC_PREREQ(2,30)
  #define HAVE_GETTID 0
#else
  #define HAVE_GETTID 1
#endif

static pid_t selinux_gettid(void)
{
#if HAVE_GETTID
	return gettid();
#else
	return syscall(__NR_gettid);
#endif
}

static void procattr_thread_destructor(void __attribute__((unused)) *unused)
{
	if (prev_current != UNSET)
		free(prev_current);
	if (prev_exec != UNSET)
		free(prev_exec);
	if (prev_fscreate != UNSET)
		free(prev_fscreate);
	if (prev_keycreate != UNSET)
		free(prev_keycreate);
	if (prev_sockcreate != UNSET)
		free(prev_sockcreate);
}

void __attribute__((destructor)) procattr_destructor(void);

void  __attribute__((destructor)) procattr_destructor(void)
{
	if (destructor_key_initialized)
		__selinux_key_delete(destructor_key);
}

static inline void init_thread_destructor(void)
{
	if (destructor_initialized == 0) {
		__selinux_setspecific(destructor_key, /* some valid address to please GCC */ &selinux_page_size);
		destructor_initialized = 1;
	}
}

static void init_procattr(void)
{
	if (__selinux_key_create(&destructor_key, procattr_thread_destructor) == 0) {
		destructor_key_initialized = 1;
	}
}

static int openattr(pid_t pid, const char *attr, int flags)
{
	int fd, rc;
	char path[44];  /* must hold "/proc/self/task/%d/attr/sockcreate" */
	pid_t tid;

	static_assert(sizeof(pid_t) <= sizeof(uint32_t), "content written to path might get truncated");

	if (pid > 0) {
		rc = snprintf(path, sizeof(path), "/proc/%d/attr/%s", pid, attr);
	} else if (pid == 0) {
		rc = snprintf(path, sizeof(path), "/proc/thread-self/attr/%s", attr);
		if (rc < 0 || (size_t)rc >= sizeof(path)) {
			errno = EOVERFLOW;
			return -1;
		}
		fd = open(path, flags | O_CLOEXEC);
		if (fd >= 0 || errno != ENOENT)
			return fd;
		tid = selinux_gettid();
		rc = snprintf(path, sizeof(path), "/proc/self/task/%d/attr/%s", tid, attr);
	} else {
		errno = EINVAL;
		return -1;
	}
	if (rc < 0 || (size_t)rc >= sizeof(path)) {
		errno = EOVERFLOW;
		return -1;
	}

	return open(path, flags | O_CLOEXEC);
}

static int getprocattrcon_raw(char **context, pid_t pid, const char *attr,
			      const char *prev_context)
{
	char *buf;
	size_t size;
	int fd;
	ssize_t ret;
	int errno_hold;

	__selinux_once(once, init_procattr);
	init_thread_destructor();

	if (prev_context && prev_context != UNSET) {
		*context = strdup(prev_context);
		if (!(*context)) {
			return -1;
		}
		return 0;
	}

	fd = openattr(pid, attr, O_RDONLY | O_CLOEXEC);
	if (fd < 0)
		return -1;

	size = selinux_page_size;
	buf = calloc(1, size);
	if (!buf) {
		ret = -1;
		goto out;
	}

	do {
		ret = read(fd, buf, size - 1);
	} while (ret < 0 && errno == EINTR);
	if (ret < 0)
		goto out2;

	if (ret == 0) {
		*context = NULL;
		goto out2;
	}

	*context = strdup(buf);
	if (!(*context)) {
		ret = -1;
		goto out2;
	}
	ret = 0;
      out2:
	free(buf);
      out:
	errno_hold = errno;
	close(fd);
	errno = errno_hold;
	return ret;
}

static int getprocattrcon(char **context, pid_t pid, const char *attr,
			  const char *prev_context)
{
	int ret;
	char * rcontext;

	ret = getprocattrcon_raw(&rcontext, pid, attr, prev_context);

	if (!ret) {
		ret = selinux_raw_to_trans_context(rcontext, context);
		freecon(rcontext);
	}

	return ret;
}

static int setprocattrcon_raw(const char *context, const char *attr,
			      char **prev_context)
{
	int fd;
	ssize_t ret;
	int errno_hold;
	char *context2 = NULL;

	__selinux_once(once, init_procattr);
	init_thread_destructor();

	if (!context && !*prev_context)
		return 0;
	if (context && *prev_context && *prev_context != UNSET
	    && !strcmp(context, *prev_context))
		return 0;

	fd = openattr(0, attr, O_RDWR | O_CLOEXEC);
	if (fd < 0)
		return -1;
	if (context) {
		ret = -1;
		context2 = strdup(context);
		if (!context2)
			goto out;
		do {
			ret = write(fd, context2, strlen(context2) + 1);
		} while (ret < 0 && errno == EINTR);
	} else {
		do {
			ret = write(fd, NULL, 0);	/* clear */
		} while (ret < 0 && errno == EINTR);
	}
out:
	errno_hold = errno;
	close(fd);
	errno = errno_hold;
	if (ret < 0) {
		free(context2);
		return -1;
	} else {
		if (*prev_context != UNSET)
			free(*prev_context);
		*prev_context = context2;
		return 0;
	}
}

static int setprocattrcon(const char *context, const char *attr,
			  char **prev_context)
{
	int ret;
	char * rcontext;

	if (selinux_trans_to_raw_context(context, &rcontext))
		return -1;

	ret = setprocattrcon_raw(rcontext, attr, prev_context);

	freecon(rcontext);

	return ret;
}

#define getselfattr_def(fn, attr, prev_context) \
	int get##fn##_raw(char **c) \
	{ \
		return getprocattrcon_raw(c, 0, attr, prev_context); \
	} \
	int get##fn(char **c) \
	{ \
		return getprocattrcon(c, 0, attr, prev_context); \
	}

#define setselfattr_def(fn, attr, prev_context) \
	int set##fn##_raw(const char * c) \
	{ \
		return setprocattrcon_raw(c, attr, &prev_context); \
	} \
	int set##fn(const char * c) \
	{ \
		return setprocattrcon(c, attr, &prev_context); \
	}

#define all_selfattr_def(fn, attr, prev_context) \
	getselfattr_def(fn, attr, prev_context)	 \
	setselfattr_def(fn, attr, prev_context)

all_selfattr_def(con, "current", prev_current)
    getselfattr_def(prevcon, "prev", NULL)
    all_selfattr_def(execcon, "exec", prev_exec)
    all_selfattr_def(fscreatecon, "fscreate", prev_fscreate)
    all_selfattr_def(sockcreatecon, "sockcreate", prev_sockcreate)
    all_selfattr_def(keycreatecon, "keycreate", prev_keycreate)

int getpidcon_raw(pid_t pid, char **c)
{
	if (pid <= 0) {
		errno = EINVAL;
		return -1;
	}
	return getprocattrcon_raw(c, pid, "current", NULL);
}

int getpidcon(pid_t pid, char **c)
{
	if (pid <= 0) {
		errno = EINVAL;
		return -1;
	}
	return getprocattrcon(c, pid, "current", NULL);
}

int getpidprevcon_raw(pid_t pid, char **c)
{
        if (pid <= 0) {
                errno = EINVAL;
                return -1;
        }
        return getprocattrcon_raw(c, pid, "prev", NULL);
}

int getpidprevcon(pid_t pid, char **c)
{
        if (pid <= 0) {
                errno = EINVAL;
                return -1;
        }
        return getprocattrcon(c, pid, "prev", NULL);
}
