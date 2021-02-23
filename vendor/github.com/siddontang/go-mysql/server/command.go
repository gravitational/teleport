package server

import (
	"bytes"
	"fmt"

	. "github.com/siddontang/go-mysql/mysql"
	"github.com/siddontang/go/hack"
)

type Handler interface {
	//handle COM_INIT_DB command, you can check whether the dbName is valid, or other.
	UseDB(dbName string) error
	//handle COM_QUERY command, like SELECT, INSERT, UPDATE, etc...
	//If Result has a Resultset (SELECT, SHOW, etc...), we will send this as the response, otherwise, we will send Result
	HandleQuery(query string) (*Result, error)
	//handle COM_FILED_LIST command
	HandleFieldList(table string, fieldWildcard string) ([]*Field, error)
	//handle COM_STMT_PREPARE, params is the param number for this statement, columns is the column number
	//context will be used later for statement execute
	HandleStmtPrepare(query string) (params int, columns int, context interface{}, err error)
	//handle COM_STMT_EXECUTE, context is the previous one set in prepare
	//query is the statement prepare query, and args is the params for this statement
	HandleStmtExecute(context interface{}, query string, args []interface{}) (*Result, error)
	//handle COM_STMT_CLOSE, context is the previous one set in prepare
	//this handler has no response
	HandleStmtClose(context interface{}) error
	//handle any other command that is not currently handled by the library,
	//default implementation for this method will return an ER_UNKNOWN_ERROR
	HandleOtherCommand(cmd byte, data []byte) error
}

func (c *Conn) HandleCommand() error {
	if c.Conn == nil {
		return fmt.Errorf("connection closed")
	}

	data, err := c.ReadPacket()
	if err != nil {
		c.Close()
		c.Conn = nil
		return err
	}

	v := c.dispatch(data)

	err = c.writeValue(v)

	if c.Conn != nil {
		c.ResetSequence()
	}

	if err != nil {
		c.Close()
		c.Conn = nil
	}
	return err
}

func (c *Conn) dispatch(data []byte) interface{} {
	cmd := data[0]
	data = data[1:]

	switch cmd {
	case COM_QUIT:
		c.Close()
		c.Conn = nil
		return noResponse{}
	case COM_QUERY:
		if r, err := c.h.HandleQuery(hack.String(data)); err != nil {
			return err
		} else {
			return r
		}
	case COM_PING:
		return nil
	case COM_INIT_DB:
		if err := c.h.UseDB(hack.String(data)); err != nil {
			return err
		} else {
			return nil
		}
	case COM_FIELD_LIST:
		index := bytes.IndexByte(data, 0x00)
		table := hack.String(data[0:index])
		wildcard := hack.String(data[index+1:])

		if fs, err := c.h.HandleFieldList(table, wildcard); err != nil {
			return err
		} else {
			return fs
		}
	case COM_STMT_PREPARE:
		c.stmtID++
		st := new(Stmt)
		st.ID = c.stmtID
		st.Query = hack.String(data)
		var err error
		if st.Params, st.Columns, st.Context, err = c.h.HandleStmtPrepare(st.Query); err != nil {
			return err
		} else {
			st.ResetParams()
			c.stmts[c.stmtID] = st
			return st
		}
	case COM_STMT_EXECUTE:
		if r, err := c.handleStmtExecute(data); err != nil {
			return err
		} else {
			return r
		}
	case COM_STMT_CLOSE:
		c.handleStmtClose(data)
		return noResponse{}
	case COM_STMT_SEND_LONG_DATA:
		c.handleStmtSendLongData(data)
		return noResponse{}
	case COM_STMT_RESET:
		if r, err := c.handleStmtReset(data); err != nil {
			return err
		} else {
			return r
		}
	default:
		return c.h.HandleOtherCommand(cmd, data)
	}

	return fmt.Errorf("command %d is not handled correctly", cmd)
}

type EmptyHandler struct {
}

func (h EmptyHandler) UseDB(dbName string) error {
	return nil
}
func (h EmptyHandler) HandleQuery(query string) (*Result, error) {
	return nil, fmt.Errorf("not supported now")
}

func (h EmptyHandler) HandleFieldList(table string, fieldWildcard string) ([]*Field, error) {
	return nil, fmt.Errorf("not supported now")
}
func (h EmptyHandler) HandleStmtPrepare(query string) (int, int, interface{}, error) {
	return 0, 0, nil, fmt.Errorf("not supported now")
}
func (h EmptyHandler) HandleStmtExecute(context interface{}, query string, args []interface{}) (*Result, error) {
	return nil, fmt.Errorf("not supported now")
}

func (h EmptyHandler) HandleStmtClose(context interface{}) error {
	return nil
}

func (h EmptyHandler) HandleOtherCommand(cmd byte, data []byte) error {
	return NewError(
		ER_UNKNOWN_ERROR,
		fmt.Sprintf("command %d is not supported now", cmd),
	)
}
