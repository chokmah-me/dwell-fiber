#!/bin/bash
# dwell-fiber WSL environment diagnostic
# Collects everything needed to debug BPF/tracepoint/cgroup issues on WSL2.
# Run: bash test/wsl_env_diag.sh > /tmp/wsl_diag.txt 2>&1
set -u

echo "=== WSL ENV DIAGNOSTIC ==="
echo "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

echo "--- Kernel ---"
uname -a
echo ""

echo "--- WSL version ---"
wsl.exe --version 2>/dev/null || echo "(wsl.exe not available from inside WSL)"
echo ""

echo "--- PID namespace ---"
echo "Own PID ns: $(readlink /proc/self/ns/pid 2>/dev/null)"
echo "Init PID ns: $(sudo readlink /proc/1/ns/pid 2>/dev/null || echo 'sudo needed')"
echo ""

echo "--- bpf(2) syscall test ---"
python3 -c "
import ctypes, os
libc = ctypes.CDLL('libc.so.6', use_errno=True)
# BPF_MAP_CREATE with BPF_MAP_TYPE_HASH, key=4, value=8, max_entries=1
class BpfAttr(ctypes.Structure):
    _fields_ = [('map_type', ctypes.c_uint), ('key_size', ctypes.c_uint),
                ('value_size', ctypes.c_uint), ('max_entries', ctypes.c_uint)]
attr = BpfAttr(1, 4, 8, 1)
fd = libc.syscall(321, 0, ctypes.byref(attr), ctypes.sizeof(attr))
err = ctypes.get_errno()
print(f'bpf(BPF_MAP_CREATE) -> fd={fd} errno={err} ({os.strerror(err) if err else \"OK\"})')
" 2>&1
echo ""

echo "--- Seccomp ---"
grep -i seccomp /proc/self/status 2>/dev/null || echo "no seccomp info"
echo ""

echo "--- Tracepoint availability ---"
for tp in sys_enter_openat sys_enter_openat2 sys_enter_write sys_exit_openat sys_enter_close; do
    if [ -d "/sys/kernel/tracing/events/syscalls/$tp" ]; then
        id=$(cat /sys/kernel/tracing/events/syscalls/$tp/id 2>/dev/null || echo "?")
        echo "  $tp: present (id=$id)"
    else
        echo "  $tp: MISSING"
    fi
done
echo ""

echo "--- Tracepoint enable status ---"
for tp in sys_enter_openat sys_enter_write; do
    f="/sys/kernel/tracing/events/syscalls/$tp/enable"
    if [ -f "$f" ]; then
        echo "  $tp/enable: $(cat $f 2>/dev/null || echo 'read failed')"
    fi
done
echo ""

echo "--- Kernel BPF config ---"
for opt in CONFIG_BPF CONFIG_BPF_SYSCALL CONFIG_BPF_JIT CONFIG_HAVE_EBPF_JIT \
           CONFIG_FTRACE_SYSCALLS CONFIG_KPROBE_EVENTS CONFIG_UPROBE_EVENTS; do
    val=$(grep "^$opt=" /boot/config-$(uname -r) 2>/dev/null || \
          zgrep "^$opt=" /proc/config.gz 2>/dev/null || echo "unknown")
    echo "  $val"
done
echo ""

echo "--- cgroup version ---"
stat -fc %T /sys/fs/cgroup/ 2>/dev/null
echo ""

echo "--- dwell-fiber slice ---"
if [ -d "/sys/fs/cgroup/dwell-fiber-v3.slice" ]; then
    echo "  exists"
    echo "  io.max: $(cat /sys/fs/cgroup/dwell-fiber-v3.slice/io.max 2>/dev/null | head -3)"
    echo "  procs: $(cat /sys/fs/cgroup/dwell-fiber-v3.slice/cgroup.procs 2>/dev/null | wc -l) PIDs"
else
    echo "  not present"
fi
echo ""

echo "--- Tools ---"
for t in bpftool bpftrace clang llvm-strip; do
    which $t 2>/dev/null && echo "  $t: $(which $t)" || echo "  $t: MISSING"
done
echo ""

echo "--- Go toolchain ---"
which go 2>/dev/null && go version || echo "go: MISSING"
echo ""

echo "--- AppArmor ---"
cat /proc/self/attr/current 2>/dev/null || echo "no apparmor attr"
echo ""

echo "=== END ==="
