/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package sshutils

import (
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type DirectTCPIPReq struct {
	Host string
	Port uint32

	Orig     string
	OrigPort uint32
}

func ParseDirectTCPIPReq(data []byte) (*DirectTCPIPReq, error) {
	var r DirectTCPIPReq
	if err := ssh.Unmarshal(data, &r); err != nil {
		log.Infof("failed to parse Direct TCP IP request: %v", err)
		return nil, err
	}
	return &r, nil
}
