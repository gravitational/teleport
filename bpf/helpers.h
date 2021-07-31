#ifndef HELPERS_H
#define HELPERS_H

#define BPF_ARRAY(name, val_type, size) \
    struct { \
        __uint(type, BPF_MAP_TYPE_ARRAY); \
        __uint(max_entries, size); \
        __type(key, u32); \
        __type(value, val_type); \
    } name SEC(".maps")

#include "vmlinux.h"

#define BPF_HASH(name, key_type, val_type, size) \
    struct { \
        __uint(type, BPF_MAP_TYPE_HASH); \
        __uint(max_entries, size); \
        __type(key, key_type); \
        __type(value, val_type); \
    } name SEC(".maps")

#define BPF_LPM_TRIE(name, key_type, val_type, size) \
	struct { \
		__uint(type, BPF_MAP_TYPE_LPM_TRIE); \
		__uint(max_entries, size); \
		__type(key, key_type); \
		__type(value, val_type); \
		__uint(map_flags, BPF_F_NO_PREALLOC); \
	} name SEC(".maps")

#define BPF_RING_BUF(name, size) \
    struct { \
        __uint(type, BPF_MAP_TYPE_RINGBUF); \
        __uint(max_entries, size); \
    } name SEC(".maps")

#define TASK_COMM_LEN 16
#define __user

#define DOORBELL_BUF_SIZE 4096

#define BPF_COUNTER(name) \
    BPF_ARRAY(name##_counter, u64, 1); \
    BPF_RING_BUF(name##_doorbell, DOORBELL_BUF_SIZE);

#define INCR_COUNTER(name) incr_counter(&(name##_counter), &(name##_doorbell))

// Increments counter and rings the doorbell by inserting
// a byte into the ring buffer
static inline void incr_counter(void *counter, void *doorbell)
{
    u32 key = 0;
    u64 *value = bpf_map_lookup_elem(counter, &key);
    if (value)
    {
        u8 ding = 0;
        __sync_fetch_and_add(value, 1);

        // Ring the doorbell by sending a single byte. If bpf_ringbuf_output fails,
        // it does not matter. In that case the ring buffer is full so the consumer
        // is sure to still be woken up.
        bpf_ringbuf_output(doorbell, &ding, sizeof(u8), 0);
    }
}

#endif
