#ifndef BPF_COMMON_H
#define BPF_COMMON_H

#include <bpf/bpf_core_read.h>

#include "../helpers.h"

// Uncomment to enable debug messages via bpf_printk
//#define PRINT_DEBUG_MSGS

// Maximum monitored sessions.
#define MAX_MONITORED_SESSIONS 1024

#define FILENAMESIZE 512

// ARGSIZE specifies the max argument size read.
#define _ARGSIZE 1024
#define _MAXARGS 20
#define ARGBUFSIZE (_ARGSIZE * _MAXARGS)

// Easier to use bpf_printk taken from https://nakryiko.com/posts/bpf-tips-printk/

// Define our own struct definition if our vmlinux.h is outdated; this empty
// definition will not conflict because of the ___x but anything after the
// triple underscore will get ignored by BPF CO-RE.
struct trace_event_raw_bpf_trace_printk___x {};

#undef bpf_printk

#ifdef PRINT_DEBUG_MSGS
#define bpf_printk(fmt, ...)                                                   \
    ({                                                                         \
        static char ____fmt[] = fmt "\0";                                      \
        if (bpf_core_type_exists(struct trace_event_raw_bpf_trace_printk___x)) \
        {                                                                      \
            bpf_trace_printk(____fmt, sizeof(____fmt) - 1, ##__VA_ARGS__);     \
        }                                                                      \
        else                                                                   \
        {                                                                      \
            ____fmt[sizeof(____fmt) - 2] = '\n';                               \
            bpf_trace_printk(____fmt, sizeof(____fmt), ##__VA_ARGS__);         \
        }                                                                      \
    })

static void print_event(struct task_struct *task) {
    char comm[TASK_COMM_LEN];
    bpf_get_current_comm(&comm, sizeof(comm));
    u32 session_id = BPF_CORE_READ(task, sessionid);

    bpf_printk("  comm:       %s", comm);
    bpf_printk("  pid:        %d", bpf_get_current_pid_tgid() >> 32);
    bpf_printk("  session ID: %lu", session_id);
}

static void print_command_event(struct task_struct *task, const u8 filename[FILENAMESIZE], const u8 argv[ARGBUFSIZE]) {
    const char *arg = NULL;

    bpf_printk("command:");
    print_event(task);
    bpf_printk("  filename:   %s", filename);
    bpf_printk("  argv[0]:    %s", argv);
}

static void print_disk_event(struct task_struct *task, const char *path) {
    bpf_printk("disk:");
    print_event(task);
    bpf_printk("  path:       %s", path);
}
#else
#define bpf_printk(fmt, ...)
static void print_event(struct task_struct *task) {}
static void print_command_event(struct task_struct *task, const u8 filename[FILENAMESIZE], const u8 argv[ARGBUFSIZE]) {}
static void print_disk_event(struct task_struct *task, const char *path) {}
#endif

#endif // BPF_COMMON_H
