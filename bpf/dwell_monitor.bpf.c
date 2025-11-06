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

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, struct dwell_key);
    __type(value, struct dwell_value);
} dwell_tracker SEC(".maps");

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
    
    // Record activity for this PID (used for cleanup)
    bpf_map_update_elem(&pid_activity, &pid, &now, BPF_ANY);
    
    // Note: File descriptor and inode are not yet available at sys_enter_openat
    // They will be available at sys_exit_openat (not yet tracked)
    // For now, track with placeholder FD=0 to avoid data loss
    
    struct dwell_key key = {
        .pid = pid,
        .fd = 0,  // Will be filled at exit handler with real FD
    };
    
    struct dwell_value value = {
        .open_time = now,
        .inode = 0,  // Will be filled when inode becomes available
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
    
    // Update activity tracking
    bpf_map_update_elem(&pid_activity, &pid, &now, BPF_ANY);
    
    // Try to find entry by FD
    // Note: sys_enter_close only has FD number, not inode
    // A full implementation would need to correlate FD→inode via kernel data structures
    
    // For now, iterate to find matching entry (will be one per PID for open ops)
    struct dwell_key key = {
        .pid = pid,
        .fd = 0,  // Match the entry created in handle_openat_enter
    };
    
    struct dwell_value *value = bpf_map_lookup_elem(&dwell_tracker, &key);
    if (!value) {
        // Entry not found - may have already been processed or process crashed
        return 0;
    }
    
    // Filter out noise: only report dwell > 100ms
    __u64 duration = now - value->open_time;
    if (duration < 100000000) {  // 100ms in nanoseconds
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
