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
    __u64 inode;
};

struct dwell_value {
    __u64 open_time;
    __u64 total_dwell;
    __u32 access_count;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, struct dwell_key);
    __type(value, struct dwell_value);
} dwell_tracker SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_openat")
int handle_openat_enter(void *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    
    struct dwell_key key = {
        .pid = pid,
        .inode = 0,
    };
    
    struct dwell_value value = {
        .open_time = bpf_ktime_get_ns(),
        .total_dwell = 0,
        .access_count = 1,
    };
    
    bpf_map_update_elem(&dwell_tracker, &key, &value, BPF_ANY);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_close")
int handle_close_enter(void *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();
    
    struct dwell_key key = {
        .pid = pid,
        .inode = 0,
    };
    
    struct dwell_value *value = bpf_map_lookup_elem(&dwell_tracker, &key);
    if (!value) {
        return 0;
    }
    
    __u64 duration = now - value->open_time;
    
    struct dwell_event *event = bpf_ringbuf_reserve(&events, 
                                                     sizeof(*event), 0);
    if (!event) {
        return 0;
    }
    
    event->pid = pid;
    event->tid = (__u32)pid_tgid;
    event->inode = key.inode;
    event->duration_ns = duration;
    event->timestamp = now;
    bpf_get_current_comm(&event->comm, sizeof(event->comm));
    
    bpf_ringbuf_submit(event, 0);
    bpf_map_delete_elem(&dwell_tracker, &key);
    
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
