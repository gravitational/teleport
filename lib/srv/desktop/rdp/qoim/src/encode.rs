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
    hash_index, QOI_OP_DIFF, QOI_OP_EXTENDED_RUN, QOI_OP_INDEX, QOI_OP_LUMA, QOI_OP_RGB, QOI_OP_RUN,
};

pub fn encode_ptr(data: &[u8], out: *mut u8) -> u64 {
    let out = &mut PointerWriter(0, out);
    encode(data, out);
    out.0
}

pub(crate) fn encode<T>(data: &[u8], out: &mut T)
where
    T: Buf,
{
    let mut px_prev = Pixel::default();
    let mut hash_prev = hash_index(&[0, 0, 0]);
    let mut index = [Pixel::default(); 64];
    let mut run = 0u32;
    for px in data
        .chunks_exact(2)
        .map(|chunk| u16::from_ne_bytes(chunk.try_into().unwrap()))
    {
        if px == px_prev.value {
            run += 1;
        } else {
            let px = decode(px);
            if run != 0 {
                push_run(out, hash_prev, run);
                run = 0;
            }
            hash_prev = hash_index(&[px.r as u8, px.g as u8, px.b as u8]);

            let index_px = &mut index[hash_prev as usize];
            if *index_px == px {
                out.write_byte(QOI_OP_INDEX | hash_prev);
            } else {
                *index_px = px;
                encode_pixel(px, px_prev, out);
            }
            px_prev = px;
        }
    }
    if run != 0 {
        push_run(out, hash_prev, run);
    }
}

#[inline]
fn push_run<T>(out: &mut T, hash_prev: u8, run: u32)
where
    T: Buf,
{
    if run == 1 {
        out.write_byte(QOI_OP_INDEX | hash_prev);
    } else if run > 30 {
        out.write_byte(QOI_OP_EXTENDED_RUN);
        encode_vint(out, run - 31);
    } else {
        out.write_byte(QOI_OP_RUN | (run as u8 - 1))
    }
}

#[inline]
fn encode_pixel<T>(px: Pixel, px_prev: Pixel, out: &mut T)
where
    T: Buf,
{
    let vg = px.g.wrapping_sub(px_prev.g) as u8;
    let vg_16 = vg.wrapping_add(16) % 64;
    if vg_16 | 31 == 31 {
        let vr = px.r.wrapping_sub(px_prev.r) as u8;
        let vb = px.b.wrapping_sub(px_prev.b) as u8;
        let vg_r = vr.wrapping_sub(vg);
        let vg_b = vb.wrapping_sub(vg);
        let vr_2 = vr.wrapping_add(2) % 32;
        let vg_2 = vg.wrapping_add(2) % 64;
        let vb_2 = vb.wrapping_add(2) % 32;

        if vr_2 | vg_2 | vb_2 | 3 == 3 {
            out.write_byte(QOI_OP_DIFF | (vr_2 << 4) | (vg_2 << 2) | vb_2);
            return;
        }
        let (vg_r_8, vg_b_8) = (vg_r.wrapping_add(8) % 32, vg_b.wrapping_add(8) % 32);
        if px.value >= 0x0400 && (vg_r_8 | vg_b_8 | 15 == 15) {
            out.write_byte(QOI_OP_LUMA | vg_16);
            out.write_byte((vg_r_8 << 4) | vg_b_8);
            return;
        }
    }
    if px.value >= 0x0400 {
        out.write_byte(QOI_OP_RGB);
    }
    out.write_byte((px.value >> 8) as u8);
    out.write_byte((px.value & 0xFF) as u8);
}

pub fn encode_vint<T>(output: &mut T, mut value: u32) -> u8
where
    T: Buf,
{
    let do_one = |output: &mut T, value: &mut u32| {
        output.write_byte(((*value & 127) | 128) as u8);
        *value >>= 7;
    };
    let do_last = |output: &mut T, value: u32| {
        output.write_byte((value & 127) as u8);
    };

    if value < 1 << 7 {
        //128
        do_last(output, value);
        1
    } else if value < 1 << 14 {
        do_one(output, &mut value);
        do_last(output, value);
        2
    } else if value < 1 << 21 {
        do_one(output, &mut value);
        do_one(output, &mut value);
        do_last(output, value);
        3
    } else if value < 1 << 28 {
        do_one(output, &mut value);
        do_one(output, &mut value);
        do_one(output, &mut value);
        do_last(output, value);
        4
    } else {
        do_one(output, &mut value);
        do_one(output, &mut value);
        do_one(output, &mut value);
        do_one(output, &mut value);
        do_last(output, value);
        5
    }
}

#[derive(Default, Copy, Clone, Eq, PartialEq)]
struct Pixel {
    value: u16,
    r: u16,
    g: u16,
    b: u16,
}

fn decode(value: u16) -> Pixel {
    Pixel {
        value,
        r: value >> 11,
        g: (value >> 5) % 64,
        b: value % 32,
    }
}

struct PointerWriter(u64, *mut u8);

pub trait Buf {
    fn write_byte(&mut self, data: u8);
}

impl Buf for PointerWriter {
    #[inline]
    fn write_byte(&mut self, data: u8) {
        unsafe {
            *self.1 = data;
            self.1 = self.1.add(1)
        }
        self.0 += 1;
    }
}

impl Buf for Vec<u8> {
    #[inline]
    fn write_byte(&mut self, data: u8) {
        self.push(data);
    }
}
