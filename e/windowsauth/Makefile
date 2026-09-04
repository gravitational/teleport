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

# Linking with IMAGE_DLLCHARACTERISTICS_FORCE_INTEGRITY makes Windows verify the
# image signature before loading the DLL. The Microsoft-signed DLL needs it to
# load into a protected LSA, but an *unsigned* DLL carrying it does not load at
# all -- and the DLL shipped in the installer today is unsigned. So this stays
# off for the release build and is turned on only by the cab target, which
# builds the DLL that gets submitted for signing.
FORCE_INTEGRITY ?= no

ifeq ($(FORCE_INTEGRITY),yes)
RUSTFLAGS += -C link-arg=-Wl,--forceinteg
export RUSTFLAGS
endif

INSTALLER=build/teleport-windows-auth-setup-$(VERSION_FULL)-amd64.exe
DLL=target/x86_64-pc-windows-gnu/release/teleport_auth_package.dll

# The DLL submitted for signing is stamped with the Teleport version it was cut
# from, which is what the release build already stamps. Once Microsoft signs it
# the bytes are frozen, so Teleport will move on ahead of it -- a DLL reading
# v19.0.0 still shipping in 21.x is the intended outcome, and records the source
# revision that was signed.
CAB=build/teleport-auth-package-$(VERSION_FULL).cab

.PHONY: all
all: $(DLL) $(INSTALLER)

.PHONY: test
test: installer/teleport.dll
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

# The cabinet submitted to the Microsoft LSA file signing portal. Microsoft
# requires a single cabinet containing no folders and nothing but the binaries
# to be signed, so this packs exactly one file, stored as a bare "teleport.dll".
#
# Windows only. makecab is Microsoft's own cab writer, and a rejected submission
# costs a manual round trip through the portal, so it is not worth substituting
# a third-party one to gain a Linux build.
#
# The submission build differs from the release build in exactly one way: the
# DLL is linked with FORCE_INTEGRITY. That has to be set before the DLL rules
# are evaluated, hence the recursive make. $(DLL) is phony, so this always
# relinks rather than reusing a release-built DLL that lacks the flag.
.PHONY: cab
cab:
	$(MAKE) FORCE_INTEGRITY=yes $(CAB)

$(CAB): test installer/teleport.dll
	cd installer && go run ./cmd/checkintegrity teleport.dll
	powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path build | Out-Null"
	makecab /F teleport-auth-package.ddf /D CabinetNameTemplate=$(notdir $(CAB))

.PHONY: lint
lint:
	cargo clippy --target=x86_64-pc-windows-gnu --locked -- -D warnings && cargo fmt -- --check

.PHONY: format
format:
	cargo fmt
	go fmt ./...
	gci write -s standard -s default .
