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

/* Pointer-chasing flavor for base pointers the verifier only knows as
 * scalars -- notably pointers derived from kprobe pt_regs. Values loaded out
 * of the program context are typed u64, so a direct (src)->field dereference
 * is rejected ("R1 invalid mem access 'scalar'"). Routing the access through
 * bpf_probe_read_kernel fixes that: the helper accepts a scalar address
 * operand. The field address still goes through
 * __builtin_preserve_access_index, so the loader relocates the offset
 * against the running kernel's BTF -- CO-RE is preserved, only the access
 * mechanism changes. Rule of thumb: plain CORE_READ for verifier-tracked
 * pointers (map values); CORE_READ_PROBE when the base pointer came from
 * the context or any other scalar-typed value. */
#define CORE_READ_PROBE(src, field) ({                                        \
    __typeof__((src)->field) ___v = (__typeof__((src)->field))0;              \
    bpf_probe_read_kernel(&___v, sizeof(___v),                                \
        (const void *)__builtin_preserve_access_index(&(src)->field));        \
    ___v;                                                                     \
})

#endif /* __DWELL_BPF_CORE_READ_H */
