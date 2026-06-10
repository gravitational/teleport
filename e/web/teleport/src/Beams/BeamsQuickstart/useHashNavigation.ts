import { useCallback, useEffect } from 'react';

function scrollToId(id: string, behavior: ScrollBehavior) {
  document.getElementById(id)?.scrollIntoView({ behavior, block: 'start' });
}

/**
 * Keeps the page in sync with the URL hash so each section is
 * independently linkable.
 */
export function useHashNavigation<T extends string>(
  sectionIds: T[],
  activeId: T | undefined
) {
  const onSelect = useCallback((id: T) => {
    scrollToId(id, 'smooth');
    // Skip when the hash already matches so repeated clicks on the same
    // item don't pile up history entries.
    if (window.location.hash !== `#${id}`) {
      history.pushState(null, '', `#${id}`);
    }
  }, []);

  useEffect(() => {
    const goToHash = () =>
      scrollToId(window.location.hash.slice(1) || sectionIds[0], 'auto');

    if (window.location.hash) requestAnimationFrame(goToHash);
    window.addEventListener('popstate', goToHash);

    return () => window.removeEventListener('popstate', goToHash);
  }, [sectionIds]);

  useEffect(() => {
    if (!activeId) return;
    if (window.location.hash !== `#${activeId}`) {
      history.replaceState(null, '', `#${activeId}`);
    }
  }, [activeId]);

  return onSelect;
}
