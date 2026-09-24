CLANG ?= clang
# Override for a hermetic toolchain, e.g.:
#   make -C bpf CLANG=$HOME/workspace/.tooling/LLVM-21.1.2-Linux-X64/bin/clang
# Any clang with the BPF backend (>= 12) works; no libbpf-dev/bpftool needed.
ARCH := $(shell uname -m | sed 's/x86_64/x86/')

BPF_CFLAGS := -target bpf -D__TARGET_ARCH_$(ARCH) -g -O2 -Wall
# -g emits BTF so the loader can apply CO-RE field relocations.
# -I/usr/include covers <linux/types.h>; the arch triplet dir covers
# <asm/types.h> (Debian/Ubuntu multiarch layout); kernel-structure access
# otherwise comes from the minimal preserve_access_index declarations in
# the .bpf.c (or an optional generated vmlinux.h, see the vmlinux-h target).
BPF_INCLUDES := -I/usr/include -I/usr/include/$(shell uname -m)-linux-gnu

TARGETS := dwell_monitor.bpf.o
HEADERS := include/bpf_helpers.h include/bpf_core_read.h

.PHONY: all clean vmlinux-h

all: $(TARGETS)

# Optional: generate a full vmlinux.h from the running kernel's BTF.
# Not required for the build: dwell_monitor.bpf.c carries minimal
# preserve_access_index kernel-type declarations and builds hermetically
# without it (no bpftool / libbpf-dev needed). When present next to the
# .bpf.c, vmlinux.h is picked up automatically via __has_include.
vmlinux-h:
	@if command -v bpftool >/dev/null 2>&1 && [ -r /sys/kernel/btf/vmlinux ]; then \
		bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h && \
		echo "✓ Generated vmlinux.h"; \
	else \
		echo "bpftool or /sys/kernel/btf/vmlinux unavailable;" \
		     "using minimal CO-RE types (no vmlinux.h needed)"; \
	fi

%.bpf.o: %.bpf.c $(HEADERS)
	$(CLANG) $(BPF_CFLAGS) $(BPF_INCLUDES) -c $< -o $@
	@echo "✓ Compiled $@"

clean:
	rm -f $(TARGETS)
