#ifndef DWELL_MONITOR_BPF_H
#define DWELL_MONITOR_BPF_H

#include <linux/types.h>

struct io_event {
	__u32 pid;
	__u64 bytes_written;
	__u32 unique_files;
	__u64 timestamp_ns;
};

#endif // DWELL_MONITOR_BPF_H
