/**
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

import { GrpcTransport } from '@protobuf-ts/grpc-transport';

import { createInsecureClientCredentials } from 'teleterm/services/grpcCredentials';

import { createTshdClient } from './createClient';

// This test detects situations where one of the generated JS protobufs has missing dependencies.
// Dependencies must be provided as `--path` values to `buf generate` in build.assets/genproto.sh.
test('generated protos import necessary dependencies', () => {
  expect(() => {
    createTshdClient(
      new GrpcTransport({
        host: 'localhost:1337',
        channelCredentials: createInsecureClientCredentials(),
      })
    );
  }).not.toThrow();
});
