ifeq ($(VERSION),)
VERSION_FULL := v0.0.0
else
VERSION_FULL := $(VERSION)
endif

ifeq ($(OS),Windows_NT)
    WINDRES := windres
    GO_FLAGS :=
    CMD := powershell
else
    WINDRES := x86_64-w64-mingw32-windres
    GO_FLAGS := CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC='x86_64-w64-mingw32-gcc'
    CMD :=
endif

INSTALLER=build/teleport-windows-auth-setup-$(VERSION_FULL)-amd64.exe
DLL=target/x86_64-pc-windows-gnu/release/teleport_auth_package.dll

# this transforms version to something FILEVERSION in *.rc files understands, e.g
# v12.0.0+4afbc1 -> 12,0,0
# v12.0.0-alpha1 -> 12,0,0
# v12.1 -> 12,1
# v12.0.0.0.0.0 -> 12,0,0
dot:= .
comma:= ,
empty:=
space:= $(empty) $(empty)
VERSION_COMMA:=$(subst v,,$(firstword $(VERSION_FULL)))
VERSION_COMMA:=$(firstword $(subst -, ,$(subst +, ,$(VERSION_COMMA))))
VERSION_COMMA:=$(subst $(space),$(comma),$(wordlist 1,3,$(subst $(dot), ,$(VERSION_COMMA))))

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
	cargo build --target=x86_64-pc-windows-gnu --release

installer/resources.syso: installer/resources.rc installer/teleport.ico installer/manifest.xml
	$(WINDRES) -D VERSION="$(VERSION_FULL)" -D VERSION_COMMA="$(VERSION_COMMA)" -i installer/resources.rc -o installer/resources.syso --use-temp-file -O coff

installer/teleport.dll: $(DLL)
	$(CMD) cp $(DLL) installer/teleport.dll

.PHONY: $(INSTALLER)
$(INSTALLER): installer/resources.syso installer/teleport.dll
	cd installer && \
	$(GO_FLAGS) go build -o "../$@" .

.PHONY: format
format:
	clang-format -i *.c *.h
	go fmt ./...
	gci write -s standard -s default .
