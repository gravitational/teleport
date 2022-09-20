package mysql

import "github.com/pingcap/errors"

type GTIDSet interface {
	String() string

	// Encode GTID set into binary format used in binlog dump commands
	Encode() []byte

	Equal(o GTIDSet) bool

	Contain(o GTIDSet) bool

	Update(GTIDStr string) error

	Clone() GTIDSet
}

func ParseGTIDSet(flavor string, s string) (GTIDSet, error) {
	switch flavor {
	case MySQLFlavor:
		return ParseMysqlGTIDSet(s)
	case MariaDBFlavor:
		return ParseMariadbGTIDSet(s)
	default:
		return nil, errors.Errorf("invalid flavor %s", flavor)
	}
}
