package mysql

import (
	"fmt"
	"strconv"
	"strings"
)

// For binlog filename + position based replication
type Position struct {
	Name string
	Pos  uint32
}

func (p Position) Compare(o Position) int {
	// First compare binlog name
	nameCmp := CompareBinlogFileName(p.Name, o.Name)
	if nameCmp != 0 {
		return nameCmp
	}
	// Same binlog file, compare position
	if p.Pos > o.Pos {
		return 1
	} else if p.Pos < o.Pos {
		return -1
	} else {
		return 0
	}
}

func (p Position) String() string {
	return fmt.Sprintf("(%s, %d)", p.Name, p.Pos)
}

func CompareBinlogFileName(a, b string) int {
	// sometimes it's convenient to construct a `Position` literal with no `Name`
	if a == "" && b == "" {
		return 0
	} else if a == "" {
		return -1
	} else if b == "" {
		return 1
	}

	splitBinlogName := func(n string) (string, int) {
		// mysqld appends a numeric extension to the binary log base name to generate binary log file names
		// ...
		// If you supply an extension in the log name (for example, --log-bin=base_name.extension),
		// the extension is silently removed and ignored.
		// ref: https://dev.mysql.com/doc/refman/8.0/en/binary-log.html
		i := strings.LastIndexByte(n, '.')
		if i == -1 {
			// try keeping backward compatibility
			return n, 0
		}

		seq, err := strconv.Atoi(n[i+1:])
		if err != nil {
			panic(fmt.Sprintf("binlog file %s doesn't contain numeric extension", err))
		}
		return n[:i], seq
	}

	aBase, aSeq := splitBinlogName(a)
	bBase, bSeq := splitBinlogName(b)

	if aBase > bBase {
		return 1
	} else if aBase < bBase {
		return -1
	}

	if aSeq > bSeq {
		return 1
	} else if aSeq < bSeq {
		return -1
	} else {
		return 0
	}
}
