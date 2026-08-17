.PHONY: build test vet run status toggle android release install install-indicator

build:
	cd receiver && go build -o localwhisper ./cmd/localwhisper && go build -o localwhisper-integration ./cmd/localwhisper-integration

test:
	cd receiver && go test ./...

vet:
	cd receiver && go vet ./...

run: build
	./receiver/localwhisper

status:
	curl --fail --silent http://127.0.0.1:8766/status

toggle:
	./scripts/toggle.sh

android:
	cd android && ./gradlew assembleDebug

release:
	cd android && ./gradlew assembleRelease

install:
	./scripts/setup-fedora.sh

install-indicator:
	mkdir -p /tmp/localwhisper-extension
	gnome-extensions pack --force --out-dir /tmp/localwhisper-extension gnome-extension/localwhisper-indicator@local
	gnome-extensions install --force /tmp/localwhisper-extension/localwhisper-indicator@local.shell-extension.zip
