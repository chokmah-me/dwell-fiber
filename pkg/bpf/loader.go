package bpf

import (
	"bytes"
	"encoding/binary"
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
