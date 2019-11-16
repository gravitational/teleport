// Copyright 2018 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include "_cgo_export.h"
#include <bcc/bcc_common.h>
#include <bcc/libbpf.h>
#include <dlfcn.h>

// bcc_library_name returns the name of the library to load at runtime.
char *bcc_library_name()
{
    return "libbcc.so.0";
}

// Position in the symbol lookup table.
#define BCC_FOREACH_FUNCTION_SYMBOL     0
#define BCC_PROG_LOAD                   1
#define BCC_RESOLVE_SYMNAME             2
#define BCC_SYMCACHE_NEW                3
#define BCC_SYMCACHE_RESOLVE_NAME       4
#define BPF_ATTACH_KPROBE               5
#define BPF_ATTACH_PERF_EVENT           6
#define BPF_ATTACH_RAW_TRACEPOINT       7
#define BPF_ATTACH_TRACEPOINT           8
#define BPF_ATTACH_UPROBE               9
#define BPF_ATTACH_XDP                  10
#define BPF_CLOSE_PERF_EVENT_FD         11
#define BPF_DELETE_ELEM                 12
#define BPF_DETACH_KPROBE               13
#define BPF_DETACH_TRACEPOINT           14
#define BPF_DETACH_UPROBE               15
#define BPF_FUNCTION_SIZE               16
#define BPF_FUNCTION_START              17
#define BPF_GET_FIRST_KEY               18
#define BPF_GET_NEXT_KEY                19
#define BPF_LOOKUP_ELEM                 20
#define BPF_MODULE_CREATE_C_FROM_STRING 21
#define BPF_MODULE_DESTROY              22
#define BPF_MODULE_KERN_VERSION         23
#define BPF_MODULE_LICENSE              24
#define BPF_NUM_TABLES                  25
#define BPF_OPEN_PERF_BUFFER            26
#define BPF_PROG_GET_TAG                27
#define BPF_TABLE_FD_ID                 28
#define BPF_TABLE_ID                    29
#define BPF_TABLE_KEY_DESC_ID           30
#define BPF_TABLE_KEY_SIZE_ID           31
#define BPF_TABLE_KEY_SNPRINTF          32
#define BPF_TABLE_KEY_SSCANF            33
#define BPF_TABLE_LEAF_DESC_ID          34
#define BPF_TABLE_LEAF_SIZE_ID          35
#define BPF_TABLE_LEAF_SNPRINTF         36
#define BPF_TABLE_LEAF_SSCANF           37
#define BPF_TABLE_NAME                  38
#define BPF_UPDATE_ELEM                 39
#define PERF_READER_FD                  40
#define PERF_READER_POLL                41
#define SYM_TABLE_SIZE                  42

// symlookup is a table of function pointers to all libbcc functions called
// in gobpf.
void *symlookup[SYM_TABLE_SIZE];

// init_symlookup initializes the symbol lookup table.
int init_symlookup(void *handle) {
    symlookup[BCC_FOREACH_FUNCTION_SYMBOL    ] = dlsym(handle, "bcc_foreach_function_symbol");
    symlookup[BCC_PROG_LOAD                  ] = dlsym(handle, "bcc_prog_load");
    symlookup[BCC_RESOLVE_SYMNAME            ] = dlsym(handle, "bcc_resolve_symname");
    symlookup[BCC_SYMCACHE_NEW               ] = dlsym(handle, "bcc_symcache_new");
    symlookup[BCC_SYMCACHE_RESOLVE_NAME      ] = dlsym(handle, "bcc_symcache_resolve_name");
    symlookup[BPF_ATTACH_KPROBE              ] = dlsym(handle, "bpf_attach_kprobe");
    symlookup[BPF_ATTACH_PERF_EVENT          ] = dlsym(handle, "bpf_attach_perf_event");
    symlookup[BPF_ATTACH_RAW_TRACEPOINT      ] = dlsym(handle, "bpf_attach_raw_tracepoint");
    symlookup[BPF_ATTACH_TRACEPOINT          ] = dlsym(handle, "bpf_attach_tracepoint");
    symlookup[BPF_ATTACH_UPROBE              ] = dlsym(handle, "bpf_attach_uprobe");
    symlookup[BPF_ATTACH_XDP                 ] = dlsym(handle, "bpf_attach_xdp");
    symlookup[BPF_CLOSE_PERF_EVENT_FD        ] = dlsym(handle, "bpf_close_perf_event_fd");
    symlookup[BPF_DELETE_ELEM                ] = dlsym(handle, "bpf_delete_elem");
    symlookup[BPF_DETACH_KPROBE              ] = dlsym(handle, "bpf_detach_kprobe");
    symlookup[BPF_DETACH_TRACEPOINT          ] = dlsym(handle, "bpf_detach_tracepoint");
    symlookup[BPF_DETACH_UPROBE              ] = dlsym(handle, "bpf_detach_uprobe");
    symlookup[BPF_FUNCTION_SIZE              ] = dlsym(handle, "bpf_function_size");
    symlookup[BPF_FUNCTION_START             ] = dlsym(handle, "bpf_function_start");
    symlookup[BPF_GET_FIRST_KEY              ] = dlsym(handle, "bpf_get_first_key");
    symlookup[BPF_GET_NEXT_KEY               ] = dlsym(handle, "bpf_get_next_key");
    symlookup[BPF_LOOKUP_ELEM                ] = dlsym(handle, "bpf_lookup_elem");
    symlookup[BPF_MODULE_CREATE_C_FROM_STRING] = dlsym(handle, "bpf_module_create_c_from_string");
    symlookup[BPF_MODULE_DESTROY             ] = dlsym(handle, "bpf_module_destroy");
    symlookup[BPF_MODULE_KERN_VERSION        ] = dlsym(handle, "bpf_module_kern_version");
    symlookup[BPF_MODULE_LICENSE             ] = dlsym(handle, "bpf_module_license");
    symlookup[BPF_NUM_TABLES                 ] = dlsym(handle, "bpf_num_tables");
    symlookup[BPF_OPEN_PERF_BUFFER           ] = dlsym(handle, "bpf_open_perf_buffer");
    symlookup[BPF_PROG_GET_TAG               ] = dlsym(handle, "bpf_prog_get_tag");
    symlookup[BPF_TABLE_FD_ID                ] = dlsym(handle, "bpf_table_fd_id");
    symlookup[BPF_TABLE_ID                   ] = dlsym(handle, "bpf_table_id");
    symlookup[BPF_TABLE_KEY_DESC_ID          ] = dlsym(handle, "bpf_table_key_desc_id");
    symlookup[BPF_TABLE_KEY_SIZE_ID          ] = dlsym(handle, "bpf_table_key_size_id");
    symlookup[BPF_TABLE_KEY_SNPRINTF         ] = dlsym(handle, "bpf_table_key_snprintf");
    symlookup[BPF_TABLE_KEY_SSCANF           ] = dlsym(handle, "bpf_table_key_sscanf");
    symlookup[BPF_TABLE_LEAF_DESC_ID         ] = dlsym(handle, "bpf_table_leaf_desc_id");
    symlookup[BPF_TABLE_LEAF_SIZE_ID         ] = dlsym(handle, "bpf_table_leaf_size_id");
    symlookup[BPF_TABLE_LEAF_SNPRINTF        ] = dlsym(handle, "bpf_table_leaf_snprintf");
    symlookup[BPF_TABLE_LEAF_SSCANF          ] = dlsym(handle, "bpf_table_leaf_sscanf");
    symlookup[BPF_TABLE_NAME                 ] = dlsym(handle, "bpf_table_name");
    symlookup[BPF_UPDATE_ELEM                ] = dlsym(handle, "bpf_update_elem");
    symlookup[PERF_READER_FD                 ] = dlsym(handle, "perf_reader_fd");
    symlookup[PERF_READER_POLL               ] = dlsym(handle, "perf_reader_poll");                    

    // Make sure all symbols were resolvable.
    int i;
    for (i = 0; i < SYM_TABLE_SIZE; i++)  {
        if (symlookup[i] == NULL) {
            return -1;
        }
    }
    return 0;
}

int bcc_foreach_function_symbol(const char *module, SYM_CB cb)
{
    int (*f)(const char *, SYM_CB);
    f = symlookup[BCC_FOREACH_FUNCTION_SYMBOL];
    return (f)(module, cb);
}

int bcc_prog_load(enum bpf_prog_type prog_type, const char *name,
                   const struct bpf_insn *insns, int prog_len,
                   const char *license, unsigned kern_version,
                   int log_level, char *log_buf, unsigned log_buf_size)
{
    int (*f)(enum bpf_prog_type, const char *, const struct bpf_insn *, int,
                   const char *, unsigned, int, char *, unsigned);
    f = symlookup[BCC_PROG_LOAD];
    return (f)(prog_type, name, insns, prog_len, license, kern_version, log_level, log_buf, log_buf_size);
}


int bcc_resolve_symname(const char *module, const char *symname,
                        const uint64_t addr, int pid,
                        struct bcc_symbol_option* option,
                        struct bcc_symbol *sym)
{
    int (*f)(const char *, const char *, const uint64_t, int, struct bcc_symbol_option*, struct bcc_symbol *);
    f = symlookup[BCC_RESOLVE_SYMNAME];
    return (f)(module, symname, addr, pid, option, sym);
}

void *bcc_symcache_new(int pid, struct bcc_symbol_option *option)
{
    void *(*f)(int, struct bcc_symbol_option*);
    f = symlookup[BCC_SYMCACHE_NEW];
    return (f)(pid, option);
}

int bcc_symcache_resolve_name(void *resolver, const char *module,
                              const char *name, uint64_t *addr)
{
    int (*f)(void *, const char *, const char *, uint64_t *);
    f = symlookup[BCC_SYMCACHE_RESOLVE_NAME];
    return (f)(resolver, module, name, addr);
}

int bpf_attach_kprobe(int progfd, enum bpf_probe_attach_type attach_type,
                      const char *ev_name, const char *fn_name, uint64_t fn_offset,
                      int maxactive)
{
    int (*f)(int, enum bpf_probe_attach_type, const char*, const char *, uint64_t, int);
    f = symlookup[BPF_ATTACH_KPROBE];
    return (f)(progfd, attach_type, ev_name, fn_name, fn_offset, maxactive);
}

int bpf_attach_perf_event(int progfd, uint32_t ev_type, uint32_t ev_config,
                          uint64_t sample_period, uint64_t sample_freq,
                          pid_t pid, int cpu, int group_fd)
{
    int (*f)(int, uint32_t, uint32_t, uint64_t, uint64_t, pid_t, int, int);
    f = symlookup[BPF_ATTACH_PERF_EVENT];
    return (f)(progfd, ev_type, ev_config, sample_period, sample_freq, pid, cpu, group_fd);
}

int bpf_attach_raw_tracepoint(int progfd, char *tp_name)
{
    int (*f)(int, char*);
    f = symlookup[BPF_ATTACH_RAW_TRACEPOINT];
    return (f)(progfd, tp_name);
}

int bpf_attach_tracepoint(int progfd, const char *tp_category,
                          const char *tp_name)
{
    int (*f)(int, const char *, const char*);
    f = symlookup[BPF_ATTACH_TRACEPOINT];
    return (f)(progfd, tp_category, tp_name);
}

int bpf_attach_uprobe(int progfd, enum bpf_probe_attach_type attach_type,
                      const char *ev_name, const char *binary_path,
                      uint64_t offset, pid_t pid)
{
    int (*f)(int, enum bpf_probe_attach_type, const char *, const char *, uint64_t, pid_t);
    f = symlookup[BPF_ATTACH_UPROBE];
    return (f)(progfd, attach_type, ev_name, binary_path, offset, pid);
}

int bpf_attach_xdp(const char *dev_name, int progfd, uint32_t flags)
{
    int (*f)(const char *, int, uint32_t);
    f = symlookup[BPF_ATTACH_XDP];
    return (f)(dev_name, progfd, flags);
}

int bpf_close_perf_event_fd(int fd)
{
    int (*f)(int);
    f = symlookup[BPF_CLOSE_PERF_EVENT_FD];
    return (f)(fd);
}

int bpf_delete_elem(int fd, void *key)
{
    int (*f)(int, void *);
    f = symlookup[BPF_DELETE_ELEM];
    return (f)(fd, key);
}

int bpf_detach_kprobe(const char *ev_name)
{
    int (*f)(const char *);
    f = symlookup[BPF_DETACH_KPROBE];
    return (f)(ev_name);
}

int bpf_detach_tracepoint(const char *tp_category, const char *tp_name)
{
    int (*f)(const char *, const char *);
    f = symlookup[BPF_DETACH_TRACEPOINT];
    return (f)(tp_category, tp_name);
}

int bpf_detach_uprobe(const char *ev_name)
{
    int (*f)(const char *);
    f = symlookup[BPF_DETACH_UPROBE];
    return (f)(ev_name);
}

size_t bpf_function_size(void *program, const char *name)
{
    size_t (*f)(void *, const char *);
    f = symlookup[BPF_FUNCTION_SIZE];
    return (f)(program, name);
}

void *bpf_function_start(void *program, const char *name)
{
    void *(*f)(void *, const char *);
    f = symlookup[BPF_FUNCTION_START];
    return (f)(program, name);
}


int bpf_get_first_key(int fd, void *key, size_t key_size)
{
    int (*f)(int, void *, size_t);
    f = symlookup[BPF_GET_FIRST_KEY];
    return (f)(fd, key, key_size);
}

int bpf_get_next_key(int fd, void *key, void *next_key)
{
    int (*f)(int, void *, void *);
    f = symlookup[BPF_GET_NEXT_KEY];
    return (f)(fd, key, next_key);
}

int bpf_lookup_elem(int fd, void *key, void *value)
{
    int (*f)(int, void *, void *);
    f = symlookup[BPF_LOOKUP_ELEM];
    return (f)(fd, key, value);
}

void *bpf_module_create_c_from_string(const char *text, unsigned flags, const char *cflags[],
                                       int ncflags, bool allow_rlimit, const char *dev_name)
{
    void *(*f)(const char *, unsigned, const char *[], int, bool, const char *);
    f = symlookup[BPF_MODULE_CREATE_C_FROM_STRING];
    return (f)(text, flags, cflags, ncflags, allow_rlimit, dev_name);
}

void bpf_module_destroy(void *program)
{
    void (*f)(void *);
    f = symlookup[BPF_MODULE_DESTROY];
    (f)(program);
    return;
}

unsigned bpf_module_kern_version(void *program)
{
    unsigned (*f)(void *);
    f = symlookup[BPF_MODULE_KERN_VERSION];
    return (f)(program);
}

char *bpf_module_license(void *program)
{
    char *(*f)(void *);
    f = symlookup[BPF_MODULE_LICENSE];
    return (f)(program);
}

size_t bpf_num_tables(void *program)
{
    size_t (*f)(void *);
    f = symlookup[BPF_NUM_TABLES];
    return (f)(program);
}

void *bpf_open_perf_buffer(perf_reader_raw_cb raw_cb,
                            perf_reader_lost_cb lost_cb, void *cb_cookie,
                            int pid, int cpu, int page_cnt)
{
    void *(*f)(perf_reader_raw_cb, perf_reader_lost_cb, void *, int, int, int);
    f = symlookup[BPF_OPEN_PERF_BUFFER];
    return (f)(raw_cb, lost_cb, cb_cookie, pid, cpu, page_cnt);
}

int bpf_prog_get_tag(int fd, unsigned long long *tag)
{
    int (*f)(int, unsigned long long*);
    f = symlookup[BPF_PROG_GET_TAG];
    return (f)(fd, tag);
}

int bpf_table_fd_id(void *program, size_t id)
{
    int (*f)(void *, size_t);
    f = symlookup[BPF_TABLE_FD_ID];
    return (f)(program, id);
}


size_t bpf_table_id(void *program, const char *table_name)
{
    size_t (*f)(void *, const char *);
    f = symlookup[BPF_TABLE_ID];
    return (f)(program, table_name);
}

const char *bpf_table_key_desc_id(void *program, size_t id)
{
    const char *(*f)(void *, size_t);
    f = symlookup[BPF_TABLE_KEY_DESC_ID];
    return (f)(program, id);
}

size_t bpf_table_key_size_id(void *program, size_t id)
{
    size_t (*f)(void *, size_t);
    f = symlookup[BPF_TABLE_KEY_SIZE_ID];
    return (f)(program, id);
}

int bpf_table_key_snprintf(void *program, size_t id, char *buf, size_t buflen, const void *key)
{
    int (*f)(void *, size_t, char *, size_t, const void *);
    f = symlookup[BPF_TABLE_KEY_SNPRINTF];
    return (f)(program, id, buf, buflen, key);
}

int bpf_table_key_sscanf(void *program, size_t id, const char *buf, void *key)
{
    int (*f)(void *, size_t, const char *, void *);
    f = symlookup[BPF_TABLE_KEY_SSCANF];
    return (f)(program, id, buf, key);
}

const char *bpf_table_leaf_desc_id(void *program, size_t id)
{
    const char *(*f)(void *, size_t);
    f = symlookup[BPF_TABLE_LEAF_DESC_ID];
    return (f)(program, id);
}

size_t bpf_table_leaf_size_id(void *program, size_t id)
{
    size_t (*f)(void *, size_t);
    f = symlookup[BPF_TABLE_LEAF_SIZE_ID];
    return (f)(program, id);
}

int bpf_table_leaf_snprintf(void *program, size_t id, char *buf, size_t buflen, const void *leaf)
{
    int (*f)(void *, size_t, char *, size_t, const void *);
    f = symlookup[BPF_TABLE_LEAF_SNPRINTF];
    return (f)(program, id, buf, buflen, leaf);
}

int bpf_table_leaf_sscanf(void *program, size_t id, const char *buf, void *leaf)
{
    int (*f)(void *, size_t, const char *, void *);
    f = symlookup[BPF_TABLE_LEAF_SSCANF];
    return (f)(program, id, buf, leaf);
}

const char *bpf_table_name(void *program, size_t id)
{
    const char *(*f)(void *, size_t);
    f = symlookup[BPF_TABLE_NAME];
    return (f)(program, id);
}

int bpf_update_elem(int fd, void *key, void *value, unsigned long long flags)
{
    int (*f)(int, void *, void *, unsigned long long);
    f = symlookup[BPF_UPDATE_ELEM];
    return (f)(fd, key, value, flags);
}

int perf_reader_fd(struct perf_reader *reader)
{
    int (*f)(struct perf_reader *);
    f = symlookup[PERF_READER_FD];
    return (f)(reader);
}

int perf_reader_poll(int num_readers, struct perf_reader **readers, int timeout)
{
    int (*f)(int, struct perf_reader **, int);
    f = symlookup[PERF_READER_POLL];
    return (f)(num_readers, readers, timeout);
}
