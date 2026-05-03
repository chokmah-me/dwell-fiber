// SPDX-License-Identifier: GPL-2.0
/* Dwell-Fiber eBPF Program - File Dwell Time Monitor */

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

/* Pending open: keyed by (pid, tgid) so concurrent opens in the same
 * thread are serialized by the kernel (a thread cannot have two openats
 * in flight). We move the entry to the real (pid, fd) key on sys_exit_openat
 * once the fd is known. */
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

// Track last activity per PID for cleanup
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);  // pid
    __type(value, __u64);  // last timestamp
} pid_activity SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_openat")
int handle_openat_enter(void *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();

    bpf_map_update_elem(&pid_activity, &pid, &now, BPF_ANY);

    /* The fd is the syscall return value, not available until sys_exit_openat.
     * Stash the open timestamp keyed by pid_tgid; promote to (pid, fd) on exit. */
    struct pending_open_value pending = {
        .open_time = now,
        .tgid = (__u32)(pid_tgid & 0xFFFFFFFF),
    };
    bpf_map_update_elem(&pending_opens, &pid_tgid, &pending, BPF_ANY);
    return 0;
}

/* Tracepoint format for sys_exit_openat exposes the return value (fd or -errno)
 * at a fixed offset. Using the BPF tracepoint context struct keeps this CO-RE-friendly. */
struct trace_event_raw_sys_exit {
    unsigned long long unused;
    long id;
    long ret;
};

SEC("tracepoint/syscalls/sys_exit_openat")
int handle_openat_exit(struct trace_event_raw_sys_exit *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    long ret = ctx->ret;

    struct pending_open_value *pending = bpf_map_lookup_elem(&pending_opens, &pid_tgid);
    if (!pending) {
        return 0;
    }

    /* Always remove the pending entry; on failure we don't promote it. */
    if (ret < 0) {
        bpf_map_delete_elem(&pending_opens, &pid_tgid);
        return 0;
    }

    struct dwell_key key = {
        .pid = pid,
        .fd = (__u32)ret,
    };

    struct dwell_value value = {
        .open_time = pending->open_time,
        .inode = 0,
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

    /* sys_enter_close passes fd as its first argument. Read it from the
     * tracepoint context. The format is:
     *   field:int __syscall_nr; offset 8
     *   field:unsigned int fd;  offset 16
     */
    __u32 fd = 0;
    bpf_probe_read_kernel(&fd, sizeof(fd), (char *)ctx + 16);

    struct dwell_key key = {
        .pid = pid,
        .fd = fd,
    };

    struct dwell_value *value = bpf_map_lookup_elem(&dwell_tracker, &key);
    if (!value) {
        return 0;
    }

    __u64 duration = now - value->open_time;
    if (duration < 100000000) {  /* 100ms */
        bpf_map_delete_elem(&dwell_tracker, &key);
        return 0;
    }

    struct dwell_event *event = bpf_ringbuf_reserve(&events,
                                                     sizeof(*event), 0);
    if (!event) {
        return 0;
    }

    event->pid = pid;
    event->tid = (__u32)pid_tgid;
    event->inode = value->inode;
    event->duration_ns = duration;
    event->timestamp = now;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));

    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&dwell_tracker, &key);

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
