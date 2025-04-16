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
import { color, ColorProps, style } from 'styled-system';

import { IconProps } from 'design/Icon/Icon';
import { StatusIcon, StatusKind } from 'design/StatusIcon';

import Box from '../Box';
import { Button, ButtonFill, ButtonIntent } from '../Button';
import ButtonIcon from '../ButtonIcon';
import Flex from '../Flex';
import * as Icon from '../Icon';
import { space, SpaceProps, width, WidthProps } from '../system';
import Text from '../Text';
import { Theme } from '../theme';

const linkColor = style({
  prop: 'linkColor',
  cssProperty: 'color',
  key: 'colors',
});

export type AlertKind =
  | 'neutral'
  | 'danger'
  | 'info'
  | 'warning'
  | 'success'
  | 'outline-danger'
  | 'outline-info'
  | 'outline-warn';

const alertBorder = (
  props: ThemedAlertProps
): { borderColor: string; border?: string | number } => {
  const { kind, theme } = props;
  switch (kind) {
    case 'success':
      return {
        borderColor: theme.colors.interactive.solid.success.default,
      };
    case 'danger':
    case 'outline-danger':
      return {
        borderColor: theme.colors.interactive.solid.danger.default,
      };
    case 'info':
    case 'outline-info':
      return {
        borderColor: theme.colors.interactive.solid.accent.default,
      };
    case 'warning':
    case 'outline-warn':
      return {
        borderColor: theme.colors.interactive.solid.alert.default,
      };
    case 'neutral':
      return {
        border: theme.borders[1],
        borderColor: theme.colors.text.disabled,
      };
  }
};

const backgroundColor = (
  props: Pick<ThemedAlertProps, 'kind' | 'theme'>
): { background: string } => {
  const { kind, theme } = props;
  switch (kind) {
    case 'success':
      return {
        background: theme.colors.interactive.tonal.success[0],
      };
    case 'danger':
    case 'outline-danger':
      return {
        background: theme.colors.interactive.tonal.danger[0],
      };
    case 'info':
    case 'outline-info':
      return {
        background: theme.colors.interactive.tonal.informational[0],
      };
    case 'warning':
    case 'outline-warn':
      return {
        background: theme.colors.interactive.tonal.alert[0],
      };
    case 'neutral':
      return {
        background: theme.colors.interactive.tonal.neutral[0],
      };
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
  alignItems?: 'center' | 'flex-start';
}

/**
 * Specifies parameters of an action button. If no `href` is specified, the
 * button is rendered as a regular button; otherwise, a link (with a button
 * appearance) is used instead, and `href` is used as a link target. A link
 * button can still have an `onClick` handler, too.
 */
export interface Action {
  content: React.ReactNode;
  href?: string;
  onClick?: (event: React.MouseEvent) => void;
}

export interface AlertProps
  extends Props<AlertKind>,
    SpaceProps,
    WidthProps,
    ColorProps {
  linkColor?: string;
}

interface ThemedAlertProps extends AlertPropsWithRequiredKind {
  theme: Theme;
}

type WithRequired<T, K extends keyof T> = T & { [P in K]-?: T[P] };

type AlertPropsWithRequiredKind = WithRequired<AlertProps, 'kind'>;

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
  alignItems = 'center',
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
      <InnerContainer kind={kind} alignItems={alignItems}>
        <IconContainer kind={kind}>
          <StatusIcon
            kind={iconKind(kind)}
            customIcon={icon}
            size={alertIconSize}
            color="inherit"
          />
        </IconContainer>
        <Box
          flex="1"
          css={`
            // This preserves white spaces from Go errors (mainly in Teleport Connect).
            // Thanks to it, each error line is nicely indented with tab,
            //  instead od being treated as a one, long line.
            white-space: pre-wrap;
          `}
        >
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
const OuterContainer = styled.div<AlertPropsWithRequiredKind>`
  box-sizing: border-box;
  margin: 0 0 24px 0;

  border: ${p => p.theme.borders[2]};
  border-radius: ${p => p.theme.radii[3]}px;

  ${space}
  ${width}
  ${alertBorder}
  ${color}
  a {
    // Using the same color as Link (theme.solid.interactive.solid.accent) looks bad in the BBLP
    // theme, so instead let's default to the color of the text and decorate links only with an
    // underline.
    color: inherit;
    ${linkColor}
  }
`;

/** Renders a transparent color overlay. */
const InnerContainer = styled.div<
  Pick<WithRequired<AlertProps, 'kind' | 'alignItems'>, 'kind' | 'alignItems'>
>`
  padding: 12px 16px;
  overflow: auto;
  word-break: break-word;
  display: flex;
  align-items: ${p => p.alignItems};

  ${backgroundColor}
`;

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
        background: theme.colors.interactive.solid.success.default,
        padding: `${theme.space[2]}px`,
      };
    case 'danger':
    case 'outline-danger':
      return {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.interactive.solid.danger.default,
        padding: `${theme.space[2]}px`,
      };
    case 'info':
    case 'outline-info':
      return {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.interactive.solid.accent.default,
        padding: `${theme.space[2]}px`,
      };
    case 'warning':
    case 'outline-warn':
      return {
        color: theme.colors.text.primaryInverse,
        background: theme.colors.interactive.solid.alert.default,
        padding: `${theme.space[2]}px`,
      };
    case 'neutral':
      return {
        color: theme.colors.text.main,
        background: 'none',
      };
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
        <ActionButton {...primaryButtonProps(kind)} action={primaryAction} />
      )}
      {secondaryAction && (
        <ActionButton
          fill="minimal"
          intent="neutral"
          action={secondaryAction}
        />
      )}
      {dismissible && !dismissed && (
        <ButtonIcon aria-label="Dismiss" onClick={onDismiss}>
          <Icon.Cross size="small" color="text.slightlyMuted" />
        </ButtonIcon>
      )}
    </Flex>
  );
};

/** Renders either a regular or a link button, depending on the action. */
export const ActionButton = ({
  action: { href, content, onClick },
  fill,
  intent,
  inputAlignment = false,
  disabled = false,
  title,
}: {
  action: Action;
  fill?: ButtonFill;
  intent?: ButtonIntent;
  inputAlignment?: boolean;
  disabled?: boolean;
  title?: string;
}) =>
  href ? (
    <Button
      as="a"
      href={href}
      target="_blank"
      fill={fill}
      intent={intent}
      onClick={onClick}
      inputAlignment={inputAlignment}
      disabled={disabled}
      title={title}
    >
      {content}
    </Button>
  ) : (
    <Button
      fill={fill}
      intent={intent}
      onClick={onClick}
      inputAlignment={inputAlignment}
      disabled={disabled}
      title={title}
    >
      {content}
    </Button>
  );

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
      <StatusIcon
        kind={iconKind(kind)}
        customIcon={icon}
        size="large"
        color={iconColor}
      />
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
        backgroundColor: theme.colors.interactive.tonal.primary[2],
        foregroundColor: theme.colors.interactive.solid.primary.default,
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
        backgroundColor: theme.colors.interactive.tonal.danger[2],
        foregroundColor: theme.colors.interactive.solid.danger.default,
        iconColor: theme.colors.interactive.solid.danger.default,
      };
    case 'warning':
      return {
        backgroundColor: theme.colors.interactive.tonal.alert[2],
        foregroundColor: theme.colors.interactive.solid.alert.default,
        iconColor: theme.colors.interactive.solid.alert.default,
      };
    case 'info':
      return {
        backgroundColor: theme.colors.interactive.tonal.informational[2],
        foregroundColor: theme.colors.interactive.solid.accent.default,
        iconColor: theme.colors.interactive.solid.accent.default,
      };
    case 'success':
      return {
        backgroundColor: theme.colors.interactive.tonal.success[2],
        foregroundColor: theme.colors.interactive.solid.success.default,
        iconColor: theme.colors.interactive.solid.success.default,
      };
  }
};

const iconKind = (kind: AlertKind | BannerKind): StatusKind => {
  switch (kind) {
    case 'outline-danger':
      return 'danger';
    case 'outline-warn':
      return 'warning';
    case 'outline-info':
      return 'info';
    case 'primary':
      return 'neutral';
    default:
      return kind;
  }
};
