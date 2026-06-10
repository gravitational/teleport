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

use ironrdp_graphics::pointer::DecodedPointer;
use std::sync::Arc;

#[derive(Default)]
pub(crate) enum CursorBitmap {
    #[default]
    Default,
    Server(Arc<DecodedPointer>),
}

impl CursorBitmap {
    pub(crate) fn data(&self) -> &[u8] {
        match self {
            CursorBitmap::Default => &DEFAULT_CURSOR_RGBA,
            CursorBitmap::Server(ptr) => &ptr.bitmap_data,
        }
    }

    pub(crate) fn dimensions(&self) -> (u16, u16) {
        match self {
            CursorBitmap::Default => (DEFAULT_CURSOR_WIDTH as u16, DEFAULT_CURSOR_HEIGHT as u16),
            CursorBitmap::Server(ptr) => (ptr.width, ptr.height),
        }
    }

    pub(crate) fn hotspot(&self) -> (u16, u16) {
        match self {
            CursorBitmap::Default => (0, 0),
            CursorBitmap::Server(ptr) => (ptr.hotspot_x, ptr.hotspot_y),
        }
    }
}

#[derive(Default)]
pub(crate) struct CursorState {
    visible: bool,
    x: u16,
    y: u16,
    bitmap: CursorBitmap,
}

impl CursorState {
    pub(crate) fn set_bitmap(&mut self, pointer: Arc<DecodedPointer>) {
        if pointer.bitmap_data.is_empty() {
            return;
        }

        self.bitmap = CursorBitmap::Server(pointer);
    }

    pub(crate) fn clear_bitmap(&mut self) {
        self.bitmap = CursorBitmap::Default;
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

    pub(crate) fn bitmap(&self) -> &CursorBitmap {
        &self.bitmap
    }
}

// Synthetic default cursor used when the server signals `PTR_DEFAULT` (no
// bitmap supplied) or when a PointerPosition arrives before any bitmap.
const DEFAULT_CURSOR_WIDTH: usize = 12;
const DEFAULT_CURSOR_HEIGHT: usize = 17;

#[rustfmt::skip]
const DEFAULT_CURSOR_SHAPE: [u8; DEFAULT_CURSOR_WIDTH*DEFAULT_CURSOR_HEIGHT] = [
    1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 2, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0,
    1, 2, 2, 2, 1, 0, 0, 0, 0, 0, 0, 0,
    1, 2, 2, 2, 2, 1, 0, 0, 0, 0, 0, 0,
    1, 2, 2, 2, 2, 2, 1, 0, 0, 0, 0, 0,
    1, 2, 2, 2, 2, 2, 2, 1, 0, 0, 0, 0,
    1, 2, 2, 2, 2, 2, 2, 2, 1, 0, 0, 0,
    1, 2, 2, 2, 2, 2, 2, 2, 2, 1, 0, 0,
    1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1, 0,
    1, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1,
    1, 2, 2, 2, 1, 2, 2, 1, 0, 0, 0, 0,
    1, 2, 2, 1, 0, 1, 2, 2, 1, 0, 0, 0,
    1, 2, 1, 0, 0, 1, 2, 2, 1, 0, 0, 0,
    1, 1, 0, 0, 0, 0, 1, 2, 1, 0, 0, 0,
    1, 0, 0, 0, 0, 0, 1, 1, 0, 0, 0, 0,
];

const DEFAULT_CURSOR_RGBA: [u8; DEFAULT_CURSOR_WIDTH * DEFAULT_CURSOR_HEIGHT * 4] = {
    let mut data = [0; DEFAULT_CURSOR_WIDTH * DEFAULT_CURSOR_HEIGHT * 4];
    let mut i = 0;
    while i < DEFAULT_CURSOR_WIDTH * DEFAULT_CURSOR_HEIGHT {
        let p = match DEFAULT_CURSOR_SHAPE[i] {
            // black outline
            1 => [0, 0, 0, 255],
            // white body
            2 => [255, 255, 255, 255],
            _ => [0, 0, 0, 0],
        };

        data[4 * i] = p[0];
        data[4 * i + 1] = p[1];
        data[4 * i + 2] = p[2];
        data[4 * i + 3] = p[3];
        i += 1;
    }

    data
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
        let (width, height) = bitmap.dimensions();

        assert_eq!(width, DEFAULT_CURSOR_WIDTH as u16);
        assert_eq!(height, DEFAULT_CURSOR_HEIGHT as u16);

        let (hotspot_x, hotspot_y) = bitmap.hotspot();

        assert_eq!(hotspot_x, 0);
        assert_eq!(hotspot_y, 0);

        let data = bitmap.data();

        assert_eq!(data, &DEFAULT_CURSOR_RGBA[..]);
    }

    #[test]
    fn pointer_default_clears_cached_bitmap() {
        let mut state = CursorState::default();

        assert_is_default(state.bitmap());

        state.set_bitmap(Arc::new(sample_pointer()));

        let cached = state.bitmap();
        let (width, height) = cached.dimensions();

        assert_eq!(width, 2);
        assert_eq!(height, 3);

        let (hotspot_x, hotspot_y) = cached.hotspot();

        assert_eq!(hotspot_x, 1);
        assert_eq!(hotspot_y, 2);

        let data = cached.data();
        assert_eq!(data, &vec![0xAB; 2 * 3 * 4][..]);

        state.set_visible(true);
        state.clear_bitmap();

        assert_is_default(state.bitmap());
        assert!(state.is_visible());
    }
}
