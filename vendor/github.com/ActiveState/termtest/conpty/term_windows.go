// +build windows

package conpty

import (
	"fmt"
	"log"
	"syscall"

	"github.com/Azure/go-ansiterm/winterm"
)

func InitTerminal(disableNewlineAutoReturn bool) (func(), error) {
	stdoutFd := int(syscall.Stdout)

	// fmt.Printf("file descriptors <%d >%d\n", stdinFd, stdoutFd)

	oldOutMode, err := winterm.GetConsoleMode(uintptr(stdoutFd))
	if err != nil {
		return func() {}, fmt.Errorf("failed to retrieve stdout mode: %w", err)
	}

	// fmt.Printf("old modes: <%d >%d\n", oldInMode, oldOutMode)
	newOutMode := oldOutMode | winterm.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if disableNewlineAutoReturn {
		newOutMode |= winterm.DISABLE_NEWLINE_AUTO_RETURN
	}

	err = winterm.SetConsoleMode(uintptr(stdoutFd), newOutMode)
	if err != nil {
		return func() {}, fmt.Errorf("failed to set stdout mode: %w", err)
	}

	// dump(uintptr(stdoutFd))
	return func() {
		err = winterm.SetConsoleMode(uintptr(stdoutFd), oldOutMode)
		if err != nil {
			log.Fatalf("Failed to reset output terminal mode to %d: %v\n", oldOutMode, err)
		}
	}, nil
}

func dump(fd uintptr) {
	fmt.Printf("FD=%d\n", fd)
	modes, err := winterm.GetConsoleMode(fd)
	if err != nil {
		panic(err)
	}

	fmt.Printf("ENABLE_ECHO_INPUT=%d, ENABLE_PROCESSED_INPUT=%d ENABLE_LINE_INPUT=%d\n",
		modes&winterm.ENABLE_ECHO_INPUT,
		modes&winterm.ENABLE_PROCESSED_INPUT,
		modes&winterm.ENABLE_LINE_INPUT)
	fmt.Printf("ENABLE_WINDOW_INPUT=%d, ENABLE_MOUSE_INPUT=%d\n",
		modes&winterm.ENABLE_WINDOW_INPUT,
		modes&winterm.ENABLE_MOUSE_INPUT)
	fmt.Printf("enableVirtualTerminalInput=%d, enableVirtualTerminalProcessing=%d, disableNewlineAutoReturn=%d\n",
		modes&winterm.ENABLE_VIRTUAL_TERMINAL_INPUT,
		modes&winterm.ENABLE_VIRTUAL_TERMINAL_PROCESSING,
		modes&winterm.DISABLE_NEWLINE_AUTO_RETURN)
}
