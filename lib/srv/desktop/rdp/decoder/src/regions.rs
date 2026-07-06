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

use ironrdp_pdu::geometry::{InclusiveRectangle, Rectangle};

const MAX_STORED_REGIONS: usize = 256;

#[derive(Default)]
pub(crate) struct UpdatedRegions {
    regions: Vec<InclusiveRectangle>,
}

impl UpdatedRegions {
    pub(crate) fn push(&mut self, r: InclusiveRectangle) {
        if self.regions.len() >= MAX_STORED_REGIONS {
            self.compact();
        }
        self.regions.push(r);
    }

    pub(crate) fn reset(&mut self) {
        self.regions.clear();
    }

    pub(crate) fn len(&self) -> usize {
        self.regions.len()
    }

    pub(crate) fn iter(&self) -> impl Iterator<Item = [u16; 4]> + '_ {
        self.regions
            .iter()
            .map(|r| [r.left, r.top, r.right, r.bottom])
    }

    fn compact(&mut self) {
        let mid = self.regions.len() / 2;
        if mid == 0 {
            return;
        }

        let merged = self.regions[..mid]
            .iter()
            .cloned()
            .reduce(|a, b| a.union(&b))
            .unwrap();

        self.regions.splice(..mid, std::iter::once(merged));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn rect(left: u16, top: u16, right: u16, bottom: u16) -> InclusiveRectangle {
        InclusiveRectangle {
            left,
            top,
            right,
            bottom,
        }
    }

    fn bbox(r: &UpdatedRegions) -> Option<[u16; 4]> {
        r.iter().reduce(|a, b| {
            [
                a[0].min(b[0]),
                a[1].min(b[1]),
                a[2].max(b[2]),
                a[3].max(b[3]),
            ]
        })
    }

    #[test]
    fn push_stores_coordinates_in_order() {
        let mut r = UpdatedRegions::default();
        r.push(rect(1, 2, 3, 4));
        r.push(rect(5, 6, 7, 8));

        let collected: Vec<[u16; 4]> = r.iter().collect();

        assert_eq!(collected, vec![[1, 2, 3, 4], [5, 6, 7, 8]]);
        assert_eq!(r.len(), 2);
    }

    #[test]
    fn reset_clears_all_regions() {
        let mut r = UpdatedRegions::default();
        r.push(rect(1, 2, 3, 4));
        r.push(rect(5, 6, 7, 8));
        r.reset();

        assert_eq!(r.len(), 0);
        assert_eq!(r.iter().count(), 0);
    }

    #[test]
    fn compact_triggers_at_max_and_reduces_len() {
        let mut r = UpdatedRegions::default();

        for i in 0..MAX_STORED_REGIONS {
            let v = i as u16;
            r.push(rect(v, v, v + 1, v + 1));
        }
        assert_eq!(r.len(), MAX_STORED_REGIONS);

        // The next push triggers compact() before inserting.
        r.push(rect(999, 999, 1000, 1000));
        assert!(
            r.len() < MAX_STORED_REGIONS,
            "expected compaction to reduce len below MAX, got {}",
            r.len()
        );
    }

    #[test]
    fn compact_preserves_overall_extremes() {
        let mut r = UpdatedRegions::default();

        // Known extremes so we can verify the union survives compaction.
        r.push(rect(0, 0, 1, 1));
        for i in 1..MAX_STORED_REGIONS - 1 {
            let v = i as u16;
            r.push(rect(v, v, v + 1, v + 1));
        }
        r.push(rect(5000, 6000, 7000, 8000));

        let before = bbox(&r).unwrap();
        r.push(rect(10, 10, 11, 11)); // triggers compact
        let after = bbox(&r).unwrap();

        assert_eq!(before, [0, 0, 7000, 8000]);

        // Compaction only merges existing regions into a bounding box,
        // so the overall extremes must still cover the original ones.
        assert!(after[0] <= before[0]);
        assert!(after[1] <= before[1]);
        assert!(after[2] >= before[2]);
        assert!(after[3] >= before[3]);
    }

    #[test]
    fn compact_on_empty_is_noop() {
        let mut r = UpdatedRegions::default();

        r.compact();
        assert_eq!(r.len(), 0);
    }

    #[test]
    fn compact_on_single_region_is_noop() {
        let mut r = UpdatedRegions::default();

        r.push(rect(1, 2, 3, 4));
        r.compact();

        assert_eq!(r.len(), 1);
        assert_eq!(r.iter().next(), Some([1, 2, 3, 4]));
    }
}
