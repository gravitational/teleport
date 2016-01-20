// Provides basic bulding blocks for advanced console UI
//
// Coordinate system:
//
//  1/1---X---->
//   |
//   Y
//   |
//   v
//
// Documentation for ANSI codes: http://en.wikipedia.org/wiki/ANSI_escape_code#Colors
//
// Inspired by: http://www.darkcoding.net/software/pretty-command-line-console-output-on-unix-in-python-and-go-lang/
package goterm

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// Reset all custom styles
const RESET = "\033[0m"

// Reset to default color
const RESET_COLOR = "\033[32m"

// List of possible colors
const (
	BLACK = iota
	RED
	GREEN
	YELLOW
	BLUE
	MAGENTA
	CYAN
	WHITE
)

func getColor(code int) string {
	return fmt.Sprintf("\033[3%dm", code)
}

func getBgColor(code int) string {
	return fmt.Sprintf("\033[4%dm", code)
}

// Set percent flag: num | PCT
//
// Check percent flag: num & PCT
//
// Reset percent flag: num & 0xFF
const PCT = 0x80000000

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getWinsize() (*winsize, error) {
	ws := new(winsize)

	var _TIOCGWINSZ int64

	switch runtime.GOOS {
	case "linux":
		_TIOCGWINSZ = 0x5413
	case "darwin":
		_TIOCGWINSZ = 1074295912
	}

	r1, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(_TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)

	if int(r1) == -1 {
		fmt.Println("Error:", os.NewSyscallError("GetWinsize", errno))
		return nil, os.NewSyscallError("GetWinsize", errno)
	}
	return ws, nil
}

// Global screen buffer
// Its not recommented write to buffer dirrectly, use package Print,Printf,Println fucntions instead.
var Screen *bytes.Buffer = new(bytes.Buffer)

// Get relative or absolute coorditantes
// To get relative, set PCT flag to number:
//
//      // Get 10% of total width to `x` and 20 to y
//      x, y = tm.GetXY(10|tm.PCT, 20)
//
func GetXY(x int, y int) (int, int) {
	if y == -1 {
		y = CurrentHeight() + 1
	}

	if x&PCT != 0 {
		x = int((x & 0xFF) * Width() / 100)
	}

	if y&PCT != 0 {
		y = int((y & 0xFF) * Height() / 100)
	}

	return x, y
}

type sf func(int, string) string

// Apply given transformation func for each line in string
func applyTransform(str string, transform sf) (out string) {
	out = ""

	for idx, line := range strings.Split(str, "\n") {
		out += transform(idx, line)
	}

	return
}

// Clear screen
func Clear() {
	fmt.Print("\033[2J")
}

// Move cursor to given position
func MoveCursor(x int, y int) {
	fmt.Fprintf(Screen, "\033[%d;%dH", x, y)
}

// Move string to possition
func MoveTo(str string, x int, y int) (out string) {
	x, y = GetXY(x, y)

	return applyTransform(str, func(idx int, line string) string {
		return fmt.Sprintf("\033[%d;%dH%s", y+idx, x, line)
	})
}

// Make bold
func Bold(str string) string {
	return applyTransform(str, func(idx int, line string) string {
		return fmt.Sprintf("\033[1m%s\033[0m", line)
	})
}

// Apply given color to string:
//
//     tm.Color("RED STRING", tm.RED)
//
func Color(str string, color int) string {
	return applyTransform(str, func(idx int, line string) string {
		return fmt.Sprintf("%s%s%s", getColor(color), line, RESET)
	})
}

// Change background color of string:
//
//     tm.Background("string", tm.RED)
//
func Background(str string, color int) string {
	return applyTransform(str, func(idx int, line string) string {
		return fmt.Sprintf("%s%s%s", getBgColor(color), line, RESET)
	})
}

// Get console width
func Width() int {
	ws, err := getWinsize()

	if err != nil {
		return -1
	}

	return int(ws.Col)
}

// Get console height
func Height() int {
	ws, err := getWinsize()
	if err != nil {
		return -1
	}
	return int(ws.Row)
}

// Get current height. Line count in Screen buffer.
func CurrentHeight() int {
	return strings.Count(Screen.String(), "\n")
}

// Flush buffer and ensure that it will not overflow screen
func Flush() {
	for idx, str := range strings.Split(Screen.String(), "\n") {
		if idx > Height() {
			return
		}

		fmt.Println(str)
	}

	Screen.Reset()
}

func Print(a ...interface{}) {
	fmt.Fprint(Screen, a...)
}

func Println(a ...interface{}) {
	fmt.Fprintln(Screen, a...)
}

func Printf(format string, a ...interface{}) {
	fmt.Fprintf(Screen, format, a...)
}
