// SPDX-License-Identifier: GPL-2.0
/* Dwell-Fiber eBPF Program - File Dwell Time Monitor
 *
 * CO-RE build: kernel-structure access (struct file / struct inode) uses
 * CORE_READ field relocations applied by the loader against the running
 * kernel's BTF, so no kernel-version-specific headers are needed. Syscall
 * tracepoint arguments are read through explicit context structs matching the
 * stable syscall tracepoint ABI (no raw ctx+offset reads). The kprobe program
 * reads its argument from x86_64 pt_regs (frozen arch ABI).
 *
 * If a full vmlinux.h is present next to this file (optional `make vmlinux-h`),
 * it is used for the kernel types; otherwise the minimal
 * preserve_access_index declarations below are used. Either way the object is
 * compiled with -g so BTF (and the CO-RE relocations) are emitted.
 */

/* vmlinux.h first when present (optional `make vmlinux-h`): the vendored
 * helpers below skip the definitions it already provides. */
#if __has_include("vmlinux.h")
#include "vmlinux.h"
#define DWELL_HAVE_VMLINUX_H 1
#endif

#include "include/bpf_helpers.h"
#include "include/bpf_core_read.h"

#ifndef DWELL_HAVE_VMLINUX_H
/* Minimal CO-RE kernel type declarations: only what this program touches.
 * preserve_access_index makes CORE_READ accesses record BTF relocations. */
struct inode {
	unsigned long i_ino;
} __attribute__((preserve_access_index));

struct file {
	struct inode *f_inode;
} __attribute__((preserve_access_index));
#endif

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

/* Pending inode number for the in-flight open, keyed by pid_tgid (same
 * single-in-flight-per-thread assumption as pending_opens). Filled by the
 * security_file_open kprobe; consumed on sys_exit_openat. */
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, MAX_ENTRIES);
	__type(key, __u64);   /* pid_tgid */
	__type(value, __u64); /* inode number */
} pending_inodes SEC(".maps");

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

/* Kernel-side session counters, read by userspace for /metrics. These count
 * EVERY close that matches a tracked open -- including sub-100ms dwells that
 * never reach the ring buffer -- so observers can distinguish "saw thousands
 * of fast sessions and filtered them" (fast intermittent encryption) from "saw
 * nothing" (dead pipeline). The userspace counters alone cannot: they only see
 * events that already survived the 100ms in-kernel filter below. */
#define STAT_SESSIONS_TOTAL    0  /* every matched close, pre-filter */
#define STAT_SESSIONS_FILTERED 1  /* subset dropped by the <100ms filter */

struct {
	__uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
	__uint(max_entries, 2);
	__type(key, __u32);
	__type(value, __u64);
} stats SEC(".maps");

static __always_inline void stat_inc(__u32 idx) {
	__u64 *c = bpf_map_lookup_elem(&stats, &idx);
	if (c) {
		(*c)++;
	}
}

/* ---- V3: Weighted I/O Pressure (WIP) tracking ----
 * Rate-based signal that catches fast intermittent encryption, which V2's
 * dwell-latency tracking filters out. We accumulate, per PID, the bytes written
 * (TBW) and the distinct inodes opened (true unique-inode UFM, via the
 * security_file_open kprobe + ufm_inodes map below); userspace polls these maps
 * every ~1s, divides by the elapsed window, and resets. The ufm_accum opens
 * counter is kept as a fallback for kernels where the kprobe cannot attach. */
struct wip_state {
	__u64 window_start_ns; /* set on first activity in a window */
	__u64 tbw_accum;       /* bytes written this window */
	__u64 ufm_accum;       /* opens this window (fallback UFM proxy) */
	char comm[16];         /* process name at window create (first-open wins) */
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, MAX_ENTRIES);
	__type(key, __u32);    /* pid */
	__type(value, struct wip_state);
} wip_tracker SEC(".maps");

/* Distinct (pid, inode) pairs opened in the current window. Userspace counts
 * keys per PID at each poll for the true unique-inode UFM signal, then deletes
 * the keys it read; LRU eviction bounds the map if userspace stops polling. */
struct wip_ino_key {
	__u32 pid;
	__u32 __pad;
	__u64 ino;
};

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 65536);
	__type(key, struct wip_ino_key);
	__type(value, __u8);
} ufm_inodes SEC(".maps");

/* Get-or-create the per-PID WIP window, stamping window_start_ns and comm on
 * creation. Existing entries keep their stored comm (first-open wins). */
static __always_inline struct wip_state *wip_get(__u32 pid, __u64 now) {
	struct wip_state *st = bpf_map_lookup_elem(&wip_tracker, &pid);
	if (st) {
		return st;
	}
	struct wip_state fresh = {};
	fresh.window_start_ns = now;
	/* Capture comm at syscall time so userspace tiering works even when
	 * /proc/<pid>/comm is missing (e.g. WSL PID skew). */
	bpf_get_current_comm(&fresh.comm, sizeof(fresh.comm));
	bpf_map_update_elem(&wip_tracker, &pid, &fresh, BPF_ANY);
	return bpf_map_lookup_elem(&wip_tracker, &pid);
}

/* Syscall tracepoint context structs (stable ABI). Field layout matches
 * /sys/kernel/tracing/events/syscalls/sys_enter_<name>/format:
 *   field:int __syscall_nr;  offset 8
 *   field:unsigned long args[6]; offset 16
 * and for sys_exit: field:long ret; offset 16. */
struct trace_event_raw_sys_enter {
	unsigned long long unused;
	long id;
	unsigned long args[6];
};

struct trace_event_raw_sys_exit {
	unsigned long long unused;
	long id;
	long ret;
};

/* True unique-inode capture. security_file_open(struct file *file) is called
 * from do_dentry_open() AFTER f_inode is assigned, so the inode is valid here.
 * (A kprobe on vfs_open entry would be too early: f_inode is only populated by
 * do_dentry_open.) The inode is stashed by pid_tgid and picked up on
 * sys_exit_openat; failures never reach the stash consumer, which drops it. */
SEC("kprobe/security_file_open")
int handle_file_open(struct pt_regs *ctx) {
	struct file *file = (struct file *)PT_REGS_PARM1(ctx);
	if (!file) {
		return 0;
	}
	struct inode *ip = CORE_READ(file, f_inode);
	if (!ip) {
		return 0;
	}
	__u64 ino = CORE_READ(ip, i_ino);
	if (!ino) {
		return 0;
	}
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	bpf_map_update_elem(&pending_inodes, &pid_tgid, &ino, BPF_ANY);
	return 0;
}

SEC("tracepoint/syscalls/sys_enter_openat")
int handle_openat_enter(struct trace_event_raw_sys_enter *ctx) {
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	__u32 pid = pid_tgid >> 32;
	__u64 now = bpf_ktime_get_ns();

	(void)ctx;

	bpf_map_update_elem(&pid_activity, &pid, &now, BPF_ANY);

	/* V3: count this open as a files-modified event for the WIP window.
	 * This is the fallback UFM proxy; the authoritative unique-inode count
	 * comes from ufm_inodes (populated on sys_exit_openat). */
	struct wip_state *wst = wip_get(pid, now);
	if (wst) {
		wst->ufm_accum++;
	}

	/* The fd is the syscall return value, not available until sys_exit_openat.
	 * Stash the open timestamp keyed by pid_tgid; promote to (pid, fd) on exit. */
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

	/* Resolve the inode stashed by the file-open kprobe; drop the stash
	 * entry either way so failed opens don't leak it. */
	__u64 ino = 0;
	__u64 *pino = bpf_map_lookup_elem(&pending_inodes, &pid_tgid);
	if (pino) {
		ino = *pino;
		bpf_map_delete_elem(&pending_inodes, &pid_tgid);
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
		.inode = ino,
		.access_count = 1,
	};

	bpf_map_update_elem(&dwell_tracker, &key, &value, BPF_ANY);
	bpf_map_delete_elem(&pending_opens, &pid_tgid);

	/* V3: record (pid, inode) for the true unique-inode UFM signal. */
	if (ino) {
		struct wip_ino_key ikey = {
			.pid = pid,
			.ino = ino,
		};
		__u8 one = 1;
		bpf_map_update_elem(&ufm_inodes, &ikey, &one, BPF_ANY);
	}
	return 0;
}

SEC("tracepoint/syscalls/sys_enter_close")
int handle_close_enter(struct trace_event_raw_sys_enter *ctx) {
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	__u32 pid = pid_tgid >> 32;
	__u64 now = bpf_ktime_get_ns();

	bpf_map_update_elem(&pid_activity, &pid, &now, BPF_ANY);

	/* sys_enter_close passes fd as its first argument (args[0]). */
	__u32 fd = (__u32)ctx->args[0];

	struct dwell_key key = {
		.pid = pid,
		.fd = fd,
	};

	struct dwell_value *value = bpf_map_lookup_elem(&dwell_tracker, &key);
	if (!value) {
		return 0;
	}

	/* Count the session before any filtering: this is the pre-filter total
	 * that makes fast-intermittent workloads visible in /metrics. */
	stat_inc(STAT_SESSIONS_TOTAL);

	__u64 duration = now - value->open_time;
	if (duration < 100000000) {  /* 100ms */
		stat_inc(STAT_SESSIONS_FILTERED);
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

/* V3: accumulate bytes written per PID for the TBW (total-bytes-written) signal.
 * sys_enter_write passes the byte count as its third argument (args[2]). */
SEC("tracepoint/syscalls/sys_enter_write")
int handle_write_enter(struct trace_event_raw_sys_enter *ctx) {
	__u64 pid_tgid = bpf_get_current_pid_tgid();
	__u32 pid = pid_tgid >> 32;

	__u64 count = ctx->args[2];

	/* In-kernel filtering: this tracepoint fires on EVERY write syscall
	 * system-wide (sockets, pipes, stdout, ...), so an unconditional
	 * lookup-or-create here is what made --use-v3-wip ~4x slower. Two cheap
	 * filters keep enforcement-mode overhead bounded:
	 *   1. Skip sub-page writes -- ransomware/bulk I/O writes in >=page chunks;
	 *      tiny logging/control writes are noise for a TBW rate signal.
	 *   2. Lookup only (no create): a PID accrues TBW only once it has a window,
	 *      and windows are created exclusively by the openat hook. A process that
	 *      never opens a regular file (pure socket/pipe writer) never allocates
	 *      one, so the hot path is a single failed map lookup. */
	if (count < 4096) {
		return 0;
	}

	__u32 key = pid;
	struct wip_state *wst = bpf_map_lookup_elem(&wip_tracker, &key);
	if (wst) {
		wst->tbw_accum += count;
	}
	return 0;
}

char LICENSE[] SEC("license") = "GPL";
