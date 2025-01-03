package protocol

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/exp/constraints"
)

func parseOneLine(prefixLen, dataLen int, line string) ([]byte, error) {
	if len(line) < (prefixLen + dataLen) {
		return nil, trace.BadParameter("line too short")
	}
	encodedBytes := line[prefixLen : prefixLen+dataLen]
	encodedBytes = strings.ReplaceAll(encodedBytes, " ", "")
	decodedBytes, err := hex.DecodeString(encodedBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return decodedBytes, nil
}

func parseWiresharkLine(line string) ([]byte, error) {
	// Wireshark format:
	// 0000   00 00 00 ac 06 20 00 00 00 00 de ad be ef 00 a2   ..... ..........
	prefix := "0000   "
	data := "00 00 00 ac 06 20 00 00 00 00 de ad be ef 00 a2"

	return parseOneLine(len(prefix), len(data), line)
}

func parseHexdumpLine(line string) ([]byte, error) {
	// hexdump -C / hex.Dump() format:
	// 00000000  01 05 00 00 01 00 00 00  01 3e 01 2c 0c 41 20 00  |.........>.,.A .|
	prefix := "00000000  "
	data := "01 05 00 00 01 00 00 00  01 3e 01 2c 0c 41 20 00"

	return parseOneLine(len(prefix), len(data), line)
}

func skipLine(line string) bool {
	line = strings.TrimSpace(line)
	// skip possible empty lines
	if line == "" {
		return true
	}

	// lines that start with # are comments, skip those too.
	if strings.HasPrefix(line, "#") {
		return true
	}

	return false
}

func DecodeHexDump(dump string) ([]byte, error) {
	var parseLine func(line string) ([]byte, error)
	lines := strings.Split(dump, "\n")

	if len(lines) == 0 {
		return nil, nil
	}

	const hexdumpOffsetLength = len("00000000")
	const wiresharkOffsetLength = len("0000")

	for _, line := range lines {
		if !skipLine(line) {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return nil, trace.BadParameter("expected at least 2 fields, got %d", len(fields))
			}
			firstOffset := fields[0]
			switch len(firstOffset) {
			case hexdumpOffsetLength:
				parseLine = parseHexdumpLine
			case wiresharkOffsetLength:
				parseLine = parseWiresharkLine
			default:
				return nil, trace.BadParameter("cannot guess the format (firstOffset:%v) (line: %v)", firstOffset, line)
			}
			break
		}
	}

	if parseLine == nil {
		return nil, trace.BadParameter("cannot guess the format (line: %v)", dump)
	}

	var ret bytes.Buffer
	for lineNum, line := range lines {
		if skipLine(line) {
			continue
		}

		decodedBytes, err := parseLine(line)
		if err != nil {
			return nil, trace.BadParameter("error parsing line %v, line: %q, error: %v", lineNum+1, line, err)
		}

		ret.Write(decodedBytes)
	}
	return ret.Bytes(), nil
}

func ParseDumpToPacket(largeSDU bool, dump string) (Packet, error) {
	var protocolVersion uint16
	if largeSDU {
		protocolVersion = TNSVersionMinLargeSdu
	}

	decoded, err := DecodeHexDump(dump)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reader := bytes.NewReader(decoded)

	result, err := ReadPacket(protocolVersion, reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result.SuccessPacket, nil
}

func formatBinary[T constraints.Integer](flags T) string {
	var out strings.Builder

	bin := strconv.FormatInt(int64(flags), 2)
	width := 8 * int(reflect.TypeOf(flags).Size())
	padding := width - len(bin)
	if padding > 0 {
		bin = strings.Repeat("0", padding) + bin
	}

	out.WriteString("0b")

	for pos, chr := range bin {
		out.WriteRune(chr)
		// add space after each 4th rune, but only if this isn't the last digit.
		if (pos%4) == 3 && (pos != len(bin)-1) {
			out.WriteRune('_')
		}
	}

	return out.String()
}
