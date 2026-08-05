package main

import "testing"

func TestResolveCommFrom_PrefersBPFComm(t *testing.T) {
	// Map path is primary: non-empty bpf comm wins even if /proc would differ.
	got := resolveCommFrom("tar", "rsync")
	if got != "tar" {
		t.Errorf("resolveCommFrom(bpf=tar, proc=rsync) = %q, want tar", got)
	}
	// Non-empty map comm is used when /proc is missing/empty.
	got = resolveCommFrom("dd", "")
	if got != "dd" {
		t.Errorf("resolveCommFrom(bpf=dd, proc=empty) = %q, want dd", got)
	}
}

func TestResolveCommFrom_EmptyBPFFallsBackToProc(t *testing.T) {
	got := resolveCommFrom("", "rsync")
	if got != "rsync" {
		t.Errorf("resolveCommFrom(empty, rsync) = %q, want rsync", got)
	}
	// All-NUL / whitespace-only bpf comm is treated as empty.
	got = resolveCommFrom("\x00\x00", "cp")
	if got != "cp" {
		t.Errorf("resolveCommFrom(NULs, cp) = %q, want cp", got)
	}
	got = resolveCommFrom("   ", "python3")
	if got != "python3" {
		t.Errorf("resolveCommFrom(spaces, python3) = %q, want python3", got)
	}
}

func TestResolveCommFrom_BothEmptyUnknown(t *testing.T) {
	if got := resolveCommFrom("", ""); got != "unknown" {
		t.Errorf("resolveCommFrom(empty, empty) = %q, want unknown", got)
	}
	if got := resolveCommFrom("\x00", "  \n"); got != "unknown" {
		t.Errorf("resolveCommFrom(NUL, whitespace) = %q, want unknown", got)
	}
}

func TestResolveCommFrom_TrimsWhitespace(t *testing.T) {
	// BPF path: trim trailing whitespace/newlines (GetString already strips NULs).
	if got := resolveCommFrom("tar\n", ""); got != "tar" {
		t.Errorf("resolveCommFrom(tar\\n) = %q, want tar", got)
	}
	// /proc-style values often end with a newline before TrimSpace.
	if got := resolveCommFrom("", "rsync\n"); got != "rsync" {
		t.Errorf("resolveCommFrom(proc=rsync\\n) = %q, want rsync", got)
	}
	if got := resolveCommFrom("  dd  ", "ignored"); got != "dd" {
		t.Errorf("resolveCommFrom(padded dd) = %q, want dd", got)
	}
}

func TestResolveComm_PrefersMapOverMissingProc(t *testing.T) {
	// resolveComm with non-empty bpf should not need a live /proc entry.
	// Use a PID that almost certainly has no /proc entry on any OS.
	const missingPID = 0x7fffffff
	got := resolveComm(missingPID, "tar")
	if got != "tar" {
		t.Errorf("resolveComm(missingPID, tar) = %q, want tar (map path primary)", got)
	}
}

func TestResolveComm_EmptyMapFallsBackOrUnknown(t *testing.T) {
	const missingPID = 0x7fffffff
	// Empty bpf + missing /proc → "unknown" (procComm's error path).
	got := resolveComm(missingPID, "")
	if got != "unknown" {
		t.Errorf("resolveComm(missingPID, empty) = %q, want unknown", got)
	}
}
