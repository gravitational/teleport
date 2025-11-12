#ifndef BPF_COMMON_H
#define BPF_COMMON_H

// Uncomment to enable debug messages via bpf_printk
// #define PRINT_DEBUG_MSGS

// Maximum monitored sessions.
#define MAX_MONITORED_SESSIONS 1024

#include <bpf/bpf_core_read.h>

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
#else
#define bpf_printk(fmt, ...)
#endif

#endif // BPF_COMMON_H
