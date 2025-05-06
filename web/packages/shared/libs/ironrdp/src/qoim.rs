use std::iter;

const QOI_OP_INDEX: u8 = 0x40; //           01xxxxxx
const QOI_OP_DIFF: u8 = 0x80; //            10xxxxxx
const QOI_OP_LUMA: u8 = 0xc0; //            110xxxxx
const QOI_OP_RUN: u8 = 0xe0; //             111xxxxx
const QOI_OP_EXTENDED_RUN: u8 = 0xfe; //    11111110
const QOI_OP_RGB: u8 = 0xff; //             11111111

const EXTENDED_RUN_BIAS_1: usize = 31;
const EXTENDED_RUN_BIAS_2: usize = (1 << 7) - EXTENDED_RUN_BIAS_1;
const EXTENDED_RUN_BIAS_3: usize = EXTENDED_RUN_BIAS_2 + (1 << 14);
const EXTENDED_RUN_BIAS_4: usize = EXTENDED_RUN_BIAS_3 + (1 << 21);
const EXTENDED_RUN_BIAS_5: usize = EXTENDED_RUN_BIAS_4 + (1 << 28);

fn hash_index(data: &[u8]) -> u8 {
    (data[0] ^ data[1] ^ data[2]) % 64
}

const QOI_OP_INDEX_END: u8 = QOI_OP_INDEX | 0x3f;
const QOI_OP_RUN_END: u8 = QOI_OP_RUN | 0x1d; // <- note, 0x1d (not 0x1f)
const QOI_OP_DIFF_END: u8 = QOI_OP_DIFF | 0x3f;
const QOI_OP_LUMA_END: u8 = QOI_OP_LUMA | 0x1f;

pub(crate) fn decode(data: &[u8], v: &mut Vec<u8>) {
    let mut data = data;

    let mut index = [[0u8, 0, 0, 0xFF]; 64];
    let mut px = [0u8, 0, 0, 0xFF];
    let mut px565 = [0u8, 0, 0];

    while data.len() > 0 {
        match data {
            [b1 @ 0..QOI_OP_INDEX, b2, dtail @ ..] => {
                px565 = [b1 >> 3, (b1 & 7) + (b2 >> 5), b2 & 63];
                px = rgb565_to_888(&px565);
                v.extend_from_slice(&px);
                data = dtail;
            }
            [b1 @ QOI_OP_INDEX..=QOI_OP_INDEX_END, dtail @ ..] => {
                px = index[(*b1 ^ QOI_OP_INDEX) as usize];
                v.extend_from_slice(&px);
                data = dtail;
                continue;
            }
            [QOI_OP_RGB, b1, b2, dtail @ ..] => {
                px565 = [b1 >> 3, ((b1 & 7) << 3) + (b2 >> 5), b2 & 63];
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
                px565 = update_diff(&px, *b1);
                px = rgb565_to_888(&px565);
                v.extend_from_slice(&px);
                data = dtail;
            }
            [b1 @ QOI_OP_LUMA..=QOI_OP_LUMA_END, b2, dtail @ ..] => {
                px565 = update_luma(&px, *b1, *b2);
                px = rgb565_to_888(&px565);
                v.extend_from_slice(&px);
                data = dtail;
            }
            _ => {
                panic!("unexpected")
            }
        }
        index[hash_index(&px565) as usize] = px;
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
    let vg = (b1 & 0x1f).wrapping_sub(16);
    let vg_8 = vg.wrapping_sub(8);
    let vr = vg_8.wrapping_add((b2 >> 4) & 0x0f);
    let vb = vg_8.wrapping_add(b2 & 0x0f);
    [
        data[0].wrapping_add(vr) & 31,
        data[1].wrapping_add(vg) & 63,
        data[2].wrapping_add(vb) & 31,
    ]
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

    #[test]
    pub fn test_encode() {
        struct Case<'a>((&'a str, &'a [u8], &'a [u8]));
        for tt in [
            Case((
                "short run",
                &[0xe3],
                &iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 4)
                    .flat_map(|s| s.into_iter())
                    .collect::<Vec<u8>>(),
            )),
            Case((
                "max short run",
                &[0xfd],
                &iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 30)
                    .flat_map(|s| s.into_iter())
                    .collect::<Vec<u8>>(),
            )),
            Case((
                "extended run 1",
                &[0xfe, 0x01],
                &iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 30 + 2)
                    .flat_map(|s| s.into_iter())
                    .collect::<Vec<u8>>(),
            )),
            Case((
                "extended run 2",
                &[0xfe, 0x81, 0x01],
                &iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 30 + 128 + 2)
                    .flat_map(|s| s.into_iter())
                    .collect::<Vec<u8>>(),
            )),
            Case((
                "extended run 3",
                &[0xfe, 0xd1, 0x04],
                &iter::repeat_n([0x00u8, 0x00, 0x00, 0xff], 624)
                    .flat_map(|s| s.into_iter())
                    .collect::<Vec<u8>>(),
            )),
            Case(("diff", &[0xab], &[0x00, 0x00, 0x01, 0xff])),
            Case(("small red", &[0x00, 155], &[0x00, 0x04, 0x1b, 0xff])),
            Case(("luma", &[0xd8, 0x88], &[0x08, 0x08, 0x08, 0xff])),
            Case(("rgb", &[0xff, 0xc3, 0x18], &[0x18, 0x18, 0x18, 0xff])),
            Case((
                "index",
                &[0xab, 0x40],
                &[0x00, 0x00, 0x01, 0xff, 0x00, 0x00, 0x00, 0xff],
            )),
        ] {
            let mut v = vec![];
            decode(tt.0 .1, &mut v);
            assert_eq!(v, tt.0 .2, "{}", tt.0 .0)
        }
    }
}
