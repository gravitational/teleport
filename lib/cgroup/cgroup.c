// +build linux

/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

#define _GNU_SOURCE
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>

// cgroup_id comes from get_cgroup_id in the Linux kernel.
// https://github.com/torvalds/linux/commit/f269099a7e7a0c6732c4a817d0e99e92216414d9
uint64_t cgroup_id(char *path)
{
	int dirfd, err, flags, mount_id, fhsize;
	union {
		unsigned long long cgid;
		unsigned char raw_bytes[8];
	} id;
	struct file_handle *fhp, *fhp2;
	unsigned long long ret = 0;

 	dirfd = AT_FDCWD;
	flags = 0;
	fhsize = sizeof(*fhp);
	fhp = calloc(1, fhsize);
	if (!fhp) {
		return 0;
	}
	err = name_to_handle_at(dirfd, path, fhp, &mount_id, flags);
	if (err >= 0 || fhp->handle_bytes != 8) {
		goto free_mem;
	}

 	fhsize = sizeof(struct file_handle) + fhp->handle_bytes;
	fhp2 = realloc(fhp, fhsize);
	if (!fhp2) {
		goto free_mem;
	}
	err = name_to_handle_at(dirfd, path, fhp2, &mount_id, flags);
	fhp = fhp2;
	if (err < 0) {
		goto free_mem;
	}

 	memcpy(id.raw_bytes, fhp->f_handle, 8);
	ret = id.cgid;

 free_mem:
	free(fhp);
	return ret;
}
