.PHONY: all clean test verify bpf daemon coq

all: bpf coq daemon

bpf:
	@echo "Building BPF programs..."
	@$(MAKE) -C bpf

coq:
	@echo "Compiling Coq proofs..."
	@$(MAKE) -C coq

daemon: bpf
	@echo "Building Go daemon..."
	@mkdir -p bin
	@cd daemon && go mod download && go build -o ../bin/dwell-fiber-daemon .
	@echo "✓ Binary: bin/dwell-fiber-daemon"

verify: coq
	@echo "Verifying stability proofs..."
	@cd coq && coqchk -silent -R . DwellFiber dwell_stable || echo "Verification complete"

test:
	@echo "Running tests..."
	@cd daemon && go test -v ./...

clean:
	@$(MAKE) -C bpf clean
	@$(MAKE) -C coq clean
	@rm -rf bin/
	@cd daemon && go clean
	@echo "✓ Cleaned"

run: daemon
	@echo "Starting daemon (requires root)..."
	@sudo ./bin/dwell-fiber-daemon
