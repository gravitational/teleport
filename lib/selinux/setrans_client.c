/* Author: Trusted Computer Solutions, Inc. 
 * 
 * Modified:
 * Yuichi Nakamura <ynakam@hitachisoft.jp> 
 - Stubs are used when DISABLE_SETRANS is defined, 
   it is to reduce size for such as embedded devices.
*/

#include <sys/types.h>
#include <sys/socket.h>
#include <sys/un.h>

#include <errno.h>
#include <stdlib.h>
#include <netdb.h>
#include <fcntl.h>
#include <stdio.h>
#include <string.h>
#include <ctype.h>
#include <unistd.h>
#include <sys/uio.h>
#include "selinux_internal.h"
#include "setrans_internal.h"

#ifndef DISABLE_SETRANS
static unsigned char has_setrans;

// Simple cache
static __thread char * prev_t2r_trans = NULL;
static __thread char * prev_t2r_raw = NULL;
static __thread char * prev_r2t_trans = NULL;
static __thread char * prev_r2t_raw = NULL;
static __thread char *prev_r2c_trans = NULL;
static __thread char * prev_r2c_raw = NULL;

static pthread_once_t once = PTHREAD_ONCE_INIT;
static pthread_key_t destructor_key;
static int destructor_key_initialized = 0;
static __thread char destructor_initialized;

/*
 * setransd_open
 *
 * This function opens a socket to the setransd.
 * Returns:  on success, a file descriptor ( >= 0 ) to the socket
 *           on error, a negative value
 */
static int setransd_open(void)
{
	struct sockaddr_un addr;
	int fd;
#ifdef SOCK_CLOEXEC
	fd = socket(PF_UNIX, SOCK_STREAM|SOCK_CLOEXEC, 0);
	if (fd < 0 && errno == EINVAL)
#endif
	{
		fd = socket(PF_UNIX, SOCK_STREAM, 0);
		if (fd >= 0)
			if (fcntl(fd, F_SETFD, FD_CLOEXEC)) {
				close(fd);
				return -1;
			}
	}
	if (fd < 0)
		return -1;

	memset(&addr, 0, sizeof(addr));
	addr.sun_family = AF_UNIX;

	if (strlcpy(addr.sun_path, SETRANS_UNIX_SOCKET, sizeof(addr.sun_path)) >= sizeof(addr.sun_path)) {
		close(fd);
		errno = EOVERFLOW;
		return -1;
	}

	if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
		close(fd);
		return -1;
	}

	return fd;
}

/* Returns: 0 on success, <0 on failure */
static int
send_request(int fd, uint32_t function, const char *data1, const char *data2)
{
	struct msghdr msgh;
	struct iovec iov[5];
	uint32_t data1_size;
	uint32_t data2_size;
	ssize_t count, expected;
	unsigned int i;

	if (fd < 0) {
		errno = EINVAL;
		return -1;
	}

	if (!data1)
		data1 = "";
	if (!data2)
		data2 = "";

	data1_size = strlen(data1) + 1;
	data2_size = strlen(data2) + 1;

	iov[0].iov_base = &function;
	iov[0].iov_len = sizeof(function);
	iov[1].iov_base = &data1_size;
	iov[1].iov_len = sizeof(data1_size);
	iov[2].iov_base = &data2_size;
	iov[2].iov_len = sizeof(data2_size);
	iov[3].iov_base = (char *)data1;
	iov[3].iov_len = data1_size;
	iov[4].iov_base = (char *)data2;
	iov[4].iov_len = data2_size;
	memset(&msgh, 0, sizeof(msgh));
	msgh.msg_iov = iov;
	msgh.msg_iovlen = sizeof(iov) / sizeof(iov[0]);

	expected = 0;
	for (i = 0; i < sizeof(iov) / sizeof(iov[0]); i++)
		expected += iov[i].iov_len;

	while (((count = sendmsg(fd, &msgh, MSG_NOSIGNAL)) < 0)
	       && (errno == EINTR)) ;
	if (count < 0)
		return -1;
	if (count != expected) {
		errno = EBADMSG;
		return -1;
	}

	return 0;
}

/* Returns: 0 on success, <0 on failure */
static int
receive_response(int fd, uint32_t function, char **outdata, int32_t * ret_val)
{
	struct iovec resp_hdr[3];
	uint32_t func;
	uint32_t data_size;
	char *data;
	struct iovec resp_data;
	ssize_t count;

	if (fd < 0) {
		errno = EINVAL;
		return -1;
	}

	resp_hdr[0].iov_base = &func;
	resp_hdr[0].iov_len = sizeof(func);
	resp_hdr[1].iov_base = &data_size;
	resp_hdr[1].iov_len = sizeof(data_size);
	resp_hdr[2].iov_base = ret_val;
	resp_hdr[2].iov_len = sizeof(*ret_val);

	while (((count = readv(fd, resp_hdr, 3)) < 0) && (errno == EINTR)) ;
	if (count < 0) {
		return -1;
	}

	if (count != (sizeof(func) + sizeof(data_size) + sizeof(*ret_val))) {
		errno = EBADMSG;
		return -1;
	}

	if (func != function || !data_size || data_size > MAX_DATA_BUF) {
		errno = EBADMSG;
		return -1;
	}

	/* coveriety doesn't realize that data will be initialized in readv */
	data = calloc(1, data_size);
	if (!data)
		return -1;

	resp_data.iov_base = data;
	resp_data.iov_len = data_size;

	while (((count = readv(fd, &resp_data, 1))) < 0 && (errno == EINTR)) ;
	if (count < 0 || (uint32_t) count != data_size ||
	    data[data_size - 1] != '\0') {
		free(data);
		if (count >= 0)
			errno = EBADMSG;
		return -1;
	}
	*outdata = data;
	return 0;
}

static int raw_to_trans_context(const char *raw, char **transp)
{
	int ret;
	int32_t ret_val;
	int fd;

	*transp = NULL;

	fd = setransd_open();
	if (fd < 0)
		return fd;

	ret = send_request(fd, RAW_TO_TRANS_CONTEXT, raw, NULL);
	if (ret)
		goto out;

	ret = receive_response(fd, RAW_TO_TRANS_CONTEXT, transp, &ret_val);
	if (ret)
		goto out;

	ret = ret_val;
      out:
	close(fd);
	return ret;
}

static int trans_to_raw_context(const char *trans, char **rawp)
{
	int ret;
	int32_t ret_val;
	int fd;

	*rawp = NULL;

	fd = setransd_open();
	if (fd < 0)
		return fd;
	ret = send_request(fd, TRANS_TO_RAW_CONTEXT, trans, NULL);
	if (ret)
		goto out;

	ret = receive_response(fd, TRANS_TO_RAW_CONTEXT, rawp, &ret_val);
	if (ret)
		goto out;

	ret = ret_val;
      out:
	close(fd);
	return ret;
}

static int raw_context_to_color(const char *raw, char **colors)
{
	int ret;
	int32_t ret_val;
	int fd;

	fd = setransd_open();
	if (fd < 0)
		return fd;

	ret = send_request(fd, RAW_CONTEXT_TO_COLOR, raw, NULL);
	if (ret)
		goto out;

	ret = receive_response(fd, RAW_CONTEXT_TO_COLOR, colors, &ret_val);
	if (ret)
		goto out;

	ret = ret_val;
out:
	close(fd);
	return ret;
}

static void setrans_thread_destructor(void __attribute__((unused)) *unused)
{
	free(prev_t2r_trans);
	free(prev_t2r_raw);
	free(prev_r2t_trans);
	free(prev_r2t_raw);
	free(prev_r2c_trans);
	free(prev_r2c_raw);
}

void __attribute__((destructor)) setrans_lib_destructor(void);

void  __attribute__((destructor)) setrans_lib_destructor(void)
{
	if (!has_setrans)
		return;
	if (destructor_key_initialized)
		__selinux_key_delete(destructor_key);
}

static inline void init_thread_destructor(void)
{
	if (!has_setrans)
		return;
	if (destructor_initialized == 0) {
		__selinux_setspecific(destructor_key, /* some valid address to please GCC */ &selinux_page_size);
		destructor_initialized = 1;
	}
}

static void init_context_translations(void)
{
	has_setrans = (access(SETRANS_UNIX_SOCKET, F_OK) == 0);
	if (!has_setrans)
		return;
	if (__selinux_key_create(&destructor_key, setrans_thread_destructor) == 0)
		destructor_key_initialized = 1;
}

int selinux_trans_to_raw_context(const char * trans,
				 char ** rawp)
{
	if (!trans) {
		*rawp = NULL;
		return 0;
	}

	__selinux_once(once, init_context_translations);
	init_thread_destructor();

	if (!has_setrans) {
		*rawp = strdup(trans);
		goto out;
	}

	if (prev_t2r_trans && strcmp(prev_t2r_trans, trans) == 0) {
		*rawp = strdup(prev_t2r_raw);
	} else {
		free(prev_t2r_trans);
		prev_t2r_trans = NULL;
		free(prev_t2r_raw);
		prev_t2r_raw = NULL;
		if (trans_to_raw_context(trans, rawp))
			*rawp = strdup(trans);
		if (*rawp) {
			prev_t2r_trans = strdup(trans);
			if (!prev_t2r_trans)
				goto out;
			prev_t2r_raw = strdup(*rawp);
			if (!prev_t2r_raw) {
				free(prev_t2r_trans);
				prev_t2r_trans = NULL;
			}
		}
	}
      out:
	return *rawp ? 0 : -1;
}


int selinux_raw_to_trans_context(const char * raw,
				 char ** transp)
{
	if (!raw) {
		*transp = NULL;
		return 0;
	}

	__selinux_once(once, init_context_translations);
	init_thread_destructor();

	if (!has_setrans)  {
		*transp = strdup(raw);
		goto out;
	}

	if (prev_r2t_raw && strcmp(prev_r2t_raw, raw) == 0) {
		*transp = strdup(prev_r2t_trans);
	} else {
		free(prev_r2t_raw);
		prev_r2t_raw = NULL;
		free(prev_r2t_trans);
		prev_r2t_trans = NULL;
		if (raw_to_trans_context(raw, transp))
			*transp = strdup(raw);
		if (*transp) {
			prev_r2t_raw = strdup(raw);
			if (!prev_r2t_raw)
				goto out;
			prev_r2t_trans = strdup(*transp);
			if (!prev_r2t_trans) {
				free(prev_r2t_raw);
				prev_r2t_raw = NULL;
			}
		}
	}
      out:
	return *transp ? 0 : -1;
}


int selinux_raw_context_to_color(const char * raw, char **transp)
{
	if (!raw) {
		*transp = NULL;
		return -1;
	}

	__selinux_once(once, init_context_translations);
	init_thread_destructor();

	if (!has_setrans) {
		*transp = strdup(raw);
		goto out;
	}

	if (prev_r2c_raw && strcmp(prev_r2c_raw, raw) == 0) {
		*transp = strdup(prev_r2c_trans);
	} else {
		free(prev_r2c_raw);
		prev_r2c_raw = NULL;
		free(prev_r2c_trans);
		prev_r2c_trans = NULL;
		if (raw_context_to_color(raw, transp))
			return -1;
		if (*transp) {
			prev_r2c_raw = strdup(raw);
			if (!prev_r2c_raw)
				goto out;
			prev_r2c_trans = strdup(*transp);
			if (!prev_r2c_trans) {
				free(prev_r2c_raw);
				prev_r2c_raw = NULL;
			}
		}
	}
      out:
	return *transp ? 0 : -1;
}

#else /*DISABLE_SETRANS*/

int selinux_trans_to_raw_context(const char * trans,
				 char ** rawp)
{
	if (!trans) {
		*rawp = NULL;
		return 0;
	}

	*rawp = strdup(trans);
	
	return *rawp ? 0 : -1;
}


int selinux_raw_to_trans_context(const char * raw,
				 char ** transp)
{
	if (!raw) {
		*transp = NULL;
		return 0;
	}
	*transp = strdup(raw);
	
	return *transp ? 0 : -1;
}

#endif /*DISABLE_SETRANS*/
