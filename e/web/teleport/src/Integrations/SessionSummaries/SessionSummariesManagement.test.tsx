import { act, renderHook } from '@testing-library/react';
import type { PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';

import {
  OverlayEntity,
  OverlayType,
  SessionSummariesManagementProvider,
  useSessionSummariesManagement,
} from './SessionSummariesManagement';

test('parses empty hash as no overlays', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper(),
  });

  expect(result.current.overlays).toEqual([]);
});

test('parses edit-model overlay', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper({ initialEntries: ['/#edit-model:my-model'] }),
  });

  expect(result.current.overlays).toEqual([
    { entity: OverlayEntity.Model, name: 'my-model', type: OverlayType.Edit },
  ]);
});

test('parses edit-policy overlay', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper({ initialEntries: ['/#edit-policy:my-policy'] }),
  });

  expect(result.current.overlays).toEqual([
    {
      entity: OverlayEntity.Policy,
      name: 'my-policy',
      type: OverlayType.Edit,
    },
  ]);
});

test('parses new-model overlay', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper({ initialEntries: ['/#new-model'] }),
  });

  expect(result.current.overlays).toEqual([
    { entity: OverlayEntity.Model, type: OverlayType.New },
  ]);
});

test('parses new-policy overlay', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper({ initialEntries: ['/#new-policy'] }),
  });

  expect(result.current.overlays).toEqual([
    { entity: OverlayEntity.Policy, type: OverlayType.New },
  ]);
});

test('parses multiple overlays', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper({
      initialEntries: ['/#new-model;edit-secret:api-key'],
    }),
  });

  expect(result.current.overlays).toEqual([
    { entity: OverlayEntity.Model, type: OverlayType.New },
    { entity: OverlayEntity.Secret, name: 'api-key', type: OverlayType.Edit },
  ]);
});

test('ignores invalid hash entries', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper({
      initialEntries: ['/#invalid;new-model;also-invalid'],
    }),
  });

  expect(result.current.overlays).toEqual([
    { entity: OverlayEntity.Model, type: OverlayType.New },
  ]);
});

test('createNewOverlayLink creates correct link for model', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper(),
  });

  const link = result.current.createNewOverlayLink(OverlayEntity.Model);

  expect(link).toBe('/#new-model');
});

test('createNewOverlayLink creates correct link for policy', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper(),
  });

  const link = result.current.createNewOverlayLink(OverlayEntity.Policy);

  expect(link).toBe('/#new-policy');
});

test('createEditOverlayLink creates correct link for model', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper(),
  });

  const link = result.current.createEditOverlayLink(
    OverlayEntity.Model,
    'gpt-4'
  );

  expect(link).toBe('/#edit-model:gpt-4');
});

test('createEditOverlayLink creates correct link for policy', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper(),
  });

  const link = result.current.createEditOverlayLink(
    OverlayEntity.Policy,
    'default-policy'
  );

  expect(link).toBe('/#edit-policy:default-policy');
});

test('link creation appends to existing overlays', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper({ initialEntries: ['/#new-model'] }),
  });

  const link = result.current.createEditOverlayLink(
    OverlayEntity.Secret,
    'api-key'
  );

  expect(link).toBe('/#new-model;edit-secret:api-key');
});

test('starts with null pending selection', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper(),
  });

  expect(result.current.pendingSelection).toBeNull();
});

test('sets and clears pending selection', () => {
  const { result } = renderHook(() => useSessionSummariesManagement(), {
    wrapper: wrapper(),
  });

  act(() => {
    result.current.setPendingSelection({
      entity: OverlayEntity.Secret,
      name: 'my-secret',
    });
  });

  expect(result.current.pendingSelection).toEqual({
    entity: OverlayEntity.Secret,
    name: 'my-secret',
  });

  act(() => {
    result.current.clearPendingSelection();
  });

  expect(result.current.pendingSelection).toBeNull();
});

function wrapper({
  initialEntries = ['/'],
}: { initialEntries?: string[] } = {}) {
  return function Wrapper({ children }: PropsWithChildren) {
    return (
      <MemoryRouter initialEntries={initialEntries}>
        <SessionSummariesManagementProvider>
          {children}
        </SessionSummariesManagementProvider>
      </MemoryRouter>
    );
  };
}
