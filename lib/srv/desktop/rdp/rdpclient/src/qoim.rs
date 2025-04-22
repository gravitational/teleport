use std::iter;

const QOI_OP_INDEX: u8 = 0x00; // 00xxxxxx
const QOI_OP_DIFF: u8 = 0x40; // 01xxxxxx
const QOI_OP_LUMA: u8 = 0x80; // 10xxxxxx
const QOI_OP_RUN: u8 = 0xc0; // 11xxxxxx
const QOI_OP_RGB: u8 = 0xfe; // 11111110
const QOI_OP_EXTENDED_RUN: u8 = 0xff; // 11111111

const EXTENDED_RUN_BIAS_1: usize = 63;
const EXTENDED_RUN_BIAS_2: usize = (1 << 7) - EXTENDED_RUN_BIAS_1;
const EXTENDED_RUN_BIAS_3: usize = EXTENDED_RUN_BIAS_2 + (1 << 14);
const EXTENDED_RUN_BIAS_4: usize = EXTENDED_RUN_BIAS_3 + (1 << 21);
const EXTENDED_RUN_BIAS_5: usize = EXTENDED_RUN_BIAS_4 + (1 << 28);

pub(crate) fn encode(v: &mut Vec<u8>, data: &[u8]) {
    let mut px_prev = [0u8, 0, 0, 0xFF].as_slice();
    let mut hash_prev = hash_index(px_prev);
    let mut index = [[0u8, 0, 0, 0xFF].as_slice(); 64];
    let mut index_allowed = false;
    let mut run = 0u32;
    for px in data.chunks_exact(4) {
        if px == px_prev {
            run += 1;
        } else {
            if run != 0 {
                push_run(v, hash_prev, index_allowed, run);
                run = 0;
            }
            index_allowed = true;
            hash_prev = hash_index(px);

            let index_px = &mut index[hash_prev as usize];
            if *index_px == px {
                v.push(QOI_OP_INDEX | hash_prev);
            } else {
                *index_px = px;
                encode_pixel(px, px_prev, v);
            }
            px_prev = px;
        }
    }
    if run != 0 {
        push_run(v, hash_prev, index_allowed, run);
    }
}

#[inline]
fn push_run(mut out: &mut Vec<u8>, hash_prev: u8, index_allowed: bool, run: u32) {
    if run == 1 && index_allowed {
        out.push(QOI_OP_INDEX | hash_prev);
    } else if run > 62 {
        out.push(QOI_OP_EXTENDED_RUN);
        encode_vint(&mut out, run - 63);
    } else {
        out.push(QOI_OP_RUN | (run as u8 - 1))
    }
}

fn hash_index(data: &[u8]) -> u8 {
    (data[0] ^ data[1] ^ data[2]) % 64
}

fn encode_pixel(data: &[u8], px_prev: &[u8], buf: &mut Vec<u8>) -> () {
    let vg = data[1].wrapping_sub(px_prev[1]);
    let vg_32 = vg.wrapping_add(32);
    if vg_32 | 63 == 63 {
        let vr = data[2].wrapping_sub(px_prev[2]);
        let vb = data[0].wrapping_sub(px_prev[0]);
        let vg_r = vr.wrapping_sub(vg);
        let vg_b = vb.wrapping_sub(vg);
        let (vr_2, vg_2, vb_2) = (vr.wrapping_add(2), vg.wrapping_add(2), vb.wrapping_add(2));
        if vr_2 | vg_2 | vb_2 | 3 == 3 {
            buf.push(QOI_OP_DIFF | (vr_2 << 4) | (vg_2 << 2) | vb_2);
        } else {
            let (vg_r_8, vg_b_8) = (vg_r.wrapping_add(8), vg_b.wrapping_add(8));
            if vg_r_8 | vg_b_8 | 15 == 15 {
                buf.extend_from_slice(&[QOI_OP_LUMA | vg_32, (vg_r_8 << 4) | vg_b_8]);
            } else {
                buf.extend_from_slice(&[QOI_OP_RGB, data[2], data[1], data[0]])
            }
        }
    } else {
        buf.extend_from_slice(&[QOI_OP_RGB, data[2], data[1], data[0]]);
    }
}

pub fn encode_vint(output: &mut Vec<u8>, mut value: u32) -> u8 {
    let do_one = |output: &mut Vec<u8>, value: &mut u32| {
        output.push(((*value & 127) | 128) as u8);
        *value >>= 7;
    };
    let do_last = |output: &mut Vec<u8>, value: u32| {
        output.push((value & 127) as u8);
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

const QOI_OP_INDEX_END: u8 = QOI_OP_INDEX | 0x3f;
const QOI_OP_RUN_END: u8 = QOI_OP_RUN | 0x3d; // <- note, 0x3d (not 0x3f)
const QOI_OP_DIFF_END: u8 = QOI_OP_DIFF | 0x3f;
const QOI_OP_LUMA_END: u8 = QOI_OP_LUMA | 0x3f;

fn decode(data: &[u8]) -> Vec<u8> {
    let mut v = Vec::with_capacity(1920 * 1080 * 4);
    let mut data = data;

    let mut index = [[0u8, 0, 0, 0xFF]; 64];
    let mut px = [0u8, 0, 0, 0xFF];

    while data.len() > 0 {
        match data {
            [b1 @ QOI_OP_INDEX..=QOI_OP_INDEX_END, dtail @ ..] => {
                px = index[*b1 as usize];
                v.extend_from_slice(&px);
                data = dtail;
                continue;
            }
            [QOI_OP_RGB, r, g, b, dtail @ ..] => {
                px = [*r, *g, *b, 0xFF];
                v.extend_from_slice(&px);
                data = dtail;
            }
            [b1 @ QOI_OP_RUN..=QOI_OP_RUN_END, dtail @ ..] => {
                let run = (b1 & 0x3f) as usize + 1;
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
                for i in 0..run {}
                v.extend(iter::repeat_n(&px, run).flat_map(|s| s.iter()));
                data = dtail;
                continue;
            }
            [b1 @ QOI_OP_DIFF..=QOI_OP_DIFF_END, dtail @ ..] => {
                px = update_diff(&px, *b1);
                v.extend_from_slice(&px);
                data = dtail;
            }
            [b1 @ QOI_OP_LUMA..=QOI_OP_LUMA_END, b2, dtail @ ..] => {
                px = update_luma(&px, *b1, *b2);
                v.extend_from_slice(&px);
                data = dtail;
            }
            _ => {
                panic!("unexpected")
            }
        }
        index[hash_index(&px) as usize] = px;
    }

    v
}

fn shift(b: &u8, shift: usize) -> usize {
    (*b as usize) << shift
}

#[inline]
pub fn update_diff(data: &[u8], b1: u8) -> [u8; 4] {
    [
        data[0].wrapping_add((b1 >> 4) & 0x03).wrapping_sub(2),
        data[1].wrapping_add((b1 >> 2) & 0x03).wrapping_sub(2),
        data[2].wrapping_add(b1 & 0x03).wrapping_sub(2),
        0xFF,
    ]
}

#[inline]
pub fn update_luma(data: &[u8], b1: u8, b2: u8) -> [u8; 4] {
    let vg = (b1 & 0x3f).wrapping_sub(32);
    let vg_8 = vg.wrapping_sub(8);
    let vr = vg_8.wrapping_add((b2 >> 4) & 0x0f);
    let vb = vg_8.wrapping_add(b2 & 0x0f);
    [
        data[0].wrapping_add(vr),
        data[1].wrapping_add(vg),
        data[2].wrapping_add(vb),
        0xFF,
    ]
}

#[cfg(test)]
mod test {
    use crate::qoim::{decode, encode};
    use std::iter;

    #[test]
    fn encode_long_run() -> () {
        for run in [128, 1 << 14, 1 << 21] {
            let mut encoded = Vec::with_capacity(20);
            let data = vec![0xFFu8; 4 * run];
            encode(&mut encoded, &data);
            let decoded = decode(&encoded);
            assert_eq!(data.len(), decoded.len(), "run={}", run);
            assert_eq!(data, decoded, "run={}", run);
        }
    }

    #[test]
    fn encode_short_run() -> () {
        let mut encoded = Vec::with_capacity(20);
        let data = vec![0xFFu8; 4 * 60];
        encode(&mut encoded, &data);
        let decoded = decode(&encoded);
        assert_eq!(data.len(), decoded.len());
        assert_eq!(data, decoded);
    }

    #[test]
    fn encode_index() -> () {
        let mut encoded = Vec::with_capacity(20);
        let data: Vec<u8> = iter::repeat_n([1u8, 0u8, 0u8, 0xFF, 2u8, 0u8, 0u8, 0xFF], 50)
            .flat_map(|a| a.into_iter())
            .collect();
        let expected: Vec<u8> = iter::repeat_n([0u8, 0u8, 1u8, 0xFF, 0u8, 0u8, 2u8, 0xFF], 50)
            .flat_map(|a| a.into_iter())
            .collect();
        encode(&mut encoded, &data);
        let decoded = decode(&encoded);
        assert_eq!(expected.len(), decoded.len());
        assert_eq!(expected, decoded);
    }
}
