.PHONY: all vm controller ios install clean test test-integration lint

all: vm controller

vm:
	@scripts/build-vm.sh

controller:
	@scripts/build-controller.sh

install: all
	@scripts/install.sh

clean:
	rm -rf dist/

test:
	cd controller && go test -v -race ./...
	cd android && ./gradlew testDebugUnitTest

test-integration:
	cd controller && go test -v -tags integration -race ./...

ios:
	xcodebuild -project ios/TorVM.xcodeproj -scheme TorVM -sdk iphoneos -configuration Release build

lint:
	cd controller && golangci-lint run ./...
