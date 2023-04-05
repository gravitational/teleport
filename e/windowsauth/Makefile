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
DLL=build/teleport.dll

# this transforms version to something FILEVERSION in *.rc files understands, e.g
# v12.0.0-alpha1 -> 12,0,0
# v12.1 -> 12,1
# v12.0.0.0.0.0 -> 12,0,0
dot:= .
comma:= ,
empty:=
space:= $(empty) $(empty)
VERSION_COMMA:=$(subst v,,$(firstword $(VERSION_FULL)))
VERSION_COMMA:=$(firstword $(subst -, ,$(VERSION_COMMA)))
VERSION_COMMA:=$(subst $(space),$(comma),$(wordlist 1,3,$(subst $(dot), ,$(VERSION_COMMA))))

.PHONY: all
all: $(DLL) $(INSTALLER)

.PHONY: test
test: $(DLL)
	$(GO_FLAGS) go test ./...

.PHONY: clean
clean:
	rm -rf build
	rm -f installer/teleport.dll
	rm -f installer/*.syso
	rm -f *.syso

$(DLL): *.go *.c *.h logo.syso
	$(GO_FLAGS) go build -buildmode=c-shared -o build/teleport.dll

logo.syso: logo.rc icon.bmp
	$(WINDRES) -D VERSION="$(VERSION_FULL)" -D VERSION_COMMA="$(VERSION_COMMA)" -i logo.rc -o logo.syso --use-temp-file -O coff

installer/resources.syso: installer/resources.rc installer/teleport.ico installer/manifest.xml
	$(WINDRES) -D VERSION="$(VERSION_FULL)" -D VERSION_COMMA="$(VERSION_COMMA)" -i installer/resources.rc -o installer/resources.syso --use-temp-file -O coff

installer/teleport.dll: $(DLL)
	$(CMD) cp build/teleport.dll installer/teleport.dll

$(INSTALLER): installer/resources.syso installer/*.go installer/teleport.dll
	cd installer && \
	$(GO_FLAGS) go build -o "../$@" .

.PHONY: format
format:
	clang-format -i *.c *.h
	go fmt ./...
	gci write -s standard -s default .
