.PHONY: build test vet run status toggle android release install install-desktop install-overlay

build:
	cd receiver && go build -o localwhisper ./cmd/localwhisper && go build -o localwhisper-integration ./cmd/localwhisper-integration && go build -o localwhisper-overlay ./cmd/localwhisper-overlay

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
	./scripts/install-desktop.sh

install-desktop:
	./scripts/install-desktop.sh

install-overlay:
	./scripts/install-overlay.sh
