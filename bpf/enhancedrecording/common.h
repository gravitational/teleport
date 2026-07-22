#ifndef BPF_COMMON_H
#define BPF_COMMON_H

#include <bpf/bpf_core_read.h>

#include "../helpers.h"

// Uncomment to enable debug messages via bpf_printk
//#define PRINT_DEBUG_MSGS

// Maximum monitored sessions.
#define MAX_MONITORED_SESSIONS 1024

#define FILENAMESIZE 512

#define MAXARGLEN 1024
#define MAXARGS 20
#define ARGBUFSIZE (MAXARGLEN * MAXARGS)

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

// MAXARGLEN is too large to fit in the eBPF stack
#define PRINT_MAX_ARG_LEN 256

static void print_command_event(struct task_struct *task, const u8 filename[FILENAMESIZE], const u8 argv[ARGBUFSIZE], int return_code) {
    const char *arg = NULL;

    bpf_printk("command:");
    print_event(task);
    bpf_printk("  filename:   %s", filename);

    u32 offset = 0;
    u8 argp[PRINT_MAX_ARG_LEN] = {0};
    for (int i = 0; i < MAXARGS; i++) {
        if (offset >= ARGBUFSIZE - PRINT_MAX_ARG_LEN) {
            break;
        }

        long ret = bpf_probe_read_kernel_str(&argp, PRINT_MAX_ARG_LEN, &argv[offset]);
        if (ret <= 0 || argp[0] == 0) {
            break;
        }

        bpf_printk("  argv[%d]:    %s", i, argp);

        offset += ret;
    }

    bpf_printk("  retcode:    %d", return_code);
}

static void print_disk_event(struct task_struct *task, const char *path, int return_code) {
    bpf_printk("disk:");
    print_event(task);
    bpf_printk("  path:       %s", path);
    bpf_printk("  retcode:    %d", return_code);
}
#else
#define bpf_printk(fmt, ...)
static void print_event(struct task_struct *task) {}
static void print_command_event(struct task_struct *task, const u8 filename[FILENAMESIZE], const u8 argv[ARGBUFSIZE], int return_code) {}
static void print_disk_event(struct task_struct *task, const char *path, int return_code) {}
#endif

#endif // BPF_COMMON_H
