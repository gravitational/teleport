/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Each process creates its own key pair. The public key is saved to disk under the specified
// filename, the private key stays in the memory.
//
// `Renderer` and `Tshd` file names are also used on the tshd side.
export enum GrpcCertName {
  Renderer = 'renderer.crt',
  Tshd = 'tshd.crt',
  Shared = 'shared.crt',
}
