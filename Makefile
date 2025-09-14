# Root Makefile - delegates to src/Makefile

.PHONY: all build build-all test clean deps run help

all build build-all test clean deps run help:
	$(MAKE) -C src $@
