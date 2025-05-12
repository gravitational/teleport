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

use crate::{
    hash_index, rgb565_to_888, QOI_OP_DIFF, QOI_OP_EXTENDED_RUN, QOI_OP_INDEX, QOI_OP_LUMA,
    QOI_OP_RGB, QOI_OP_RUN,
};
use std::iter;

const EXTENDED_RUN_BIAS_1: usize = 31;
const EXTENDED_RUN_BIAS_2: usize = (1 << 7) - EXTENDED_RUN_BIAS_1;
const EXTENDED_RUN_BIAS_3: usize = EXTENDED_RUN_BIAS_2 + (1 << 14);
const EXTENDED_RUN_BIAS_4: usize = EXTENDED_RUN_BIAS_3 + (1 << 21);
const EXTENDED_RUN_BIAS_5: usize = EXTENDED_RUN_BIAS_4 + (1 << 28);

const QOI_OP_INDEX_END: u8 = QOI_OP_INDEX | 0x3f;
const QOI_OP_RUN_END: u8 = QOI_OP_RUN | 0x1d; // <- note, 0x1d (not 0x1f)
const QOI_OP_DIFF_END: u8 = QOI_OP_DIFF | 0x3f;
const QOI_OP_LUMA_END: u8 = QOI_OP_LUMA | 0x1f;

pub fn decode(data: &[u8], v: &mut Vec<u8>) {
    let mut data = data;

    let mut index = [[0u8, 0, 0]; 64];
    let mut px = [0u8, 0, 0, 0xFF];
    let mut px565 = [0u8, 0, 0];

    while !data.is_empty() {
        match data {
            [b1 @ 0..QOI_OP_INDEX, b2, dtail @ ..] => {
                px565 = [b1 >> 3, ((b1 & 7) << 3) + (b2 >> 5), b2 & 31];
                px = rgb565_to_888(&px565);
                v.extend_from_slice(&px);
                data = dtail;
            }
            [b1 @ QOI_OP_INDEX..=QOI_OP_INDEX_END, dtail @ ..] => {
                px565 = index[(*b1 ^ QOI_OP_INDEX) as usize];
                px = rgb565_to_888(&px565);
                v.extend_from_slice(&px);
                data = dtail;
                continue;
            }
            [QOI_OP_RGB, b1, b2, dtail @ ..] => {
                px565 = [b1 >> 3, ((b1 & 7) << 3) + (b2 >> 5), b2 & 31];
                px = rgb565_to_888(&px565);
                v.extend_from_slice(&px);
                data = dtail;
            }
            [b1 @ QOI_OP_RUN..=QOI_OP_RUN_END, dtail @ ..] => {
                let run = (b1 & 0x1f) as usize + 1;
                v.extend(iter::repeat_n(&px, run).flat_map(|s| s.iter()));
                data = dtail;
                continue;
            }
            [QOI_OP_EXTENDED_RUN, b1 @ 0..=127, dtail @ ..] => {
                let run = *b1 as usize + EXTENDED_RUN_BIAS_1;
                v.extend(iter::repeat_n(&px, run).flat_map(|s| s.iter()));
                data = dtail;
                continue;
            }
            [QOI_OP_EXTENDED_RUN, b1, b2 @ 0..=127, dtail @ ..] => {
                let run = shift(b1, 0) + shift(b2, 7) - EXTENDED_RUN_BIAS_2;
                v.extend(iter::repeat_n(&px, run).flat_map(|s| s.iter()));
                data = dtail;
                continue;
            }
            [QOI_OP_EXTENDED_RUN, b1, b2, b3 @ 0..=127, dtail @ ..] => {
                let run = shift(b1, 0) + shift(b2, 7) + shift(b3, 14) - EXTENDED_RUN_BIAS_3;
                v.extend(iter::repeat_n(&px, run).flat_map(|s| s.iter()));
                data = dtail;
                continue;
            }
            [QOI_OP_EXTENDED_RUN, b1, b2, b3, b4 @ 0..=127, dtail @ ..] => {
                let run = shift(b1, 0) + shift(b2, 7) + shift(b3, 14) + shift(b4, 21)
                    - EXTENDED_RUN_BIAS_4;
                v.extend(iter::repeat_n(&px, run).flat_map(|s| s.iter()));
                data = dtail;
                continue;
            }
            [QOI_OP_EXTENDED_RUN, b1, b2, b3, b4, b5, dtail @ ..] => {
                let run =
                    shift(b1, 0) + shift(b2, 7) + shift(b3, 14) + shift(b4, 21) + shift(b5, 28)
                        - EXTENDED_RUN_BIAS_5;
                v.extend(iter::repeat_n(&px, run).flat_map(|s| s.iter()));
                data = dtail;
                continue;
            }
            [b1 @ QOI_OP_DIFF..=QOI_OP_DIFF_END, dtail @ ..] => {
                px565 = update_diff(&px565, *b1);
                px = rgb565_to_888(&px565);
                v.extend_from_slice(&px);
                data = dtail;
            }
            [b1 @ QOI_OP_LUMA..=QOI_OP_LUMA_END, b2, dtail @ ..] => {
                px565 = update_luma(&px565, *b1, *b2);
                px = rgb565_to_888(&px565);
                v.extend_from_slice(&px);
                data = dtail;
            }
            _ => {
                panic!("unexpected")
            }
        }
        index[hash_index(&px565) as usize] = px565;
    }
}

#[inline]
fn shift(b: &u8, shift: usize) -> usize {
    (*b as usize) << shift
}

#[inline]
fn update_diff(data: &[u8], b1: u8) -> [u8; 3] {
    [
        data[0].wrapping_add((b1 >> 4) & 0x03).wrapping_sub(2) & 31,
        data[1].wrapping_add((b1 >> 2) & 0x03).wrapping_sub(2) & 63,
        data[2].wrapping_add(b1 & 0x03).wrapping_sub(2) & 31,
    ]
}

#[inline]
fn update_luma(data: &[u8], b1: u8, b2: u8) -> [u8; 3] {
    let vg = b1.wrapping_sub(16);
    let vg_8 = vg.wrapping_sub(8);
    let vr = vg_8.wrapping_add(b2 >> 4);
    let vb = vg_8.wrapping_add(b2 & 0x0f);
    [
        data[0].wrapping_add(vr) % 32,
        data[1].wrapping_add(vg) % 64,
        data[2].wrapping_add(vb) % 32,
    ]
}

#[cfg(test)]
mod test {
    use super::*;

    fn test_decode_iter<T>(input: &[u8], output: T)
    where
        T: IntoIterator<Item = [u8; 4]>,
    {
        let out: Vec<u8> = output.into_iter().flat_map(|s| s.into_iter()).collect();
        test_decode(input, &out);
    }

    fn test_decode(input: &[u8], output: &[u8]) {
        let mut v = vec![];
        decode(input, &mut v);
        assert_eq!(v, output)
    }

    #[test]
    pub fn short_run() {
        test_decode_iter(&[0xe3], iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 4));
    }

    #[test]
    pub fn max_short_run() {
        test_decode_iter(&[0xfd], iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 30));
    }

    #[test]
    pub fn extended_run1() {
        test_decode_iter(
            &[0xfe, 0x01],
            iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 30 + 2),
        );
    }

    #[test]
    pub fn extended_run2() {
        test_decode_iter(
            &[0xfe, 0x81, 0x01],
            iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 30 + 128 + 2),
        );
    }

    #[test]
    pub fn diff() {
        test_decode(&[0xab], &[0x00, 0x00, 0x08, 0xff]);
    }

    #[test]
    pub fn small_red() {
        test_decode(&[0x00, 155], &rgb565_to_888(&[0x00, 0x04, 0x1b]));
    }

    #[test]
    pub fn luma() {
        test_decode(&[0xd8, 0x88], &rgb565_to_888(&[0x08, 0x08, 0x08, 0xff]));
    }

    #[test]
    pub fn rgb() {
        test_decode(&[0xff, 0xc3, 0x18], &rgb565_to_888(&[0x18, 0x18, 0x18]));
    }

    #[test]
    pub fn index() {
        test_decode(
            &[0xab, 0x40],
            &[0x00, 0x00, 0x08, 0xff, 0x00, 0x00, 0x00, 0xff],
        );
    }
}
