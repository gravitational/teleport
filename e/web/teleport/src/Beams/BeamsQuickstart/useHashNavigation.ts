import { useCallback, useEffect, useRef } from 'react';

function getHashId(): string {
  return window.location.hash.slice(1);
}

function hashMatches(id: string): boolean {
  return window.location.hash === `#${id}`;
}

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
    if (!hashMatches(id)) {
      history.pushState(null, '', `#${id}`);
    }
  }, []);

  const hasScrolledToInitialHash = useRef(false);

  // Used for reloads or visits from external links.
  useEffect(() => {
    const initialHash = getHashId();
    if (initialHash) {
      requestAnimationFrame(() => {
        scrollToId(initialHash, 'auto');
        hasScrolledToInitialHash.current = true;
      });
    } else {
      // Nothing to preserve, so scroll syncing can start right away.
      hasScrolledToInitialHash.current = true;
    }

    // Used for back/forward navigation as the url is updated without page reload.
    const onPopState = () => scrollToId(getHashId() || sectionIds[0], 'auto');
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, [sectionIds]);

  // Used for url syncing when the active id changes as we scroll to the new section.
  useEffect(() => {
    if (!activeId || !hasScrolledToInitialHash.current) return;
    if (!hashMatches(activeId)) {
      history.replaceState(null, '', `#${activeId}`);
    }
  }, [activeId]);

  return onSelect;
}
