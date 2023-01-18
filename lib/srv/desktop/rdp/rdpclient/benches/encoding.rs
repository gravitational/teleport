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

use criterion::{criterion_group, criterion_main, BatchSize, Criterion};

use rdp::core::event::BitmapEvent;
use rdp_client::encode_png;

pub fn criterion_benchmark(c: &mut Criterion) {
    let bitmap = BitmapEvent {
        dest_left: 0,
        dest_right: 0,
        dest_bottom: 0,
        dest_top: 0,
        width: 64,
        height: 64,
        is_compress: true,
        bpp: 32,
        data: std::fs::read("./benches/testdata/bitmap.in").unwrap(),
    };

    let d = bitmap.decompress().unwrap();
    let mut result = vec![0; 10240];

    c.bench_function("encode", move |b| {
        b.iter_batched(
            || d.clone(),
            |d| encode_png(&mut result, 64, 64, d),
            BatchSize::SmallInput,
        )
    });
}

criterion_group!(benches, criterion_benchmark);
criterion_main!(benches);
