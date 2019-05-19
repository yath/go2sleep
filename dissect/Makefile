GO := go

dissect.so: dissect.go cglue.go cglue.c
	$(GO) build -buildmode=c-shared -o $@

INSTALL_DIR := $(HOME)/.local/lib/wireshark/plugins/2.6/epan/

.PHONY: install
install: dissect.so
	install -d $(INSTALL_DIR)
	install -m755 $< $(INSTALL_DIR)

.PHONY: clean
clean:
	$(GO) clean
	rm -f dissect.h
