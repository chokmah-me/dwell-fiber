// SPDX-License-Identifier: GPL-2.0
/* Dwell-Fiber eBPF Program - File Dwell Time Monitor */

#include "dwell_monitor.bpf.h"
#include <linux/bpf.h>
#include <linux/fs.h>
#include <linux/sched.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

#define WINDOW_NS 1000000000ULL

struct per_pid_stats {
	u64 window_start_ns;
	u64 bytes_written;
	u32 unique_files;
	u64 window_id;
};

struct pid_inode_key {
	u32 pid;
	u64 inode;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 1024);
	__type(key, u32);
	__type(value, struct per_pid_stats);
} pid_stats SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 8192);
	__type(key, struct pid_inode_key);
	__type(value, u64);
} pid_file_seen SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1 << 15);
} io_events SEC(".maps");

static __always_inline void emit_event(u32 pid, struct per_pid_stats *stats, u64 ts) {
	struct io_event *evt = bpf_ringbuf_reserve(&io_events, sizeof(*evt), 0);
	if (!evt)
		return;
	evt->pid = pid;
	evt->bytes_written = stats->bytes_written;
	evt->unique_files = stats->unique_files;
	evt->timestamp_ns = ts;
	bpf_ringbuf_submit(evt, 0);
}

SEC("kprobe/vfs_write")
int trace_vfs_write(struct pt_regs *ctx, struct file *file, const char __user *buf, size_t count, loff_t *pos) {
	u64 ts = bpf_ktime_get_ns();
	u32 pid = bpf_get_current_pid_tgid() >> 32;
	struct per_pid_stats *stats = bpf_map_lookup_elem(&pid_stats, &pid);
	if (!stats) {
		struct per_pid_stats zero = {};
		zero.window_start_ns = ts;
		zero.window_id = 1;
		bpf_map_update_elem(&pid_stats, &pid, &zero, BPF_ANY);
		stats = bpf_map_lookup_elem(&pid_stats, &pid);
		if (!stats)
			return 0;
	}
	if (ts - stats->window_start_ns >= WINDOW_NS) {
		emit_event(pid, stats, ts);
		stats->window_start_ns = ts;
		stats->bytes_written = 0;
		stats->unique_files = 0;
		stats->window_id++;
	}
	stats->bytes_written += count;
	struct pid_inode_key key = {
		.pid = pid,
		.inode = BPF_CORE_READ(file, f_inode, i_ino),
	};
	u64 window_id = stats->window_id;
	u64 *seen = bpf_map_lookup_elem(&pid_file_seen, &key);
	if (!seen || *seen != window_id) {
		stats->unique_files++;
		bpf_map_update_elem(&pid_file_seen, &key, &window_id, BPF_ANY);
	}
	return 0;
}

char LICENSE[] SEC("license") = "GPL";
