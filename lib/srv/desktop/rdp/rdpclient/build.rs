// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

fn main() {
    // the cwd of build scripts is the root of the crate
    let bindings = cbindgen::Builder::new()
        .with_crate(".")
        .with_language(cbindgen::Language::C)
        .generate()
        .unwrap();

    // atomically swap the header in place, just in case there's multiple
    // compilations at the same time
    let out = tempfile::NamedTempFile::new_in(".").unwrap();
    bindings.write(&out);

    // TODO(espadolini): target-specific paths. Ideally we want the header in
    // target/arch-vendor-os/release/ so that the Go side can refer to it like
    // it refers to the .a, but the only way I found so far is to use
    // ${OUT_DIR}/../../ which isn't really guaranteed to work, afaict.
    out.persist("librdprs.h").unwrap();
}
