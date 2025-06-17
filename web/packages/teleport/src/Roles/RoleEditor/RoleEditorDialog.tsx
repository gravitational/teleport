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

import { forwardRef, useRef } from 'react';
import { CSSTransition } from 'react-transition-group';
import { css } from 'styled-components';

import Dialog from 'design/Dialog';

import { State as ResourcesState } from 'teleport/components/useResources';
import { RoleWithYaml } from 'teleport/services/resources';

import { RolesProps } from '../Roles';
import { RoleEditorAdapter } from './RoleEditorAdapter';

const animationDuration = 300;

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
  roleDiffProps,
}: {
  open: boolean;
  onClose(): void;
  resources: ResourcesState;
  onSave(role: Partial<RoleWithYaml>): Promise<void>;
} & RolesProps) {
  const transitionRef = useRef<HTMLDivElement>(null);
  return (
    <CSSTransition
      in={open}
      nodeRef={transitionRef}
      timeout={animationDuration}
      mountOnEnter
      unmountOnExit
    >
      <DialogInternal
        ref={transitionRef}
        onClose={onClose}
        resources={resources}
        onSave={onSave}
        roleDiffProps={roleDiffProps}
      />
    </CSSTransition>
  );
}

const DialogInternal = forwardRef<
  HTMLDivElement,
  {
    onClose(): void;
    resources: ResourcesState;
    onSave(role: Partial<RoleWithYaml>): Promise<void>;
  } & RolesProps
>(({ onClose, resources, onSave, roleDiffProps }, ref) => {
  return (
    <Dialog
      modalCss={() => modalCss}
      disableEscapeKeyDown={false}
      open={true}
      modalRef={ref}
      BackdropProps={{ className: 'backdrop' }}
      className="dialog"
    >
      <RoleEditorAdapter
        resources={resources}
        onSave={onSave}
        onCancel={onClose}
        roleDiffProps={roleDiffProps}
      />
    </Dialog>
  );
});

const modalCss = css`
  & .dialog {
    padding: 0;
    width: 100%;
    height: 100%;
    max-height: 100%;
    right: 0;
    border-radius: 0;
    overflow-y: hidden;
    flex-direction: row;
    background: ${props => props.theme.colors.levels.sunken};
  }

  &.enter {
    .backdrop {
      opacity: 0;
    }

    .dialog {
      transform: scale(0.8);
      opacity: 0;
    }
  }

  &.enter-active {
    .backdrop {
      opacity: 100%;
      // Chakra likes to globally disable transitions when mounting the access
      // graph component, and it does so with an !important rule matching all
      // elements. We override it with our own !important rule.
      transition: opacity ${animationDuration}ms ease-out !important;
    }

    .dialog {
      transform: scale(1);
      opacity: 100%;
      // See the comment above about the !important hack.
      transition:
        transform ${animationDuration}ms ease-out,
        opacity ${animationDuration}ms ease-out !important;
    }
  }

  &.exit {
    .backdrop {
      opacity: 100%;
    }

    .dialog {
      transform: scale(1);
      opacity: 100%;
    }
  }

  &.exit-active {
    .backdrop {
      opacity: 0;
      transition: opacity ${animationDuration}ms ease-in;
    }

    .dialog {
      transform: scale(0.8);
      opacity: 0;
      transition:
        transform ${animationDuration}ms ease-in,
        opacity ${animationDuration}ms ease-in;
    }
  }
`;
