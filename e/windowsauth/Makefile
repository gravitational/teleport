ifeq ($(VERSION),)
VERSION_FULL := v0.0.0
else
VERSION_FULL := $(VERSION)
endif

ifeq ($(OS),Windows_NT)
    GO_FLAGS :=
    CMD := powershell
else
    GO_FLAGS := CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC='x86_64-w64-mingw32-gcc'
    CMD :=
endif

INSTALLER=build/teleport-windows-auth-setup-$(VERSION_FULL)-amd64.exe
DLL=target/x86_64-pc-windows-gnu/release/teleport_auth_package.dll

.PHONY: all
all: $(DLL) $(INSTALLER)

.PHONY: test
test: $(DLL)
	cd installer && $(GO_FLAGS) go test ./...

.PHONY: clean
clean:
	rm -rf build
	rm -f installer/teleport.dll
	rm -f installer/*.syso
	rm -f *.syso

.PHONY: $(DLL)
$(DLL):
	cargo build --target=x86_64-pc-windows-gnu --release --locked

installer/teleport.dll: $(DLL)
	$(CMD) cp $(DLL) installer/teleport.dll

.PHONY: $(INSTALLER)
$(INSTALLER): installer/teleport.dll
	cd installer && \
	$(GO_FLAGS) go build -trimpath -ldflags "-s -w" -o "../$@" .

.PHONY: exe
exe: $(INSTALLER)

.PHONY: lint
lint:
	cargo clippy --target=x86_64-pc-windows-gnu --locked -- -D warnings && cargo fmt -- --check

.PHONY: format
format:
	cargo fmt
	go fmt ./...
	gci write -s standard -s default .
