import { act, renderHook } from '@testing-library/react';

import { useHashNavigation } from './useHashNavigation';

const sectionIds = ['intro', 'faq', 'pricing'];

describe('useHashNavigation', () => {
  let rafCallbacks: FrameRequestCallback[];

  beforeEach(() => {
    rafCallbacks = [];
    jest.spyOn(window, 'requestAnimationFrame').mockImplementation(cb => {
      rafCallbacks.push(cb);
      return 0;
    });
    history.replaceState(null, '', window.location.pathname);
  });

  afterEach(() => {
    jest.restoreAllMocks();
    history.replaceState(null, '', window.location.pathname);
  });

  const flushRaf = () =>
    act(() => {
      const cbs = rafCallbacks;
      rafCallbacks = [];
      cbs.forEach(cb => cb(0));
    });

  const render = (activeId: string | undefined) =>
    renderHook(({ id }) => useHashNavigation(sectionIds, id), {
      initialProps: { id: activeId },
    });

  it('does not overwrite an incoming hash before the target section is reached', () => {
    history.replaceState(null, '', '#faq');

    render('intro');

    expect(window.location.hash).toBe('#faq');
  });

  it('resumes syncing the hash once the initial scroll has settled', () => {
    history.replaceState(null, '', '#faq');

    const { rerender } = render('intro');
    expect(window.location.hash).toBe('#faq');

    flushRaf();

    rerender({ id: 'pricing' });
    expect(window.location.hash).toBe('#pricing');
  });

  it('syncs the hash from the active section when loaded without a hash', () => {
    const { rerender } = render('intro');

    rerender({ id: 'faq' });
    expect(window.location.hash).toBe('#faq');
  });

  it('pushes the hash when a section is explicitly selected', () => {
    const { result } = render('intro');

    act(() => result.current('pricing'));
    expect(window.location.hash).toBe('#pricing');
  });
});
