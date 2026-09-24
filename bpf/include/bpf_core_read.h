/* Minimal CO-RE field access.
 *
 * Full libbpf is unavailable on some build hosts, so this header provides the
 * one primitive dwell_monitor.bpf.c needs: a single-level struct field read
 * that the compiler records as a BTF field relocation
 * (__builtin_preserve_access_index). The loader (cilium/ebpf) applies the
 * relocation against the running kernel's BTF, so field offsets track the
 * kernel instead of being baked in at compile time.
 *
 * Requirements: the accessed struct types must be declared with
 * __attribute__((preserve_access_index)) (see the minimal declarations in
 * dwell_monitor.bpf.c, or a generated vmlinux.h), and the object must be
 * compiled with -g so BTF is emitted.
 */
#ifndef __DWELL_BPF_CORE_READ_H
#define __DWELL_BPF_CORE_READ_H

#define CORE_READ(src, field) __builtin_preserve_access_index((src)->field)

#endif /* __DWELL_BPF_CORE_READ_H */
