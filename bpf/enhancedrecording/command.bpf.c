#include "../vmlinux.h"
#include <bpf/bpf_helpers.h>       /* most used helpers: SEC, __always_inline, etc */
#include <bpf/bpf_core_read.h>     /* for BPF CO-RE helpers */
#include <bpf/bpf_tracing.h>       /* for getting kprobe arguments */

#include "./common.h"

// Size, in bytes, of the ring buffer used to report
// audit events to userspace. This is the default,
// the userspace can adjust this value based on config.
//
// Each event is 21012 bytes, so this default buffer size
// will fit just under 400 events at once which should be
// a safe default.
#define EVENTS_BUF_SIZE (4096 * 2048)

char LICENSE[] SEC("license") = "Dual BSD/GPL";

struct data_t {
    // pid as in the userspace term (i.e. task->tgid in kernel).
    u64 pid;
    // ppid is the userspace term (i.e task->real_parent->tgid in kernel).
    u64 ppid;
    // Command is the executable.
    u8 command[TASK_COMM_LEN];
    // Filename is the path of the executable.
    u8 filename[FILENAMESIZE];
    // Args is the list of arguments to the program.
    u8 args[ARGBUFSIZE];
    // ArgsLen is the length of the args.
    u32 args_len;
    // ArgsTruncated is true if the args were truncated.
    bool args_truncated;
    // CgroupID is the internal cgroupv2 ID of the event.
    u64 cgroup;
    // AuditSessionID is the audit session ID that is used to correlate
    // events with specific sessions.
    u32 audit_session_id;
    // Return_code is the return code of execve.
    // 0 on success, negative errno on failure.
    int return_code;
};

// Force emitting struct data_t into the ELF. bpf2go needs this
// to generate the Go bindings.
const struct data_t *unused __attribute__((unused));

// Task-local storage for best-effort filename and argv captured at 
// sys_enter_execve. On a successful exec, fexit/bprm_execve overwrites
// this with reliable data from mm->arg_start. On a failed exec, this 
// is the only argv source.
struct inflight_exec_t {
    bool valid;
    u8 filename[FILENAMESIZE];
    u64 argv;
    bool emitted;
};

struct {
    __uint(type, BPF_MAP_TYPE_TASK_STORAGE);
    __uint(map_flags, BPF_F_NO_PREALLOC);
    __type(key, int);
    __type(value, struct inflight_exec_t);
} inflight_exec SEC(".maps");

// Hashmap that keeps all audit session IDs that should be monitored 
// by Teleport.
BPF_HASH(monitored_sessionids, u32, u8, MAX_MONITORED_SESSIONS);

BPF_RING_BUF(execve_events, EVENTS_BUF_SIZE);

BPF_COUNTER(lost);

// Read the filename and argv from userspace and stash them in task-local
// storage for exit_execve to use if necessary. The data from userspace
// may not be paged into memory yet so it's not reliable and why getting
// the data in fexit/bprm_execve is preferred; when its called the data
// is guaranteed to be paged in and accessible to us.
static int enter_execve(const char *filename, const char *const *argv)
{
    struct inflight_exec_t *info = NULL;
    struct task_struct *task = bpf_get_current_task_btf();
    u32 session_id = task->sessionid;
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    info = bpf_task_storage_get(&inflight_exec, task, NULL, BPF_LOCAL_STORAGE_GET_F_CREATE);
    if (!info) {
        return 0;
    }

    info->valid = true;
    if (bpf_probe_read_user_str(&info->filename, sizeof(info->filename), filename) <= 0) {
        // if reading the filename failed set it to empty to it won't
        // be used in an event
        info->filename[0] = 0;
    }
    info->argv = (u64)argv;
    info->emitted = false;

    return 0;
}

SEC("tp/syscalls/sys_enter_execve")
int tracepoint__syscalls__sys_enter_execve(struct syscall_trace_enter *tp)
{
    const char *filename = (const char *)tp->args[0];
    const char *const *argv = (const char *const *)tp->args[1];

    return enter_execve(filename, argv);
}

SEC("tp/syscalls/sys_enter_execveat")
int tracepoint__syscalls__sys_enter_execveat(struct syscall_trace_enter *tp)
{
    // execveat has a directory file descriptor as the zeroth argument,
    // and all other arguments from execve follow it, so we need to
    // start from the first argument.
    const char *filename = (const char *)tp->args[1];
    const char *const *argv = (const char *const *)tp->args[2];

    return enter_execve(filename, argv);
}

// fexit/bprm_execve is hit for both execve and execveat most of the
// time, but an exec can fail before bprm_execve is called, hence the
// need for exit_execve as well.
SEC("fexit/bprm_execve")
int BPF_PROG(bprm_execve_exit, struct linux_binprm *bprm, int ret)
{
    struct inflight_exec_t *info = NULL;
    struct task_struct *task = bpf_get_current_task_btf();
    u32 session_id = task->sessionid;
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    info = bpf_task_storage_get(&inflight_exec, task, NULL, 0);
    if (ret != 0) {
        if (info && info->valid) {
            // If bprm_execve is called but the return code is non-zero,
            // the argv in task->mm won't be populated, but we can still
            // get the filename. In this case we pass the filename to
            // exit_execve and let it emit the event with the best-effort
            // argv.
            bpf_printk("bprm_execve_exit: failed exec");
            bpf_probe_read_kernel_str(info->filename, sizeof(info->filename), BPF_CORE_READ(bprm, filename));
        }
        return 0;
    }

    struct data_t *data = bpf_ringbuf_reserve(&execve_events, sizeof(*data), 0);
    if (!data) {
        INCR_COUNTER(lost);
        bpf_printk("execve_events ring buffer full");
        return 0;
    }

    void *arg_start = (void *)BPF_CORE_READ(task, mm, arg_start);
    void *arg_end = (void *)BPF_CORE_READ(task, mm, arg_end);
    // Use a local uint64 here to apease the verifier.
    u64 args_len = arg_end - arg_start;

    data->args_truncated = false;
    if (args_len > ARGBUFSIZE) {
        args_len = ARGBUFSIZE;
        data->args_truncated = true;
    }
    int read_ret = bpf_probe_read_user(&data->args, args_len, arg_start);
    if (read_ret < 0) {
        args_len = 0;
    }
    data->args_len = args_len;

    bpf_probe_read_kernel_str(&data->filename, sizeof(data->filename), BPF_CORE_READ(bprm, filename));
    data->return_code = ret;

    print_command_event(task, data->filename, data->args, data->return_code);

    data->pid = bpf_get_current_pid_tgid() >> 32;
    data->cgroup = bpf_get_current_cgroup_id();
    data->audit_session_id = session_id;

    data->ppid = BPF_CORE_READ(task, real_parent, tgid);
    bpf_get_current_comm(&data->command, sizeof(data->command));

    bpf_printk("bprm_execve_exit: emitted event");
    bpf_ringbuf_submit(data, 0);

    if (info && info->valid) {
        info->emitted = true;
    }

    return 0;
}

// exit_execve is always called, but we only emit an event here if
// bprm_execve_exit wasn't called or wasn't able to emit an event.
static int exit_execve(int retCode)
{
    struct task_struct *task = bpf_get_current_task_btf();
    u32 session_id = task->sessionid;
    u8 *is_monitored = bpf_map_lookup_elem(&monitored_sessionids, &session_id);
    if (is_monitored == NULL) {
        return 0;
    }

    struct inflight_exec_t *info = bpf_task_storage_get(&inflight_exec, task, NULL, 0);
    if (!info) {
        bpf_printk("exit_execve: no inflight_exec_t found, not emitting event");
        return 0;
    }
    if (!info->valid || info->emitted) {
        bpf_printk("execve_exit: not emitting event: valid=%d emitted=%d", info->valid, info->emitted);
        goto out;
    }

    struct data_t *data = bpf_ringbuf_reserve(&execve_events, sizeof(*data), 0);
    if (!data) {
        INCR_COUNTER(lost);
        bpf_printk("execve_events ring buffer full");
        goto out;
    }
    data->args_truncated = false;

    u32 offset = 0;
    const char *const *argv = (const char *const *)info->argv;

    int i = 0;
    for (; i < MAXARGS; i++) {
        const char *argp = NULL;
        long ret = bpf_probe_read_user(&argp, sizeof(argp), &argv[i]);
        if (ret < 0 || !argp) {
            break;
        }

        // Check the offset before we read to appease the eBPF verifier.
        if (offset >= ARGBUFSIZE - MAXARGLEN) {
            data->args_truncated = true;
            break;
        }

        ret = bpf_probe_read_user_str(&data->args[offset], MAXARGLEN, argp);
        if (ret < 0) {
            break;
        }

        offset += ret;
    }
    data->args_len = offset;

    // Set the args as truncated if we weren't able to read all of them
    if (i == MAXARGS) {
        const char *argp = NULL;
        if (bpf_probe_read_user(&argp, sizeof(argp), &argv[i]) == 0 && argp) {
            data->args_truncated = true;
        }
    }

    bpf_probe_read_kernel_str(&data->filename, sizeof(data->filename), info->filename);
    data->return_code = retCode;

    print_command_event(task, data->filename, data->args, data->return_code);

    data->pid = bpf_get_current_pid_tgid() >> 32;
    data->cgroup = bpf_get_current_cgroup_id();
    data->audit_session_id = session_id;

    data->ppid = BPF_CORE_READ(task, real_parent, tgid);
    bpf_get_current_comm(&data->command, sizeof(data->command));

    bpf_printk("execve_exit: emitted event");
    bpf_ringbuf_submit(data, 0);

out:
    // Mark consumed so a subsequent open on the same thread doesn't 
    // pick up stale data.
    info->valid = false;

    return 0;
}

SEC("tp/syscalls/sys_exit_execve")
int tracepoint__syscalls__sys_exit_execve(struct syscall_trace_exit *tp)
{
    return exit_execve(tp->ret);
}

SEC("tp/syscalls/sys_exit_execveat")
int tracepoint__syscalls__sys_exit_execveat(struct syscall_trace_exit *tp)
{
    return exit_execve(tp->ret);
}
