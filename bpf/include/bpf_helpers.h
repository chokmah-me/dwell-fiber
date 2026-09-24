/* Minimal vendored subset of libbpf's bpf_helpers.h.
 *
 * The full libbpf headers (and bpftool) are not available on every build host,
 * so this file declares exactly the helper functions, map types, and macros
 * that dwell_monitor.bpf.c uses. It is intentionally hermetic: the BPF object
 * builds with plain `clang -target bpf`, no libbpf-dev required.
 *
 * BPF helper IDs and map types follow include/uapi/linux/bpf.h.
 */
#ifndef __DWELL_BPF_HELPERS_H
#define __DWELL_BPF_HELPERS_H

#include <linux/types.h> /* __u8/__u16/__u32/__u64 */

#define SEC(NAME) __attribute__((section(NAME), used))

#define __uint(name, val) int (*name)[val]
#define __type(name, val) typeof(val) *name
#define __array(name, val) typeof(val) *name[]

#ifndef __always_inline
#define __always_inline __attribute__((always_inline))
#endif

/* Subset of enum bpf_func_id used by this program. */
enum {
	BPF_FUNC_unspec = 0,
	BPF_FUNC_map_lookup_elem = 1,
	BPF_FUNC_map_update_elem = 2,
	BPF_FUNC_map_delete_elem = 3,
	BPF_FUNC_ktime_get_ns = 5,
	BPF_FUNC_get_current_pid_tgid = 14,
	BPF_FUNC_get_current_comm = 16,
	BPF_FUNC_probe_read_kernel = 45,
	BPF_FUNC_ringbuf_reserve = 131,
	BPF_FUNC_ringbuf_submit = 132,
};

/* Subset of enum bpf_map_type used by this program. */
enum {
	BPF_MAP_TYPE_UNSPEC = 0,
	BPF_MAP_TYPE_HASH = 1,
	BPF_MAP_TYPE_PERCPU_ARRAY = 6,
	BPF_MAP_TYPE_LRU_HASH = 9,
	BPF_MAP_TYPE_RINGBUF = 27,
};

/* Map update flags. */
enum {
	BPF_ANY = 0,
};

static void *(*bpf_map_lookup_elem)(void *map, const void *key) =
	(void *)BPF_FUNC_map_lookup_elem;
static long (*bpf_map_update_elem)(void *map, const void *key,
				   const void *value, __u64 flags) =
	(void *)BPF_FUNC_map_update_elem;
static long (*bpf_map_delete_elem)(void *map, const void *key) =
	(void *)BPF_FUNC_map_delete_elem;
static __u64 (*bpf_ktime_get_ns)(void) =
	(void *)BPF_FUNC_ktime_get_ns;
static __u64 (*bpf_get_current_pid_tgid)(void) =
	(void *)BPF_FUNC_get_current_pid_tgid;
static long (*bpf_get_current_comm)(void *buf, __u32 size_of_buf) =
	(void *)BPF_FUNC_get_current_comm;
static long (*bpf_probe_read_kernel)(void *dst, __u32 size,
				     const void *unsafe_ptr) =
	(void *)BPF_FUNC_probe_read_kernel;
static void *(*bpf_ringbuf_reserve)(void *ringbuf, __u64 size,
				     __u64 flags) =
	(void *)BPF_FUNC_ringbuf_reserve;
static void (*bpf_ringbuf_submit)(void *data, __u64 flags) =
	(void *)BPF_FUNC_ringbuf_submit;

/* x86_64 struct pt_regs (frozen arch ABI, not CO-RE relocated) for kprobe
 * argument access. Register order matches arch/x86/include/asm/ptrace.h.
 * Skipped when a full vmlinux.h is in use (it defines pt_regs itself). */
#ifndef DWELL_HAVE_VMLINUX_H
struct pt_regs {
	__u64 r15, r14, r13, r12, rbp, rbx, r11, r10, r9, r8;
	__u64 rax, rcx, rdx, rsi, rdi, orig_rax, rip, cs, eflags, rsp, ss;
};

#define PT_REGS_PARM1(r) ((r)->rdi)
#define PT_REGS_PARM2(r) ((r)->rsi)
#endif

#endif /* __DWELL_BPF_HELPERS_H */
