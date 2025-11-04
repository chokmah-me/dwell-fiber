COQC := coqc
COQFLAGS := -R . DwellFiber

VFILES := dwell_stable.v
VOFILES := $(VFILES:.v=.vo)

.PHONY: all clean

all: $(VOFILES)

%.vo: %.v
	@echo "Compiling $<..."
	$(COQC) $(COQFLAGS) $<
	@echo "✓ Verified $<"

clean:
	rm -f $(VOFILES) .*.aux *.glob .*.cache
