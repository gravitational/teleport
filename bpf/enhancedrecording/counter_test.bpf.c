#include "../vmlinux.h"
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "../helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

BPF_COUNTER(test_counter);

SEC("tp/syscalls/sys_close")
int tracepoint__syscalls__sys_enter_close(struct syscall_trace_enter *tp)
{
    int fd = (int)tp->args[0];

	// Special bad FD we trigger upon
	if (fd == 1234) {
		INCR_COUNTER(test_counter);
	}

	return 0;
}

SEC("tp/syscalls/sys_exit_close")
int tracepoint__syscalls__sys_exit_close(struct syscall_trace_exit *tp)
{
	return 0;
}
