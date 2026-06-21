# Use coqc from PATH by default; override with `make COQC=/path/to/coqc`.
COQC ?= coqc
COQFLAGS := -R . DwellFiber

VFILES := dwell_stable.v dwell_kernel_resilience.v dwell_extended.v
VOFILES := $(VFILES:.v=.vo)

.PHONY: all clean verify

all: $(VOFILES)

dwell_stable.vo: dwell_stable.v
	@echo "Compiling $<..."
	$(COQC) $(COQFLAGS) $<
	@echo "✓ Verified $<"

dwell_extended.vo: dwell_extended.v
	@echo "Compiling $<..."
	$(COQC) $(COQFLAGS) $<
	@echo "✓ Verified $<"

dwell_kernel_resilience.vo: dwell_kernel_resilience.v dwell_stable.vo
	@echo "Compiling $<..."
	$(COQC) $(COQFLAGS) $<
	@echo "✓ Verified $<"

verify: all
	@echo "✓ Coq proofs verified successfully"
	@echo "  - dwell_stable.v: ADMM stability"
	@echo "  - dwell_kernel_resilience.v: Event loss resilience"
	@echo "  - dwell_extended.v: Liveness, Fairness, Attack Resistance"

clean:
	rm -f $(VOFILES) .*.aux *.glob .*.cache
	@echo "✓ Cleaned"
