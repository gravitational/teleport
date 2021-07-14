/*
Copyright 2021 Gravitational, Inc.

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

#include "../vmlinux.h"
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "../helpers.h"

char LICENSE[] SEC("license") = "Dual BSD/GPL";

BPF_COUNTER(test_counter);

SEC("tp/syscalls/sys_close")
int tracepoint__syscalls__sys_enter_close(struct trace_event_raw_sys_enter *tp)
{
    int fd = (int)tp->args[0];

	// Special bad FD we trigger upon
	if (fd == 1234) {
		INCR_COUNTER(test_counter);
	}

	return 0;
}

SEC("tp/syscalls/sys_exit_close")
int tracepoint__syscalls__sys_exit_close(struct trace_event_raw_sys_exit *tp)
{
	return 0;
}
