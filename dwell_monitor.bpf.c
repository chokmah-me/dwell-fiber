// SPDX-License-Identifier: GPL-2.0
/* Dwell-Fiber eBPF Program - File Dwell Time Monitor
 *
 * v1.5.0: FD-tracking fix.
 *
 * Bug in v1.4.x: both handle_openat_enter and handle_close_enter used fd=0
 * as the dwell_tracker key, so a process opening N files concurrently lost
 * N-1 of them and couldn't correlate close events to the right open.
 *
 * Fix: stash open timestamps in a pending_opens map keyed by pid_tgid (a
 * thread can only have one openat in flight at a time), then promote the
 * entry to dwell_tracker keyed by (pid, real_fd) on sys_exit_openat using
 * the syscall return value. close handlers read fd from the tracepoint
 * context and look it up by (pid, fd).
 */

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/types.h>

#define MAX_ENTRIES 10240
#define MAX_FILENAME 256

struct dwell_event {
    __u32 pid;
    __u32 tid;
    __u64 inode;
    __u64 duration_ns;
    __u64 timestamp;
    char filename[MAX_FILENAME];
    char comm[16];
};

struct dwell_key {
    __u32 pid;
    __u32 fd;
};

struct dwell_value {
    __u64 open_time;
    __u64 inode;
    __u32 access_count;
};

/* Pending open: indexed by pid_tgid because the fd is not known until
 * sys_exit_openat. A single thread cannot have two openats in flight,
 * so this is a safe key. */
struct pending_open_value {
    __u64 open_time;
    __u32 tgid;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, struct dwell_key);
    __type(value, struct dwell_value);
} dwell_tracker SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u64);   /* pid_tgid */
    __type(value, struct pending_open_value);
} pending_opens SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);   /* pid */
    __type(value, __u64); /* last timestamp */
} pid_activity SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

/* Tracepoint context shape for sys_exit_*: 'ret' carries the syscall return value. */
struct trace_event_raw_sys_exit {
    unsigned long long unused;
    long id;
    long ret;
};

SEC("tracepoint/syscalls/sys_enter_openat")
int handle_openat_enter(void *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();

    bpf_map_update_elem(&pid_activity, &pid, &now, BPF_ANY);

    struct pending_open_value pending = {
        .open_time = now,
        .tgid = (__u32)(pid_tgid & 0xFFFFFFFF),
    };
    bpf_map_update_elem(&pending_opens, &pid_tgid, &pending, BPF_ANY);
    return 0;
}

SEC("tracepoint/syscalls/sys_exit_openat")
int handle_openat_exit(struct trace_event_raw_sys_exit *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    long ret = ctx->ret;

    struct pending_open_value *pending = bpf_map_lookup_elem(&pending_opens, &pid_tgid);
    if (!pending) {
        return 0;
    }

    /* Failed openat: discard pending state, do not create a tracker entry. */
    if (ret < 0) {
        bpf_map_delete_elem(&pending_opens, &pid_tgid);
        return 0;
    }

    struct dwell_key key = {
        .pid = pid,
        .fd  = (__u32)ret,
    };
    struct dwell_value value = {
        .open_time    = pending->open_time,
        .inode        = 0,
        .access_count = 1,
    };
    bpf_map_update_elem(&dwell_tracker, &key, &value, BPF_ANY);
    bpf_map_delete_elem(&pending_opens, &pid_tgid);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_close")
int handle_close_enter(void *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();

    bpf_map_update_elem(&pid_activity, &pid, &now, BPF_ANY);

    /* sys_enter_close tracepoint format:
     *   field:int __syscall_nr;  offset:8
     *   field:unsigned int fd;   offset:16
     */
    __u32 fd = 0;
    bpf_probe_read_kernel(&fd, sizeof(fd), (char *)ctx + 16);

    struct dwell_key key = { .pid = pid, .fd = fd };

    struct dwell_value *value = bpf_map_lookup_elem(&dwell_tracker, &key);
    if (!value) {
        return 0;
    }

    __u64 duration = now - value->open_time;
    if (duration < 100000000) {  /* 100ms noise filter */
        bpf_map_delete_elem(&dwell_tracker, &key);
        return 0;
    }

    struct dwell_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) {
        return 0;
    }

    event->pid         = pid;
    event->tid         = (__u32)pid_tgid;
    event->inode       = value->inode;
    event->duration_ns = duration;
    event->timestamp   = now;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&dwell_tracker, &key);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
