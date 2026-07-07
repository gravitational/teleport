// Teleport
// Copyright (C) 2026  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

fn main() {
    tonic_prost_build::configure()
        // Disable gRPC server code generation, since only the client code is needed.
        .build_server(false)
        // Skip deriving `Debug` for these types. They contain byte arrays that
        // can clutter the debug output. `Debug` is implemented manually instead.
        .skip_debug([
            "SharedDirectoryRequest.Write",
            "SharedDirectoryResponse.Read",
        ])
        // The `Mfa` variant is significantly larger than all other variants.
        // Box it to reduce the memory footprint of the `Envelope` enum.
        .boxed("Envelope.payload.mfa")
        .compile_protos(
            &["teleport/desktop/v1/desktop_service.proto"],
            &["../../../../../api/proto"],
        )
        .unwrap();
}
