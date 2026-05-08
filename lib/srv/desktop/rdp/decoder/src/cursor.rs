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

use std::borrow::Cow;

pub(crate) struct CursorBitmap {
    pub(crate) width: u16,
    pub(crate) height: u16,
    pub(crate) hotspot_x: u16,
    pub(crate) hotspot_y: u16,
    pub(crate) data: Cow<'static, [u8]>,
}

#[derive(Default)]
pub(crate) struct CursorState {
    visible: bool,
    x: u16,
    y: u16,
    bitmap: Option<CursorBitmap>,
}

impl CursorState {
    pub(crate) fn set_bitmap(&mut self, pointer: &ironrdp_graphics::pointer::DecodedPointer) {
        if pointer.bitmap_data.is_empty() {
            return;
        }

        self.bitmap = Some(CursorBitmap {
            width: pointer.width,
            height: pointer.height,
            hotspot_x: pointer.hotspot_x,
            hotspot_y: pointer.hotspot_y,
            data: Cow::Owned(pointer.bitmap_data.clone()),
        });
    }

    pub(crate) fn clear_bitmap(&mut self) {
        self.bitmap = None;
    }

    pub(crate) fn move_cursor(&mut self, x: u16, y: u16) {
        self.x = x;
        self.y = y;
    }

    pub(crate) fn set_visible(&mut self, visible: bool) {
        self.visible = visible;
    }

    pub(crate) fn is_visible(&self) -> bool {
        self.visible
    }

    pub(crate) fn position(&self) -> (u16, u16) {
        (self.x, self.y)
    }

    pub(crate) fn bitmap_or_default(&self) -> &CursorBitmap {
        self.bitmap.as_ref().unwrap_or(&DEFAULT_CURSOR_BITMAP)
    }
}

// Synthetic default cursor used when the server signals `PTR_DEFAULT` (no
// bitmap supplied) or when a PointerPosition arrives before any bitmap.
const DEFAULT_CURSOR_WIDTH: usize = 12;
const DEFAULT_CURSOR_HEIGHT: usize = 17;

#[rustfmt::skip]
const DEFAULT_CURSOR_SHAPE: [[u8; DEFAULT_CURSOR_WIDTH]; DEFAULT_CURSOR_HEIGHT] = [
    [1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
    [1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
    [1, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0],
    [1, 2, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0],
    [1, 2, 2, 2, 1, 0, 0, 0, 0, 0, 0, 0],
    [1, 2, 2, 2, 2, 1, 0, 0, 0, 0, 0, 0],
    [1, 2, 2, 2, 2, 2, 1, 0, 0, 0, 0, 0],
    [1, 2, 2, 2, 2, 2, 2, 1, 0, 0, 0, 0],
    [1, 2, 2, 2, 2, 2, 2, 2, 1, 0, 0, 0],
    [1, 2, 2, 2, 2, 2, 2, 2, 2, 1, 0, 0],
    [1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 0],
    [1, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1],
    [1, 2, 2, 2, 1, 2, 2, 1, 0, 0, 0, 0],
    [1, 2, 2, 1, 0, 1, 2, 2, 1, 0, 0, 0],
    [1, 2, 1, 0, 0, 1, 2, 2, 1, 0, 0, 0],
    [1, 1, 0, 0, 0, 0, 1, 2, 1, 0, 0, 0],
    [1, 0, 0, 0, 0, 0, 1, 1, 0, 0, 0, 0],
];

const DEFAULT_CURSOR_RGBA: [u8; DEFAULT_CURSOR_WIDTH * DEFAULT_CURSOR_HEIGHT * 4] = {
    let mut data = [0u8; DEFAULT_CURSOR_WIDTH * DEFAULT_CURSOR_HEIGHT * 4];
    let mut row = 0;

    while row < DEFAULT_CURSOR_HEIGHT {
        let mut col = 0;

        while col < DEFAULT_CURSOR_WIDTH {
            let val = DEFAULT_CURSOR_SHAPE[row][col];

            if val != 0 {
                let idx = (row * DEFAULT_CURSOR_WIDTH + col) * 4;

                if val == 1 {
                    // black outline
                    data[idx + 3] = 255;
                } else {
                    // white body
                    data[idx] = 255;
                    data[idx + 1] = 255;
                    data[idx + 2] = 255;
                    data[idx + 3] = 255;
                }
            }
            col += 1;
        }
        row += 1;
    }

    data
};

static DEFAULT_CURSOR_BITMAP: CursorBitmap = CursorBitmap {
    width: DEFAULT_CURSOR_WIDTH as u16,
    height: DEFAULT_CURSOR_HEIGHT as u16,
    hotspot_x: 0,
    hotspot_y: 0,
    data: Cow::Borrowed(&DEFAULT_CURSOR_RGBA),
};

#[cfg(test)]
mod tests {
    use super::*;
    use ironrdp_graphics::pointer::DecodedPointer;

    fn sample_pointer() -> DecodedPointer {
        DecodedPointer {
            width: 2,
            height: 3,
            hotspot_x: 1,
            hotspot_y: 2,
            bitmap_data: vec![0xAB; 2 * 3 * 4],
        }
    }

    fn assert_is_default(bitmap: &CursorBitmap) {
        assert_eq!(bitmap.width, DEFAULT_CURSOR_WIDTH as u16);
        assert_eq!(bitmap.height, DEFAULT_CURSOR_HEIGHT as u16);
        assert_eq!(bitmap.hotspot_x, 0);
        assert_eq!(bitmap.hotspot_y, 0);
        assert_eq!(&*bitmap.data, &DEFAULT_CURSOR_RGBA[..]);
    }

    #[test]
    fn pointer_default_clears_cached_bitmap() {
        let mut state = CursorState::default();

        assert_is_default(state.bitmap_or_default());

        state.set_bitmap(&sample_pointer());
        let cached = state.bitmap_or_default();
        assert_eq!(cached.width, 2);
        assert_eq!(cached.height, 3);
        assert_eq!(cached.hotspot_x, 1);
        assert_eq!(cached.hotspot_y, 2);
        assert_eq!(&*cached.data, &vec![0xAB; 2 * 3 * 4][..]);

        state.set_visible(true);
        state.clear_bitmap();

        assert_is_default(state.bitmap_or_default());
        assert!(state.is_visible());
    }
}
