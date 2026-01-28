# Root Makefile - delegates to src/Makefile

.PHONY: all build build-all test clean deps run lint help

all build build-all test clean deps run lint help:
	$(MAKE) -C src $@
