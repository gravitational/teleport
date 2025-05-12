/*
 * *
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

pub mod decode;
pub mod encode;

const QOI_OP_INDEX: u8 = 0x40; //           01xxxxxx
const QOI_OP_DIFF: u8 = 0x80; //            10xxxxxx
const QOI_OP_LUMA: u8 = 0xc0; //            110xxxxx
const QOI_OP_RUN: u8 = 0xe0; //             111xxxxx
const QOI_OP_EXTENDED_RUN: u8 = 0xfe; //    11111110
const QOI_OP_RGB: u8 = 0xff; //             11111111

#[inline]
fn hash_index(data: &[u8]) -> u8 {
    (data[0] ^ data[1] ^ data[2]) % 64
}

#[inline]
fn rgb565_to_888(data: &[u8]) -> [u8; 4] {
    [
        (((data[0] as u32) * 527 + 23) >> 6) as u8,
        (((data[1] as u32) * 259 + 33) >> 6) as u8,
        (((data[2] as u32) * 527 + 23) >> 6) as u8,
        0xff,
    ]
}

#[cfg(test)]
mod test {
    use super::*;
    use crate::decode::decode;
    use crate::encode::encode;
    use rand::prelude::StdRng;
    use rand::{Rng, SeedableRng};

    fn decode_to_888(px: u16) -> [u8; 4] {
        rgb565_to_888(&[(px >> 11) as u8, ((px >> 5) % 64) as u8, (px % 32) as u8])
    }

    #[test]
    pub fn all_single_values() {
        for i in 0..=65535u16 {
            let mut encoded = vec![];
            let mut decoded = vec![];
            encode(&i.to_ne_bytes(), &mut encoded);
            decode(&encoded, &mut decoded);
            assert_eq!(decode_to_888(i).as_slice(), decoded, "{}", i);
        }
    }

    #[test]
    pub fn double_values() {
        for i in 0..=500u16 {
            for j in 0..=500u16 {
                let mut encoded = vec![];
                let mut decoded = vec![];
                encode(&[i.to_ne_bytes(), j.to_ne_bytes()].concat(), &mut encoded);
                decode(&encoded, &mut decoded);
                assert_eq!(
                    [decode_to_888(i), decode_to_888(j)].concat(),
                    decoded,
                    "{} {}",
                    i,
                    j
                );
            }
        }
    }

    #[test]
    pub fn sequence() {
        let input: Vec<u8> = (0..=65535u16)
            .flat_map(|i| i.to_ne_bytes().into_iter())
            .collect();
        let output: Vec<u8> = (0..=65535u16)
            .flat_map(|i| decode_to_888(i).into_iter())
            .collect();
        let mut encoded = vec![];
        let mut decoded = vec![];
        encode(&input, &mut encoded);
        decode(&encoded, &mut decoded);
        assert_eq!(decoded, output);
    }

    #[test]
    pub fn skipping_sequence() {
        let input: Vec<u8> = (0..=65535u16)
            .skip(5)
            .flat_map(|i| i.to_ne_bytes().into_iter())
            .collect();
        let output: Vec<u8> = (0..=65535u16)
            .step_by(5)
            .flat_map(|i| decode_to_888(i).into_iter())
            .collect();
        let mut encoded = vec![];
        let mut decoded = vec![];
        encode(&input, &mut encoded);
        decode(&encoded, &mut decoded);
        assert_eq!(decoded, output);
    }

    #[test]
    pub fn random() {
        for _ in 0..2000 {
            let seed = rand::random();
            let rng = StdRng::seed_from_u64(seed);
            let input_u16: Vec<u16> = rng.random_iter().take(1000).collect();
            let input: Vec<u8> = input_u16
                .iter()
                .flat_map(|i| i.to_ne_bytes().into_iter())
                .collect();
            let output: Vec<u8> = input_u16
                .iter()
                .flat_map(|i| decode_to_888(*i).into_iter())
                .collect();
            let mut encoded = vec![];
            let mut decoded = vec![];
            encode(&input, &mut encoded);
            decode(&encoded, &mut decoded);
            let pos = decoded.iter().zip(&output).position(|(a, b)| a != b);
            assert_eq!(decoded, output, "seed: {} {:?}", seed, pos);
        }
    }
}
