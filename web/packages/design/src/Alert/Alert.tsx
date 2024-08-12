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
import styled, { useTheme } from 'styled-components';
import { style, color, ColorProps } from 'styled-system';

import { space, SpaceProps, width, WidthProps } from 'design/system';
import { Theme } from 'design/theme/themes/types';
import * as Icon from 'design/Icon';
import Flex from 'design/Flex';
import { Text, Button, Box, ButtonText, ButtonIcon } from 'design';
import { IconProps } from 'design/Icon/Icon';
import { ButtonFill, ButtonIntent } from 'design/Button';

const linkColor = style({
  prop: 'linkColor',
  cssProperty: 'color',
  key: 'colors',
});

type AlertKind =
  | 'neutral'
  | 'danger'
  | 'info'
  | 'warning'
  | 'success'
  | 'outline-danger'
  | 'outline-info'
  | 'outline-warn';

const alertBorder = (props: ThemedAlertProps) => {
  const { kind, theme } = props;
  switch (kind) {
    case 'success':
      return {
        borderColor: theme.colors.interactive.solid.success.default.background,
      };
    case 'danger':
    case 'outline-danger':
      return {
        borderColor: theme.colors.interactive.solid.danger.default.background,
      };
    case 'info':
    case 'outline-info':
      return {
        borderColor: theme.colors.interactive.solid.accent.default.background,
      };
    case 'warning':
    case 'outline-warn':
      return {
        borderColor: theme.colors.interactive.solid.alert.default.background,
      };
    case 'neutral':
      return {
        border: theme.borders[1],
        borderColor: theme.colors.text.disabled,
      };
    default:
      kind satisfies never;
  }
};

const backgroundColor = (props: ThemedAlertProps) => {
  const { kind, theme } = props;
  switch (kind) {
    case 'success':
      return {
        background: theme.colors.interactive.tonal.success[0].background,
      };
    case 'danger':
    case 'outline-danger':
      return {
        background: theme.colors.interactive.tonal.danger[0].background,
      };
    case 'info':
    case 'outline-info':
      return {
        background: theme.colors.interactive.tonal.informational[0].background,
      };
    case 'warning':
    case 'outline-warn':
      return {
        background: theme.colors.interactive.tonal.alert[0].background,
      };
    case 'neutral':
      return {
        background: theme.colors.interactive.tonal.neutral[0].background,
      };
    default:
      kind satisfies never;
  }
};

interface Props<K> {
  kind?: K;
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
  children?: React.ReactNode;
  style?: React.CSSProperties;
  onDismiss?: () => void;
}

/** Specifies parameters of an action button. */
interface Action {
  content: React.ReactNode;
  onClick: () => void;
}

export interface AlertProps
  extends Props<AlertKind>,
    SpaceProps,
    WidthProps,
    ColorProps {
  linkColor?: string;
}

interface ThemedAlertProps extends AlertProps {
  theme: Theme;
}

/**
 * Displays an in-page alert. Component's children are displayed as the alert
 * title. Use the `details` attribute to display additional information. The
 * alert may optionally contain up to 2 action buttons and a dismiss button.
 *
 * The in-page alert, by default, is semi-transparent. To display it as a
 * floating element (i.e. as an error indicator above the infinite scroll), you
 * need to set the `bg` attribute to a solid color; the element's
 * semi-transparent background will then be overlaid on top of it. Note that
 * it's not enough to just display it on a background because of the round
 * corers.
 */
export const Alert = ({
  kind = 'danger',
  children,
  details,
  icon,
  primaryAction,
  secondaryAction,
  dismissible,
  bg,
  onDismiss,
  ...otherProps
}: AlertProps) => {
  const alertIconSize = kind === 'neutral' ? 'large' : 'small';
  const [dismissed, setDismissed] = useState(false);

  const onDismissClick = () => {
    setDismissed(true);
    onDismiss?.();
  };

  if (dismissed) {
    return null;
  }

  return (
    <OuterContainer bg={bg} kind={kind} {...otherProps}>
      <InnerContainer kind={kind}>
        <IconContainer kind={kind}>
          <AlertIcon kind={kind} customIcon={icon} size={alertIconSize} />
        </IconContainer>
        <Box flex="1">
          <Text typography="h3">{children}</Text>
          {details}
        </Box>
        <ActionButtons
          kind={kind}
          primaryAction={primaryAction}
          secondaryAction={secondaryAction}
          dismissible={dismissible}
          dismissed={dismissed}
          onDismiss={onDismissClick}
        />
      </InnerContainer>
    </OuterContainer>
  );
};

/** Renders a round border and allows background color customization. */
const OuterContainer = styled.div<AlertProps>`
  box-sizing: border-box;
  margin: 0 0 24px 0;
  min-height: 40px;

  border: ${p => p.theme.borders[2]};
  border-radius: ${p => p.theme.radii[3]}px;

  ${space}
  ${width}
  ${alertBorder}
  ${color}

  a {
    color: ${({ theme }) => theme.colors.light};
    ${linkColor}
  }
`;

/** Renders a transparent color overlay. */
const InnerContainer = styled.div<AlertProps>`
  padding: 12px 16px;
  overflow: auto;
  word-break: break-word;
  display: flex;
  align-items: center;

  ${backgroundColor}
`;

const AlertIcon = ({
  kind,
  customIcon: CustomIcon,
  ...otherProps
}: {
  kind: AlertKind | BannerKind;
  customIcon: React.ComponentType<IconProps>;
} & IconProps) => {
  const commonProps = { role: 'graphics-symbol', ...otherProps };
  if (CustomIcon) {
    return <CustomIcon {...commonProps} />;
  }
  switch (kind) {
    case 'success':
      return <Icon.Checks aria-label="Success" {...commonProps} />;
    case 'danger':
    case 'outline-danger':
      return <Icon.WarningCircle aria-label="Danger" {...commonProps} />;
    case 'info':
    case 'outline-info':
      return <Icon.Info aria-label="Info" {...commonProps} />;
    case 'warning':
    case 'outline-warn':
      return <Icon.Warning aria-label="Warning" {...commonProps} />;
    case 'neutral':
    case 'primary':
      return <Icon.Notification aria-label="Note" {...commonProps} />;
    default:
      kind satisfies never;
  }
};

const iconContainerStyles = ({
  kind,
  theme,
}: {
  kind: AlertKind;
  theme: Theme;
}) => {
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

const IconContainer = styled.div<{ kind: AlertKind }>`
  border-radius: 50%;
  line-height: 0;
  margin-right: ${p => p.theme.space[3]}px;

  ${iconContainerStyles}
`;

const primaryButtonProps = (
  kind: AlertKind | BannerKind
): { fill: ButtonFill; intent: ButtonIntent } => {
  return kind === 'neutral'
    ? { fill: 'filled', intent: 'primary' }
    : { fill: 'border', intent: 'neutral' };
};

const ActionButtons = ({
  kind,
  primaryAction,
  secondaryAction,
  dismissible,
  dismissed,
  onDismiss,
}: {
  kind: AlertKind | BannerKind;
  primaryAction?: Action;
  secondaryAction?: Action;
  dismissible?: boolean;
  dismissed: boolean;
  onDismiss: () => void;
}) => {
  if (!(primaryAction || secondaryAction || dismissible)) return;

  return (
    <Flex ml={5} gap={2}>
      {primaryAction && (
        <Button {...primaryButtonProps(kind)} onClick={primaryAction.onClick}>
          {primaryAction.content}
        </Button>
      )}
      {secondaryAction && (
        <ButtonText onClick={secondaryAction.onClick}>
          {secondaryAction.content}
        </ButtonText>
      )}
      {dismissible && !dismissed && (
        <ButtonIcon aria-label="Dismiss" onClick={onDismiss}>
          <Icon.Cross size="small" color="text.slightlyMuted" />
        </ButtonIcon>
      )}
    </Flex>
  );
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

type BannerKind =
  | 'neutral'
  | 'primary'
  | 'danger'
  | 'info'
  | 'warning'
  | 'success';
type BannerProps = Props<BannerKind>;

/**
 * Renders a page-level banner alert. Use only for product-, cluster-, or
 * account-level events or states.
 */
export const Banner = ({
  kind = 'danger',
  children,
  details,
  icon,
  primaryAction,
  secondaryAction,
  dismissible,
  onDismiss,
}: BannerProps) => {
  const theme = useTheme();
  const { backgroundColor, foregroundColor, iconColor } = bannerColors(
    theme,
    kind
  );
  const [dismissed, setDismissed] = useState(false);

  const onDismissClick = () => {
    setDismissed(true);
    onDismiss?.();
  };

  if (dismissed) {
    return null;
  }

  return (
    <Flex
      bg={backgroundColor}
      borderBottom="3px solid"
      borderColor={foregroundColor}
      px={4}
      py={2}
      gap={3}
      alignItems="center"
    >
      <AlertIcon kind={kind} customIcon={icon} size="large" color={iconColor} />
      <Box flex="1">
        <Text typography="h3">{children}</Text>
        {details}
      </Box>
      <ActionButtons
        kind={kind}
        primaryAction={primaryAction}
        secondaryAction={secondaryAction}
        dismissible={dismissible}
        dismissed={dismissed}
        onDismiss={onDismissClick}
      />
    </Flex>
  );
};

const bannerColors = (theme: Theme, kind: BannerKind) => {
  switch (kind) {
    case 'primary':
      return {
        backgroundColor: theme.colors.interactive.tonal.primary[2].background,
        foregroundColor:
          theme.colors.interactive.solid.primary.default.background,
        iconColor: theme.colors.text.main,
      };
    case 'neutral':
      return {
        backgroundColor: theme.colors.levels.elevated,
        foregroundColor: theme.colors.text.main,
        iconColor: theme.colors.text.main,
      };
    case 'danger':
      return {
        backgroundColor: theme.colors.interactive.tonal.danger[2].background,
        foregroundColor:
          theme.colors.interactive.solid.danger.default.background,
        iconColor: theme.colors.interactive.solid.danger.default.background,
      };
    case 'warning':
      return {
        backgroundColor: theme.colors.interactive.tonal.alert[2].background,
        foregroundColor:
          theme.colors.interactive.solid.alert.default.background,
        iconColor: theme.colors.interactive.solid.alert.default.background,
      };
    case 'info':
      return {
        backgroundColor:
          theme.colors.interactive.tonal.informational[2].background,
        foregroundColor:
          theme.colors.interactive.solid.accent.default.background,
        iconColor: theme.colors.interactive.solid.accent.default.background,
      };
    case 'success':
      return {
        backgroundColor: theme.colors.interactive.tonal.success[2].background,
        foregroundColor:
          theme.colors.interactive.solid.success.default.background,
        iconColor: theme.colors.interactive.solid.success.default.background,
      };
    default:
      kind satisfies never;
  }
};
