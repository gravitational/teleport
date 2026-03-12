// This file contains code from Aqua Security's tracee project,
// originally found at https://github.com/aquasecurity/tracee/blob/ca359039b60a8662810136ed9ad540565128c63d/pkg/ebpf/c/common/filesystem.h.
//
// The original code is licensed under the Apache License, Version 2.0.
// The original copyright notice is included below.
//
// Copyright 2019 Aqua Security Software Ltd.
//
// This product includes software developed by Aqua Security (https://aquasec.com).

#include "../vmlinux.h"
#include <linux/limits.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

#define DCACHE_DISCONNECTED (1 << 5)

#define MAX_PATH_COMPONENTS 20

struct buf_t {
    char buf[PATH_MAX];
};

static struct mount *real_mount(struct vfsmount *mnt)
{
    return container_of(mnt, struct mount, mnt);
}

static struct dentry *get_mnt_root_ptr_from_vfsmnt(struct vfsmount *vfsmnt)
{
    return BPF_CORE_READ(vfsmnt, mnt_root);
}

static struct dentry *get_d_parent_ptr_from_dentry(struct dentry *dentry)
{
    return BPF_CORE_READ(dentry, d_parent);
}

static struct qstr get_d_name_from_dentry(struct dentry *dentry)
{
    return BPF_CORE_READ(dentry, d_name);
}

// Read the file path to the given buffer, returning the start offset of the path.
static size_t get_path_str_buf(struct path *path, struct buf_t *out_buf)
{
    if (path == NULL || out_buf == NULL) {
        return 0;
    }

    // Disconnected dentry - the file in question is not in
    // the dentry cache and its path cannot be constructed.
    struct dentry *dentry = BPF_CORE_READ(path, dentry);
    if (BPF_CORE_READ(dentry, d_flags) & DCACHE_DISCONNECTED) {
        bpf_probe_read_kernel_str(&(out_buf->buf[0]), PATH_MAX, "<disconnected>");
        return 0;
    }

    char slash = '/';
    int zero = 0;
    struct vfsmount *vfsmnt = BPF_CORE_READ(path, mnt);
    struct mount *mnt_parent_p;
    struct mount *mnt_p = real_mount(vfsmnt);
    bpf_core_read(&mnt_parent_p, sizeof(struct mount *), &mnt_p->mnt_parent);
    u32 buf_off = (PATH_MAX >> 1);
    struct dentry *mnt_root;
    struct dentry *d_parent;
    struct qstr d_name;
    unsigned int len;
    unsigned int off;
    int sz;

#pragma unroll
    for (int i = 0; i < MAX_PATH_COMPONENTS; i++) {
        mnt_root = get_mnt_root_ptr_from_vfsmnt(vfsmnt);
        d_parent = get_d_parent_ptr_from_dentry(dentry);
        if (dentry == mnt_root || dentry == d_parent) {
            if (dentry != mnt_root) {
                // We reached root, but not mount root - escaped?
                break;
            }
            if (mnt_p != mnt_parent_p) {
                // We reached root, but not global root - continue with mount point path
                bpf_core_read(&dentry, sizeof(struct dentry *), &mnt_p->mnt_mountpoint);
                bpf_core_read(&mnt_p, sizeof(struct mount *), &mnt_p->mnt_parent);
                bpf_core_read(&mnt_parent_p, sizeof(struct mount *), &mnt_p->mnt_parent);
                vfsmnt = &mnt_p->mnt;
                continue;
            }
            // Global root - path fully parsed
            break;
        }
        // Add this dentry name to path
        d_name = get_d_name_from_dentry(dentry);
        len = (d_name.len + 1) & (PATH_MAX - 1);
        off = buf_off - len;
        // Is string buffer big enough for dentry name?
        sz = 0;
        if (off <= buf_off) { // verify no wrap occurred
            len = len & ((PATH_MAX >> 1) - 1);
            sz = bpf_probe_read_kernel_str(
                &(out_buf->buf[off & ((PATH_MAX >> 1) - 1)]), len, (void *) d_name.name);
        } else
            break;
        if (sz > 1) {
            buf_off -= 1; // remove null byte termination with slash sign
            bpf_probe_read_kernel(&(out_buf->buf[buf_off & (PATH_MAX - 1)]), 1, &slash);
            buf_off -= sz - 1;
        } else {
            // If sz is 0 or 1 we have an error (path can't be null nor an empty string)
            break;
        }
        dentry = d_parent;
    }
    if (buf_off == (PATH_MAX >> 1)) {
        // memfd files have no path in the filesystem -> extract their name
        buf_off = 0;
        d_name = get_d_name_from_dentry(dentry);
        bpf_probe_read_kernel_str(&(out_buf->buf[0]), PATH_MAX, (void *) d_name.name);
    } else {
        // Add leading slash
        buf_off -= 1;
        bpf_probe_read_kernel(&(out_buf->buf[buf_off & (PATH_MAX - 1)]), 1, &slash);
        // Null terminate the path string
        bpf_probe_read_kernel(&(out_buf->buf[(PATH_MAX >> 1) - 1]), 1, &zero);
    }
    return buf_off;
}