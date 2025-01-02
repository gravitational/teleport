/**
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

import React, { useEffect, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Box, ButtonIcon, Flex, Text } from 'design';
import { ActionButton } from 'design/Alert';
import { BoxProps } from 'design/Box';
import { Cross } from 'design/Icon';
import * as Icon from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { borderColor } from 'design/system';
import { Theme } from 'design/theme/themes/types';

import type {
  NotificationItem,
  NotificationItemContent,
  NotificationItemObjectContent,
  NotificationSeverity,
} from './types';

interface NotificationProps extends BoxProps {
  item: NotificationItem;

  onRemove(): void;

  /**
   * If defined, determines whether the notification is auto-dismissed after 5
   * seconds. If undefined, the decision is based on the notification severity:
   * only 'success', 'info', and 'neutral' notifications are removable by
   * default.
   *
   * @deprecated: Define isAutoRemovable on item.content instead.
   */
  isAutoRemovable?: boolean;
}

const autoRemoveDurationMs = 5_000; // 5s

export function Notification(props: NotificationProps) {
  const {
    item,
    onRemove,
    // TODO(ravicious): Remove isAutoRemovable in favor of item.content.isAutoRemovable.
    isAutoRemovable: isAutoRemovableProp,
    ...styleProps
  } = props;
  const content = toObjectContent(item.content);
  const [isHovered, setIsHovered] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const timeoutHandler = useRef<number>();
  const theme = useTheme();

  const isAutoRemovable =
    getContentIsAutoRemovable(item.content) ??
    isAutoRemovableProp ??
    ['success', 'info', 'neutral'].includes(item.severity);
  useEffect(() => {
    if (!isHovered && isAutoRemovable) {
      timeoutHandler.current = setTimeout(
        onRemove,
        autoRemoveDurationMs
      ) as unknown as number;
    }

    return () => {
      if (timeoutHandler.current) {
        clearTimeout(timeoutHandler.current);
      }
    };
  }, [isHovered, isAutoRemovable]);

  function toggleIsExpanded() {
    setIsExpanded(wasExpanded => !wasExpanded);
  }

  const { borderColor, iconColor } = notificationColors(theme, item.severity);

  return (
    <Container
      py={3}
      // We use a custom value to offset the default padding by the width of the
      // left border.
      pl="12px"
      pr={3}
      borderColor={borderColor}
      onMouseOver={() => {
        if (isHovered === false) {
          setIsHovered(true);
        }
      }}
      onMouseLeave={() => {
        if (isHovered === true) {
          setIsHovered(false);
        }
      }}
      onClick={toggleIsExpanded}
      {...styleProps}
    >
      <Flex gap={2} flexDirection="column">
        <Flex flexDirection="row" gap={2}>
          <NotificationIcon
            severity={item.severity}
            size="medium"
            color={iconColor}
            customIcon={content.icon}
          />
          {/* Right margin leaves room for the close button. Note that we
              wouldn't have to do it if the close button was in layout, but we
              would have to nudge it a bit to top-right anyway, and it would
              then occupy too much vertical space where it used to be, causing
              other problems. */}
          <Box flex="1" mr={4}>
            <Text typography="h3">{content.title}</Text>
            <Text typography="body3">{content.subtitle}</Text>
          </Box>
        </Flex>
        <NotificationBody content={content} isExpanded={isExpanded} />
      </Flex>
      <CloseIcon
        style={{
          visibility: isHovered ? 'visible' : 'hidden',
        }}
        onClick={e => {
          e.stopPropagation();
          onRemove();
        }}
      >
        <Cross size="small" />
      </CloseIcon>
    </Container>
  );
}

const NotificationIcon = ({
  severity,
  customIcon: CustomIcon,
  ...otherProps
}: {
  severity: NotificationSeverity;
  customIcon: React.ComponentType<IconProps>;
} & IconProps) => {
  const commonProps = { role: 'graphics-symbol', ...otherProps };
  if (CustomIcon) {
    return <CustomIcon {...commonProps} />;
  }
  switch (severity) {
    case 'success':
      return <Icon.Checks aria-label="Success" {...commonProps} />;
    case 'error':
      return <Icon.WarningCircle aria-label="Danger" {...commonProps} />;
    case 'info':
      return <Icon.Info aria-label="Info" {...commonProps} />;
    case 'warn':
      return <Icon.Warning aria-label="Warning" {...commonProps} />;
    case 'neutral':
      return <Icon.Notification aria-label="Note" {...commonProps} />;
    default:
      severity satisfies never;
  }
};

const toObjectContent = (
  content: NotificationItemContent
): NotificationItemObjectContent =>
  typeof content === 'string' ? { title: content } : content;

const getContentIsAutoRemovable = (
  content: NotificationItemContent
): boolean | undefined =>
  typeof content === 'string' ? undefined : content.isAutoRemovable;

const notificationColors = (theme: Theme, severity: NotificationSeverity) => {
  switch (severity) {
    case 'neutral':
      return {
        borderColor: theme.colors.interactive.tonal.neutral[2],
        iconColor: theme.colors.text.main,
      };
    case 'error':
      return {
        borderColor: theme.colors.interactive.solid.danger.default,
        iconColor: theme.colors.interactive.solid.danger.default,
      };
    case 'warn':
      return {
        borderColor: theme.colors.interactive.solid.alert.default,
        iconColor: theme.colors.interactive.solid.alert.default,
      };
    case 'info':
      return {
        borderColor: theme.colors.interactive.solid.accent.default,
        iconColor: theme.colors.interactive.solid.accent.default,
      };
    case 'success':
      return {
        borderColor: theme.colors.interactive.solid.success.default,
        iconColor: theme.colors.interactive.solid.success.default,
      };
    default:
      severity satisfies never;
  }
};

const CloseIcon = styled(ButtonIcon)`
  /* Place the close button so that its hover state circle "overflows" the layout
   * padding a bit, placing the close icon itself in alignment with the padding. */
  position: absolute;
  top: ${props => props.theme.space[2]}px;
  right: ${props => props.theme.space[2]}px;
`;

const NotificationBody = ({
  content,
  isExpanded,
}: {
  content: NotificationItemObjectContent;
  isExpanded: boolean;
}) => {
  const longerTextCss = isExpanded ? textCss : shortTextCss;
  const hasListOrDescription = !!content.list || !!content.description;

  const { action } = content;

  return (
    <>
      {/* Note: an empty <Text/> element would still generate a flex gap, so we
          only render it if necessary. */}
      {hasListOrDescription && (
        <Text typography="body2" color="text.slightlyMuted" css={longerTextCss}>
          {content.list && <List items={content.list} />}
          {content.description}
        </Text>
      )}
      {action && (
        <Box alignSelf="flex-start">
          <ActionButton
            intent="neutral"
            action={{
              href: action.href,
              content: action.content,
              onClick: event => {
                // Prevents toggling the isExpanded flag.
                event.stopPropagation();
                action.onClick?.(event);
              },
            }}
          />
        </Box>
      )}
    </>
  );
};

function List(props: { items: string[] }) {
  return (
    <ul
      // Ideally we'd align the bullet point to the left without using list-style-position: inside
      // (because it looks bad when the list item spans multiple lines).
      //
      // However, it seems impossible to use padding-inline-start for that because the result looks
      // different on Retina vs non-Retina screens, the bullet point looks cut off on the latter if
      // padding-inline-start is set to 1em. So instead we just set it to 2em.
      css={`
        margin: 0;
        padding-inline-start: 2em;
      `}
    >
      {props.items.map((item, index) => (
        <li key={index}>{item}</li>
      ))}
    </ul>
  );
}

const textCss = `
  overflow-wrap: anywhere;
  white-space: pre-line;
`;

const shortTextCss = `
  ${textCss};
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
`;

const Container = styled(Box)`
  /* Positioning anchor for the close button. */
  position: relative;
  background: ${props => props.theme.colors.levels.elevated};
  border-left: ${props => props.theme.borders[3]};
  width: 320px;
  box-shadow:
    0px 3px 5px -1px rgba(0, 0, 0, 0.2),
    0px 6px 10px 0px rgba(0, 0, 0, 0.14),
    0px 1px 18px 0px rgba(0, 0, 0, 0.12);
  color: ${props => props.theme.colors.text.main};
  border-radius: ${props => props.theme.radii[3]}px;
  cursor: pointer;
  // Break up long addresses.
  word-break: break-word;

  ${borderColor}
`;
