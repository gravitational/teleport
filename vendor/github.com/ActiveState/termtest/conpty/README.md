# termtest/conpty

Support for the [Windows pseudo
console](https://devblogs.microsoft.com/commandline/windows-command-line-introducing-the-windows-pseudo-console-conpty/)
in Go.

Developed as part of the cross-platform terminal automation library
[expect](https://github.com/ActiveState/termtest/expect) for the [ActiveState
state tool](https://www.activestate.com/products/platform/state-tool/).

## Example

See ./cmd/example/main.go

## Client configuration

On Windows, you may have to adjust the programme that you are running in the
pseudo-console, by configuring the standard output handler to process virtual
terminal codes. See https://docs.microsoft.com/en-us/windows/console/setconsolemode

This package comes with a convenience function `InitTerminal()` that you can
use in your client to set this option.

