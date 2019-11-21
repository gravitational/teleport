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
#include <stdlib.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>

// cgid_file_handle comes from bpftrace, see:
// https://github.com/iovisor/bpftrace/blob/master/src/resolve_cgroupid.cpp
struct cgid_file_handle {
	unsigned int handle_bytes;
	int handle_type;
	uint64_t cgid;
};

// cgroup_id returns the ID of the given cgroup at path.
uint64_t cgroup_id(char *path)
{
    int ret;
    int mount_id;
    struct cgid_file_handle *handle;

    handle = malloc(sizeof(struct cgid_file_handle));
    if (handle == NULL) {
        return 0;
    }
    handle->handle_bytes = sizeof(uint64_t);

	ret = name_to_handle_at(AT_FDCWD, path, (struct file_handle *)handle, &mount_id, 0);
    if (ret != 0) {
        return 0;
	}

    free(handle);

    return handle->cgid;
}
