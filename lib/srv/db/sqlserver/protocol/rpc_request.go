/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/gravitational/trace"
	mssql "github.com/microsoft/go-mssqldb"
)

// procIDToName maps procID to the special stored procedure name
// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/619c43b6-9495-4a58-9e49-a4950db245b3
var procIDToName = []string{
	1:  "Sp_Cursor",
	2:  "Sp_CursorOpen",
	3:  "Sp_CursorPrepare",
	4:  "Sp_CursorExecute",
	5:  "Sp_CursorPrepExec",
	6:  "Sp_CursorUnprepare",
	7:  "Sp_CursorFetch",
	8:  "Sp_CursorOption",
	9:  "Sp_CursorClose",
	10: "Sp_ExecuteSql",
	11: "Sp_Prepare",
	12: "Sp_Execute",
	13: "Sp_PrepExec",
	14: "Sp_PrepExecRpc",
	15: "Sp_Unprepare",
}

// RPCRequest defines client RPC Request packet:
// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/619c43b6-9495-4a58-9e49-a4950db245b3
type RPCRequest struct {
	Packet
	// ProcName contains name of the procedure to be executed.
	ProcName string
	// Parameters contains list of RPC parameters.
	Parameters []string
}

func toRPCRequest(p Packet) (*RPCRequest, error) {
	if p.Type() != PacketTypeRPCRequest {
		return nil, trace.BadParameter("expected SQLBatch packet, got: %#v", p.Type())
	}
	data := p.Data()
	r := bytes.NewReader(p.Data())

	var headersLength uint32
	if err := binary.Read(r, binary.LittleEndian, &headersLength); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := r.Seek(int64(headersLength), io.SeekStart); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	var length uint16
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, trace.Wrap(err)
	}

	var procName string
	var err error
	// If the first USHORT contains 0xFFFF the following USHORT contains the PROCID.
	// Otherwise, NameLenProcID contains the parameter name length and parameter name.
	if length == procIDSwitchRPCRequest {
		var procID uint16
		if err := binary.Read(r, binary.LittleEndian, &procID); err != nil {
			return nil, trace.Wrap(err)
		}
		procName, err = getProcName(procID)
		if err != nil {
			return nil, trace.BadParameter("failed to get procedure name")
		}
	} else {
		procName, err = readUcs2(r, 2*int(length))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var flags uint16
	if err := binary.Read(r, binary.LittleEndian, &flags); err != nil {
		return nil, trace.Wrap(err)
	}

	// offset the reader by 2 bytes.
	if _, err := r.Seek(2, io.SeekCurrent); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	tds := mssql.NewTdsBuffer(data[int(r.Size())-r.Len():], r.Len())
	ti := mssql.ReadTypeInfo(tds)
	val := ti.Reader(&ti, tds)

	return &RPCRequest{
		Packet:     p,
		ProcName:   procName,
		Parameters: getParameters(val),
	}, nil
}

func getParameters(val interface{}) []string {
	if val == nil {
		return nil
	}
	return []string{fmt.Sprintf("%v", val)}

}

func getProcName(procID uint16) (string, error) {
	if int(procID) >= len(procIDToName) {
		return "unknownProc", nil
	}

	var procName string
	if procName = procIDToName[procID]; procName == "" {
		return "", trace.BadParameter("unmapped procID")
	}
	return procName, nil
}
