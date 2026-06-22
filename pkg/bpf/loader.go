package bpf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)

// DwellEvent matches the BPF struct dwell_event
type DwellEvent struct {
	PID        uint32
	TID        uint32
	Inode      uint64
	DurationNs uint64
	Timestamp  uint64
	Filename   [256]byte
	Comm       [16]byte
}

// BPFManager manages BPF program lifecycle
type BPFManager struct {
	Collection *ebpf.Collection
	Links      []link.Link
	reader     *ringbuf.Reader
	Events     chan DwellEvent
}

// LoadBPF loads the compiled BPF object file
func LoadBPF(objPath string) (*BPFManager, error) {
	log.Printf("Loading BPF program from %s", objPath)

	// Load BPF collection
	spec, err := ebpf.LoadCollectionSpec(objPath)
	if err != nil {
		return nil, fmt.Errorf("load collection spec: %w", err)
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		return nil, fmt.Errorf("create collection: %w", err)
	}

	log.Println("✓ BPF program loaded")

	return &BPFManager{
		Collection: coll,
		Links:      make([]link.Link, 0),
		Events:     make(chan DwellEvent, 100),
	}, nil
}

// AttachTracepoints attaches BPF programs to syscall tracepoints
func (bm *BPFManager) AttachTracepoints() error {
	log.Println("Attaching to tracepoints...")

	// Attach to sys_enter_openat
	progOpen := bm.Collection.Programs["handle_openat_enter"]
	if progOpen == nil {
		return fmt.Errorf("program handle_openat_enter not found")
	}

	tpOpen, err := link.Tracepoint("syscalls", "sys_enter_openat", progOpen, nil)
	if err != nil {
		return fmt.Errorf("attach openat tracepoint: %w", err)
	}
	bm.Links = append(bm.Links, tpOpen)
	log.Println("✓ Attached to sys_enter_openat")

	// v1.5.0: also attach sys_exit_openat to capture the real fd return value.
	progOpenExit := bm.Collection.Programs["handle_openat_exit"]
	if progOpenExit == nil {
		return fmt.Errorf("program handle_openat_exit not found")
	}

	tpOpenExit, err := link.Tracepoint("syscalls", "sys_exit_openat", progOpenExit, nil)
	if err != nil {
		return fmt.Errorf("attach openat exit tracepoint: %w", err)
	}
	bm.Links = append(bm.Links, tpOpenExit)
	log.Println("✓ Attached to sys_exit_openat")

	// Attach to sys_enter_close
	progClose := bm.Collection.Programs["handle_close_enter"]
	if progClose == nil {
		return fmt.Errorf("program handle_close_enter not found")
	}

	tpClose, err := link.Tracepoint("syscalls", "sys_enter_close", progClose, nil)
	if err != nil {
		return fmt.Errorf("attach close tracepoint: %w", err)
	}
	bm.Links = append(bm.Links, tpClose)
	log.Println("✓ Attached to sys_enter_close")

	return nil
}

// AttachWIPTracepoint attaches the V3 sys_enter_write tracepoint that feeds the
// WIP (Weighted I/O Pressure) signal. Kept separate from AttachTracepoints so
// the write tracepoint -- which fires on every write syscall system-wide -- is
// only attached when V3 observation is enabled (--use-v3-wip).
func (bm *BPFManager) AttachWIPTracepoint() error {
	prog := bm.Collection.Programs["handle_write_enter"]
	if prog == nil {
		return fmt.Errorf("program handle_write_enter not found")
	}
	tp, err := link.Tracepoint("syscalls", "sys_enter_write", prog, nil)
	if err != nil {
		return fmt.Errorf("attach write tracepoint: %w", err)
	}
	bm.Links = append(bm.Links, tp)
	log.Println("✓ Attached to sys_enter_write (V3 WIP)")
	return nil
}

// WIPSample is one PID's accumulated WIP-window state, mirroring the BPF
// struct wip_state. Counts are cumulative since the window start; the caller
// divides by the elapsed window to get per-second rates.
type WIPSample struct {
	PID           uint32
	WindowStartNs uint64
	TBWAccum      uint64
	UFMAccum      uint64
}

// wipState matches the BPF struct wip_state field order (window_start, tbw, ufm).
type wipState struct {
	WindowStartNs uint64
	TBWAccum      uint64
	UFMAccum      uint64
}

// ReadWIP snapshots the per-PID WIP accumulators and deletes the entries it read,
// so each poll measures a fresh window. Entries that reappear before the next
// poll simply start a new window on their next syscall.
func (bm *BPFManager) ReadWIP() ([]WIPSample, error) {
	m := bm.Collection.Maps["wip_tracker"]
	if m == nil {
		return nil, fmt.Errorf("map 'wip_tracker' not found")
	}

	var samples []WIPSample
	var pid uint32
	var st wipState
	iter := m.Iterate()
	for iter.Next(&pid, &st) {
		samples = append(samples, WIPSample{
			PID:           pid,
			WindowStartNs: st.WindowStartNs,
			TBWAccum:      st.TBWAccum,
			UFMAccum:      st.UFMAccum,
		})
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("iterate wip_tracker: %w", err)
	}

	// Reset windows by deleting the keys we just read.
	for i := range samples {
		key := samples[i].PID
		if err := m.Delete(&key); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
			log.Printf("WIP reset: delete pid %d: %v", key, err)
		}
	}
	return samples, nil
}

// StartReader starts reading events from the ring buffer
func (bm *BPFManager) StartReader() error {
	log.Println("Starting ring buffer reader...")

	eventsMap := bm.Collection.Maps["events"]
	if eventsMap == nil {
		return fmt.Errorf("map 'events' not found")
	}

	rd, err := ringbuf.NewReader(eventsMap)
	if err != nil {
		return fmt.Errorf("create ring buffer reader: %w", err)
	}
	bm.reader = rd

	// Start reading in goroutine
	go func() {
		defer rd.Close()

		for {
			record, err := rd.Read()
			if err != nil {
				log.Printf("Error reading ring buffer: %v", err)
				continue
			}

			var event DwellEvent
			buf := bytes.NewReader(record.RawSample)
			if err := binary.Read(buf, binary.LittleEndian, &event); err != nil {
				log.Printf("Error parsing event: %v", err)
				continue
			}

			// Send to channel
			select {
			case bm.Events <- event:
			default:
				// Channel full, drop event
			}
		}
	}()

	log.Println("✓ Ring buffer reader started")
	return nil
}

// ReadStats reads the kernel-side session counters from the BPF "stats"
// per-CPU array and sums across CPUs. total counts every close that matched a
// tracked open (pre-filter); filtered counts the subset dropped by the <100ms
// in-kernel noise filter. These let userspace report fast-intermittent activity
// that never reaches the ring buffer.
func (bm *BPFManager) ReadStats() (total, filtered uint64, err error) {
	m := bm.Collection.Maps["stats"]
	if m == nil {
		return 0, 0, fmt.Errorf("map 'stats' not found")
	}

	sum := func(idx uint32) (uint64, error) {
		var perCPU []uint64
		if err := m.Lookup(idx, &perCPU); err != nil {
			return 0, err
		}
		var s uint64
		for _, v := range perCPU {
			s += v
		}
		return s, nil
	}

	// Read filtered before total: both counters increase monotonically and
	// filtered is always a kernel-side subset of total, so sampling filtered
	// first then total guarantees total >= filtered despite the non-atomic
	// two-map-lookup read (a session landing between the reads only ever
	// raises the later-read total).
	const statTotal, statFiltered = uint32(0), uint32(1)
	if filtered, err = sum(statFiltered); err != nil {
		return 0, 0, fmt.Errorf("read filtered: %w", err)
	}
	if total, err = sum(statTotal); err != nil {
		return 0, 0, fmt.Errorf("read total: %w", err)
	}
	return total, filtered, nil
}

// Close cleans up BPF resources
func (bm *BPFManager) Close() error {
	log.Println("Closing BPF manager...")

	// Close reader
	if bm.reader != nil {
		bm.reader.Close()
	}

	// Detach links
	for _, l := range bm.Links {
		l.Close()
	}

	// Close collection
	if bm.Collection != nil {
		bm.Collection.Close()
	}

	close(bm.Events)
	log.Println("✓ BPF resources cleaned up")
	return nil
}

// GetString extracts null-terminated string from byte array
func GetString(data []byte) string {
	n := bytes.IndexByte(data, 0)
	if n == -1 {
		n = len(data)
	}
	return string(data[:n])
}
