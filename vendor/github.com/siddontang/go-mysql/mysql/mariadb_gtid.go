package mysql

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/pingcap/errors"
	"github.com/siddontang/go-log/log"
	"github.com/siddontang/go/hack"
)

// MariadbGTID represent mariadb gtid, [domain ID]-[server-id]-[sequence]
type MariadbGTID struct {
	DomainID       uint32
	ServerID       uint32
	SequenceNumber uint64
}

// ParseMariadbGTID parses mariadb gtid, [domain ID]-[server-id]-[sequence]
func ParseMariadbGTID(str string) (*MariadbGTID, error) {
	if len(str) == 0 {
		return &MariadbGTID{0, 0, 0}, nil
	}

	seps := strings.Split(str, "-")

	gtid := new(MariadbGTID)

	if len(seps) != 3 {
		return gtid, errors.Errorf("invalid Mariadb GTID %v, must domain-server-sequence", str)
	}

	domainID, err := strconv.ParseUint(seps[0], 10, 32)
	if err != nil {
		return gtid, errors.Errorf("invalid MariaDB GTID Domain ID (%v): %v", seps[0], err)
	}

	serverID, err := strconv.ParseUint(seps[1], 10, 32)
	if err != nil {
		return gtid, errors.Errorf("invalid MariaDB GTID Server ID (%v): %v", seps[1], err)
	}

	sequenceID, err := strconv.ParseUint(seps[2], 10, 64)
	if err != nil {
		return gtid, errors.Errorf("invalid MariaDB GTID Sequence number (%v): %v", seps[2], err)
	}

	return &MariadbGTID{
		DomainID:       uint32(domainID),
		ServerID:       uint32(serverID),
		SequenceNumber: sequenceID}, nil
}

func (gtid *MariadbGTID) String() string {
	if gtid.DomainID == 0 && gtid.ServerID == 0 && gtid.SequenceNumber == 0 {
		return ""
	}

	return fmt.Sprintf("%d-%d-%d", gtid.DomainID, gtid.ServerID, gtid.SequenceNumber)
}

// Contain return whether one mariadb gtid covers another mariadb gtid
func (gtid *MariadbGTID) Contain(other *MariadbGTID) bool {
	return gtid.DomainID == other.DomainID && gtid.SequenceNumber >= other.SequenceNumber
}

// Clone clones a mariadb gtid
func (gtid *MariadbGTID) Clone() *MariadbGTID {
	o := new(MariadbGTID)
	*o = *gtid
	return o
}

func (gtid *MariadbGTID) forward(newer *MariadbGTID) error {
	if newer.DomainID != gtid.DomainID {
		return errors.Errorf("%s is not same with doamin of %s", newer, gtid)
	}

	/*
		Here's a simplified example of binlog events.
		Although I think one domain should have only one update at same time, we can't limit the user's usage.
		we just output a warn log and let it go on
		| mysqld-bin.000001 | 1453 | Gtid              |       112 |        1495 | BEGIN GTID 0-112-6  |
		| mysqld-bin.000001 | 1624 | Xid               |       112 |        1655 | COMMIT xid=74       |
		| mysqld-bin.000001 | 1655 | Gtid              |       112 |        1697 | BEGIN GTID 0-112-7  |
		| mysqld-bin.000001 | 1826 | Xid               |       112 |        1857 | COMMIT xid=75       |
		| mysqld-bin.000001 | 1857 | Gtid              |       111 |        1899 | BEGIN GTID 0-111-5  |
		| mysqld-bin.000001 | 1981 | Xid               |       111 |        2012 | COMMIT xid=77       |
		| mysqld-bin.000001 | 2012 | Gtid              |       112 |        2054 | BEGIN GTID 0-112-8  |
		| mysqld-bin.000001 | 2184 | Xid               |       112 |        2215 | COMMIT xid=116      |
		| mysqld-bin.000001 | 2215 | Gtid              |       111 |        2257 | BEGIN GTID 0-111-6  |
	*/
	if newer.SequenceNumber <= gtid.SequenceNumber {
		log.Warnf("out of order binlog appears with gtid %s vs current position gtid %s", newer, gtid)
	}

	gtid.ServerID = newer.ServerID
	gtid.SequenceNumber = newer.SequenceNumber
	return nil
}

// MariadbGTIDSet is a set of mariadb gtid
type MariadbGTIDSet struct {
	Sets map[uint32]*MariadbGTID
}

// ParseMariadbGTIDSet parses str into mariadb gtid sets
func ParseMariadbGTIDSet(str string) (GTIDSet, error) {
	s := new(MariadbGTIDSet)
	s.Sets = make(map[uint32]*MariadbGTID)
	if str == "" {
		return s, nil
	}

	sp := strings.Split(str, ",")

	//todo, handle redundant same uuid
	for i := 0; i < len(sp); i++ {
		err := s.Update(sp[i])
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return s, nil
}

// AddSet adds mariadb gtid into mariadb gtid set
func (s *MariadbGTIDSet) AddSet(gtid *MariadbGTID) error {
	if gtid == nil {
		return nil
	}

	o, ok := s.Sets[gtid.DomainID]
	if ok {
		err := o.forward(gtid)
		if err != nil {
			return errors.Trace(err)
		}
	} else {
		s.Sets[gtid.DomainID] = gtid
	}

	return nil
}

// Update updates mariadb gtid set
func (s *MariadbGTIDSet) Update(GTIDStr string) error {
	gtid, err := ParseMariadbGTID(GTIDStr)
	if err != nil {
		return err
	}

	err = s.AddSet(gtid)
	return errors.Trace(err)
}

func (s *MariadbGTIDSet) String() string {
	return hack.String(s.Encode())
}

// Encode encodes mariadb gtid set
func (s *MariadbGTIDSet) Encode() []byte {
	var buf bytes.Buffer
	sep := ""
	for _, gtid := range s.Sets {
		buf.WriteString(sep)
		buf.WriteString(gtid.String())
		sep = ","
	}

	return buf.Bytes()
}

// Clone clones a mariadb gtid set
func (s *MariadbGTIDSet) Clone() GTIDSet {
	clone := &MariadbGTIDSet{
		Sets: make(map[uint32]*MariadbGTID),
	}
	for domainID, gtid := range s.Sets {
		clone.Sets[domainID] = gtid.Clone()
	}

	return clone
}

// Equal returns true if two mariadb gtid set is same, otherwise return false
func (s *MariadbGTIDSet) Equal(o GTIDSet) bool {
	other, ok := o.(*MariadbGTIDSet)
	if !ok {
		return false
	}

	if len(other.Sets) != len(s.Sets) {
		return false
	}

	for domainID, gtid := range other.Sets {
		o, ok := s.Sets[domainID]
		if !ok {
			return false
		}

		if *gtid != *o {
			return false
		}
	}

	return true
}

// Contain return whether one mariadb gtid set covers another mariadb gtid set
func (s *MariadbGTIDSet) Contain(o GTIDSet) bool {
	other, ok := o.(*MariadbGTIDSet)
	if !ok {
		return false
	}

	for doaminID, gtid := range other.Sets {
		o, ok := s.Sets[doaminID]
		if !ok {
			return false
		}

		if !o.Contain(gtid) {
			return false
		}
	}

	return true
}
