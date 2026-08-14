import { useCallback, useMemo, useState } from 'react';

import { Beam } from 'e-teleport/services/beams/types';

export type BeamSelection = ReturnType<typeof useBeamSelection>;

export function useBeamSelection() {
  const [selected, setSelected] = useState<Beam[]>([]);

  const toggleOne = useCallback((beam: Beam, checked: boolean) => {
    setSelected(prev => {
      const without = prev.filter(b => b.name !== beam.name);
      if (checked) {
        return [...without, beam];
      }
      return without;
    });
  }, []);

  const toggleAll = useCallback((beams: Beam[]) => {
    setSelected(prev => {
      const beamNames = new Set(beams.map(b => b.name));
      const remaining = prev.filter(b => !beamNames.has(b.name));
      const allSelected = prev.length - remaining.length === beams.length;
      return allSelected ? remaining : [...remaining, ...beams];
    });
  }, []);

  const removeMany = useCallback((beams: Beam[]) => {
    const names = new Set(beams.map(b => b.name));
    setSelected(prev => prev.filter(b => !names.has(b.name)));
  }, []);

  const clear = useCallback(() => setSelected([]), []);

  const has = useCallback(
    (name: string) => selected.some(b => b.name === name),
    [selected]
  );

  return useMemo(
    () => ({
      selected,
      size: selected.length,
      has,
      toggleOne,
      toggleAll,
      removeMany,
      clear,
    }),
    [selected, has, toggleOne, toggleAll, removeMany, clear]
  );
}
