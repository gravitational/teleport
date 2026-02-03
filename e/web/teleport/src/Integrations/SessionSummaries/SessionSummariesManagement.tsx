import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type PropsWithChildren,
} from 'react';
import { useHistory, useLocation } from 'react-router';

import { SessionSummariesOverlays } from 'e-teleport/Integrations/SessionSummaries/Overlays';

/**
 * In the session summaries management UI, multiple modals can be open (you can add or
 * edit a secret from a model definition, for example).
 *
 * This context helps manage the state of these modals (called "overlays" here) via
 * the URL hash.
 */

interface SessionSummariesContextValue {
  clearPendingSelection(): void;
  closeCurrentOverlay(): void;
  createEditOverlayLink(entity: OverlayEntity, name: string): string;
  createNewOverlayLink(entity: OverlayEntity): string;
  openEditOverlay(entity: OverlayEntity, name: string): void;
  openNewOverlay(entity: OverlayEntity): void;
  overlays: Overlay[];
  pendingSelection: null | PendingSelection; // when a modal is opened on top of another, it may need to pass some info back to the underlying one
  setPendingSelection(selection: PendingSelection): void;
}

const SessionSummariesContext =
  createContext<SessionSummariesContextValue>(null);

export enum OverlayEntity {
  Policy,
  Model,
  Secret,
}

export enum OverlayType {
  Edit,
  New,
}

interface EditOverlay {
  entity: OverlayEntity;
  name: string;
  type: OverlayType.Edit;
}

interface NewOverlay {
  entity: OverlayEntity;
  type: OverlayType.New;
}

type Overlay = EditOverlay | NewOverlay;

interface PendingSelection {
  entity: OverlayEntity;
  name: string;
}

export function SessionSummariesManagementProvider({
  children,
}: PropsWithChildren) {
  const history = useHistory();
  const location = useLocation();

  const [pendingSelection, setPendingSelection] =
    useState<null | PendingSelection>(null);

  const overlays = useMemo(
    () => createOverlaysFromHash(location.hash),
    [location.hash]
  );

  const closeCurrentOverlay = useCallback(() => {
    const newOverlays = overlays.slice(0, -1);

    if (newOverlays.length === 0) {
      history.push(location.pathname + location.search);

      return;
    }

    const serialized = newOverlays.map(serializeOverlay).join(SEPARATOR);

    // if there are still overlays open, replace the current history entry instead of pushing a new one
    history.replace(location.pathname + location.search + `#${serialized}`);
  }, [history, location, overlays]);

  const createEditOverlayLink = useCallback(
    (entity: OverlayEntity, name: string) => {
      const overlay: EditOverlay = {
        entity,
        name,
        type: OverlayType.Edit,
      };

      const newOverlays = [...overlays, overlay];
      const serialized = newOverlays.map(serializeOverlay).join(SEPARATOR);

      return location.pathname + location.search + `#${serialized}`;
    },
    [location, overlays]
  );

  const createNewOverlayLink = useCallback(
    (entity: OverlayEntity) => {
      const overlay: NewOverlay = {
        entity,
        type: OverlayType.New,
      };

      const newOverlays = [...overlays, overlay];
      const serialized = newOverlays.map(serializeOverlay).join(SEPARATOR);

      return location.pathname + location.search + `#${serialized}`;
    },
    [location, overlays]
  );

  const openEditOverlay = useCallback(
    (entity: OverlayEntity, name: string) => {
      history.push(createEditOverlayLink(entity, name));
    },
    [history, createEditOverlayLink]
  );

  const openNewOverlay = useCallback(
    (entity: OverlayEntity) => {
      history.push(createNewOverlayLink(entity));
    },
    [history, createNewOverlayLink]
  );

  const clearPendingSelection = useCallback(() => {
    setPendingSelection(null);
  }, []);

  const value = useMemo(
    () => ({
      clearPendingSelection,
      closeCurrentOverlay,
      createEditOverlayLink,
      createNewOverlayLink,
      openEditOverlay,
      openNewOverlay,
      overlays,
      pendingSelection,
      setPendingSelection,
    }),
    [
      clearPendingSelection,
      closeCurrentOverlay,
      createEditOverlayLink,
      createNewOverlayLink,
      openEditOverlay,
      openNewOverlay,
      overlays,
      pendingSelection,
    ]
  );

  return (
    <SessionSummariesContext.Provider value={value}>
      {children}

      <SessionSummariesOverlays />
    </SessionSummariesContext.Provider>
  );
}

export function useSessionSummariesManagement() {
  const context = useContext(SessionSummariesContext);

  if (!context) {
    throw new Error(
      'useSessionSummariesManagement must be used within a SessionSummariesManagementProvider'
    );
  }

  return context;
}

const SEPARATOR = ';';

const EDIT_MODEL_PREFIX = 'edit-model:';
const EDIT_POLICY_PREFIX = 'edit-policy:';
const EDIT_SECRET_PREFIX = 'edit-secret:';
const NEW_MODEL_ENTRY = 'new-model';
const NEW_POLICY_ENTRY = 'new-policy';
const NEW_SECRET_ENTRY = 'new-secret';

function deserializeOverlay(entry: string): null | Overlay {
  if (entry.startsWith(EDIT_MODEL_PREFIX)) {
    return {
      entity: OverlayEntity.Model,
      name: entry.slice(EDIT_MODEL_PREFIX.length),
      type: OverlayType.Edit,
    };
  }

  if (entry.startsWith(EDIT_POLICY_PREFIX)) {
    return {
      entity: OverlayEntity.Policy,
      name: entry.slice(EDIT_POLICY_PREFIX.length),
      type: OverlayType.Edit,
    };
  }

  if (entry.startsWith(EDIT_SECRET_PREFIX)) {
    return {
      entity: OverlayEntity.Secret,
      name: entry.slice(EDIT_SECRET_PREFIX.length),
      type: OverlayType.Edit,
    };
  }

  if (entry === NEW_MODEL_ENTRY) {
    return {
      entity: OverlayEntity.Model,
      type: OverlayType.New,
    };
  }

  if (entry === NEW_POLICY_ENTRY) {
    return {
      entity: OverlayEntity.Policy,
      type: OverlayType.New,
    };
  }

  if (entry === NEW_SECRET_ENTRY) {
    return {
      entity: OverlayEntity.Secret,
      type: OverlayType.New,
    };
  }

  return null;
}

function serializeOverlay(overlay: Overlay) {
  if (overlay.type === OverlayType.Edit) {
    switch (overlay.entity) {
      case OverlayEntity.Model:
        return `${EDIT_MODEL_PREFIX}${overlay.name}`;
      case OverlayEntity.Policy:
        return `${EDIT_POLICY_PREFIX}${overlay.name}`;
      case OverlayEntity.Secret:
        return `${EDIT_SECRET_PREFIX}${overlay.name}`;
    }
  }

  switch (overlay.entity) {
    case OverlayEntity.Model:
      return NEW_MODEL_ENTRY;
    case OverlayEntity.Policy:
      return NEW_POLICY_ENTRY;
    case OverlayEntity.Secret:
      return NEW_SECRET_ENTRY;
  }
}

function createOverlaysFromHash(hash: string): Overlay[] {
  if (!hash.startsWith('#')) {
    return [];
  }

  return hash
    .slice(1)
    .split(SEPARATOR)
    .map(deserializeOverlay)
    .filter((overlay): overlay is Overlay => overlay !== null);
}
