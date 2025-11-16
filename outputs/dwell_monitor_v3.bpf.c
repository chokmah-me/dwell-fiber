// SPDX-License-Identifier: GPL-2.0
/* Dwell-Fiber V3.0 eBPF Program - Weighted I/O Pressure Monitor */

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/types.h>

#define MAX_ENTRIES 10240
#define WINDOW_NS 1000000000ULL  // 1 second window

// V3.0: Track TBW (Total Bytes Written) and UFM (Unique Files Modified)
struct wip_event {
    __u32 pid;
    __u64 tbw;           // Total Bytes Written in window
    __u64 ufm;           // Unique Files Modified in window
    __u64 timestamp_ns;
    char comm[16];
};

struct wip_state {
    __u64 window_start_ns;
    __u64 tbw_current;
    __u64 ufm_current;
    __u64 inodes_seen[256];  // Bitmap for tracking unique inodes
    __u32 inode_count;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);  // PID
    __type(value, struct wip_state);
} wip_tracker SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

// Track last activity per PID for cleanup
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10240);
    __type(key, __u32);  // pid
    __type(value, __u64);  // last timestamp
} pid_activity SEC(".maps");

static __always_inline bool is_inode_seen(struct wip_state *state, __u64 inode) {
    __u32 idx = (inode >> 6) & 0xFF;  // Use upper bits for index
    __u64 bit = 1ULL << (inode & 0x3F);
    return (state->inodes_seen[idx] & bit) != 0;
}

static __always_inline void mark_inode_seen(struct wip_state *state, __u64 inode) {
    __u32 idx = (inode >> 6) & 0xFF;
    __u64 bit = 1ULL << (inode & 0x3F);
    if (!(state->inodes_seen[idx] & bit)) {
        state->inodes_seen[idx] |= bit;
        state->inode_count++;
    }
}

SEC("kprobe/vfs_write")
int track_vfs_write(struct pt_regs *ctx) {
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u32 pid = pid_tgid >> 32;
    __u64 now = bpf_ktime_get_ns();

    // Update activity timestamp
    bpf_map_update_elem(&pid_activity, &pid, &now, BPF_ANY);

    // Get or create WIP state for this PID
    struct wip_state *state = bpf_map_lookup_elem(&wip_tracker, &pid);
    struct wip_state new_state = {0};

    if (!state) {
        new_state.window_start_ns = now;
        bpf_map_update_elem(&wip_tracker, &pid, &new_state, BPF_ANY);
        state = bpf_map_lookup_elem(&wip_tracker, &pid);
        if (!state) return 0;
    }

    // Check if we need to emit event (window expired)
    if (now - state->window_start_ns >= WINDOW_NS) {
        // Emit WIP event
        struct wip_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
        if (event) {
            event->pid = pid;
            event->tbw = state->tbw_current;
            event->ufm = state->inode_count;
            event->timestamp_ns = now;
            bpf_get_current_comm(&event->comm, sizeof(event->comm));
            bpf_ringbuf_submit(event, 0);
        }

        // Reset window
        __builtin_memset(&new_state, 0, sizeof(new_state));
        new_state.window_start_ns = now;
        bpf_map_update_elem(&wip_tracker, &pid, &new_state, BPF_ANY);
        state = bpf_map_lookup_elem(&wip_tracker, &pid);
        if (!state) return 0;
    }

    // Extract write size and inode from context
    // Note: This is simplified - actual implementation needs proper struct file access
    struct file *filp = (struct file *)PT_REGS_PARM1(ctx);
    size_t count = (size_t)PT_REGS_PARM3(ctx);

    // Update TBW
    state->tbw_current += count;

    // Get inode (simplified - needs proper d_inode access)
    __u64 inode = 0;  // Would extract from filp->f_inode->i_ino
    if (inode && !is_inode_seen(state, inode)) {
        mark_inode_seen(state, inode);
    }

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
