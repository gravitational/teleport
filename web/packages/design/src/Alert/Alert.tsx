/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useState } from 'react';
import styled from 'styled-components';
import { style } from 'styled-system';

import { space, SpaceProps, width, WidthProps } from 'design/system';
import { Theme } from 'design/theme/themes/types';
import * as Icon from 'design/Icon';
import { Text, Button, Box, Flex, ButtonText, ButtonIcon } from 'design';
import { IconProps } from 'design/Icon/Icon';
import { ButtonFill, ButtonIntent } from 'design/Button';

const linkColor = style({
  prop: 'linkColor',
  cssProperty: 'color',
  key: 'colors',
});

type Kind =
  | 'neutral'
  | 'danger'
  | 'info'
  | 'warning'
  | 'success'
  | 'outline-danger'
  | 'outline-info'
  | 'outline-warn';

const kind = (props: ThemedAlertProps) => {
  const { kind, theme } = props;
  switch (kind) {
    case 'success':
      return {
        background: theme.colors.interactive.tonal.success[0].background,
        borderColor: theme.colors.interactive.solid.success.default.background,
      };
    case 'danger':
    case 'outline-danger':
      return {
        background: theme.colors.interactive.tonal.danger[0].background,
        borderColor: theme.colors.interactive.solid.danger.default.background,
      };
    case 'info':
    case 'outline-info':
      return {
        background: theme.colors.interactive.tonal.informational[0].background,
        borderColor: theme.colors.interactive.solid.accent.default.background,
      };
    case 'warning':
    case 'outline-warn':
      return {
        background: theme.colors.interactive.tonal.alert[0].background,
        borderColor: theme.colors.interactive.solid.alert.default.background,
      };
    case 'neutral':
      return {
        background: theme.colors.interactive.tonal.neutral[0].background,
        border: theme.borders[1],
        borderColor: theme.colors.text.disabled,
      };
    default:
      kind satisfies never;
  }
};

export interface AlertProps
  extends React.ComponentPropsWithoutRef<'div'>,
    SpaceProps,
    WidthProps {
  kind?: Kind;
  linkColor?: string;
  /** Additional description to be displayed below the main content. */
  details?: React.ReactNode;
  /** Overrides the icon specified by {@link AlertProps.kind}. */
  icon?: React.ComponentType<IconProps>;
  /** If specified, causes the alert to display a primary action button. */
  primaryAction?: Action;
  /** If specified, causes the alert to display a secondary action button. */
  secondaryAction?: Action;
  /** If `true`, the component displays a dismiss button that hides the alert. */
  dismissible?: boolean;
}

/** Specifies parameters of an action button. */
interface Action {
  content: React.ReactNode;
  onClick: () => void;
}

interface ThemedAlertProps extends AlertProps {
  theme: Theme;
}

/**
 * Displays an in-page alert. Component's children are displayed as the alert
 * title. Use the `details` attribute to display additional information. The
 * alert may optionally contain up to 2 action buttons and a dismiss button.
 */
export const Alert = ({
  kind = 'danger',
  children,
  details,
  icon,
  primaryAction,
  secondaryAction,
  dismissible,
  ...otherProps
}: AlertProps) => {
  const alertIconSize = kind === 'neutral' ? 'large' : 'small';
  const AlertIcon = icon || getAlertIcon(kind);
  const showActions = !!(primaryAction || secondaryAction || dismissible);

  const [dismissed, setDismissed] = useState(false);

  if (dismissed) {
    return null;
  }

  return (
    <AlertContainer kind={kind} {...otherProps}>
      <IconContainer kind={kind}>
        <AlertIcon size={alertIconSize} />
      </IconContainer>
      <Box flex="1">
        <Text typography="h3">{children}</Text>
        {details}
      </Box>
      {showActions && (
        <Flex ml={5} gap={2}>
          {primaryAction && (
            <Button
              {...primaryButtonProps(kind)}
              onClick={primaryAction.onClick}
            >
              {primaryAction.content}
            </Button>
          )}
          {secondaryAction && (
            <ButtonText onClick={secondaryAction.onClick}>
              {secondaryAction.content}
            </ButtonText>
          )}
          {dismissible && !dismissed && (
            <ButtonIcon aria-label="Dismiss" onClick={() => setDismissed(true)}>
              <Icon.Cross size="small" color="text.slightlyMuted" />
            </ButtonIcon>
          )}
        </Flex>
      )}
    </AlertContainer>
  );
};

Alert.displayName = 'Alert';

const AlertContainer = styled.div<AlertProps>`
  border-radius: ${p => p.theme.radii[3]}px;
  box-sizing: border-box;
  margin: 0 0 24px 0;
  min-height: 40px;
  padding: 12px 16px;
  overflow: auto;
  word-break: break-word;
  border: ${p => p.theme.borders[2]};
  display: flex;
  align-items: center;

  ${space}
  ${kind}
  ${width}

  a {
    color: ${({ theme }) => theme.colors.light};
    ${linkColor}
  }
`;

const getAlertIcon = (kind: Kind) => {
  switch (kind) {
    case 'success':
      return Icon.Checks;
    case 'danger':
    case 'outline-danger':
      return Icon.Warning;
    case 'info':
    case 'outline-info':
      return Icon.Info;
    case 'warning':
    case 'outline-warn':
      return Icon.Notification;
    case 'neutral':
      return Icon.Notification;
    default:
      kind satisfies never;
  }
};

const iconContainerStyles = ({ kind, theme }: { kind: Kind; theme: Theme }) => {
  switch (kind) {
    case 'success':
      return {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.interactive.solid.success.default.background,
        padding: `${theme.space[2]}px`,
      };
    case 'danger':
    case 'outline-danger':
      return {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.interactive.solid.danger.default.background,
        padding: `${theme.space[2]}px`,
      };
    case 'info':
    case 'outline-info':
      return {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.interactive.solid.accent.default.background,
        padding: `${theme.space[2]}px`,
      };
    case 'warning':
    case 'outline-warn':
      return {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.interactive.solid.alert.default.background,
        padding: `${theme.space[2]}px`,
      };
    case 'neutral':
      return {
        color: theme.colors.text.main,
        background: 'none',
      };
    default:
      kind satisfies never;
  }
};

const IconContainer = styled.div<{ kind: Kind }>`
  border-radius: 50%;
  line-height: 0;
  margin-right: ${p => p.theme.space[3]}px;

  ${iconContainerStyles}
`;

const primaryButtonProps = (
  kind: Kind
): { fill: ButtonFill; intent: ButtonIntent } => {
  switch (kind) {
    case 'neutral':
      return { fill: 'filled', intent: 'primary' };
    case 'danger':
    case 'outline-danger':
      return { fill: 'border', intent: 'neutral' };
    case 'info':
    case 'outline-info':
      return { fill: 'border', intent: 'neutral' };
    case 'warning':
    case 'outline-warn':
      return { fill: 'border', intent: 'neutral' };
    case 'success':
      return { fill: 'filled', intent: 'neutral' };
    default:
      kind satisfies never;
  }
};

export const Danger = (props: AlertProps) => <Alert kind="danger" {...props} />;
export const Info = (props: AlertProps) => <Alert kind="info" {...props} />;
export const Warning = (props: AlertProps) => (
  <Alert kind="outline-warn" {...props} />
);
export const Success = (props: AlertProps) => (
  <Alert kind="success" {...props} />
);

/** @deprecated Use {@link Danger} */
export const OutlineDanger = Danger;
/** @deprecated Use {@link Info} */
export const OutlineInfo = Info;
/** @deprecated Use {@link Warning} */
export const OutlineWarn = Warning;
