/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import Dialog from 'design/Dialog';
import { forwardRef, useRef } from 'react';
import { Transition, TransitionStatus } from 'react-transition-group';
import { css } from 'styled-components';

import { State as ResourcesState } from 'teleport/components/useResources';
import { RoleWithYaml } from 'teleport/services/resources';

import { RoleEditorAdapter } from './RoleEditorAdapter';

/**
 * Renders a full-screen dialog with a slide-in effect.
 *
 * TODO(bl-nero): This component has been copied from `ReviewRequests` and
 * `NotificationRoutingRulesDialog`. It probably should become reusable.
 */
export function RoleEditorDialog({
  open,
  onClose,
  resources,
  onSave,
}: {
  open: boolean;
  onClose(): void;
  resources: ResourcesState;
  onSave(role: Partial<RoleWithYaml>): Promise<void>;
}) {
  const transitionRef = useRef<HTMLDivElement>();
  return (
    <Transition
      in={open}
      nodeRef={transitionRef}
      timeout={300}
      mountOnEnter
      unmountOnExit
    >
      {transitionState => (
        <DialogInternal
          ref={transitionRef}
          onClose={onClose}
          transitionState={transitionState}
          resources={resources}
          onSave={onSave}
        />
      )}
    </Transition>
  );
}

const DialogInternal = forwardRef<
  HTMLDivElement,
  {
    onClose(): void;
    transitionState: TransitionStatus;
    resources: ResourcesState;
    onSave(role: Partial<RoleWithYaml>): Promise<void>;
  }
>(({ onClose, transitionState, resources, onSave }, ref) => {
  return (
    <Dialog
      dialogCss={() => fullScreenDialogCss()}
      disableEscapeKeyDown={false}
      open={true}
      ref={ref}
      className={transitionState}
    >
      <RoleEditorAdapter
        resources={resources}
        onSave={onSave}
        onCancel={onClose}
      />
    </Dialog>
  );
});

const fullScreenDialogCss = () => css`
  padding: 0;
  width: 100%;
  height: 100%;
  max-height: 100%;
  right: 0;
  border-radius: 0;
  overflow-y: hidden;
  flex-direction: row;
  background: ${props => props.theme.colors.levels.sunken};
  transition: width 300ms ease-out;

  &.entering {
    right: -100%;
  }

  &.entered {
    right: 0px;
    transition: right 300ms ease-out;
  }

  &.exiting {
    right: -100%;
    transition: right 300ms ease-out;
  }

  &.exited {
    right: -100%;
  }
`;
