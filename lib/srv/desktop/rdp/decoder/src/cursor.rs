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

pub(crate) struct CursorBitmap {
    width: u16,
    height: u16,
    hotspot_x: u16,
    hotspot_y: u16,
    data: Vec<u8>, // RGBA, 4 bytes per pixel
}

impl CursorBitmap {
    pub(crate) unsafe fn write_metadata(
        &self,
        out_width: *mut u16,
        out_height: *mut u16,
        out_hotspot_x: *mut u16,
        out_hotspot_y: *mut u16,
    ) {
        unsafe {
            *out_width = self.width;
            *out_height = self.height;
            *out_hotspot_x = self.hotspot_x;
            *out_hotspot_y = self.hotspot_y;
        }
    }

    pub(crate) fn data_ptr(&self) -> *const u8 {
        self.data.as_ptr()
    }
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
            data: pointer.bitmap_data.clone(),
        });
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

    pub(crate) fn bitmap(&self) -> Option<&CursorBitmap> {
        self.bitmap.as_ref()
    }
}
