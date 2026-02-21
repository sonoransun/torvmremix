.PHONY: all vm controller install clean

all: vm controller

vm:
	@scripts/build-vm.sh

controller:
	@scripts/build-controller.sh

install: all
	@scripts/install.sh

clean:
	rm -rf dist/
