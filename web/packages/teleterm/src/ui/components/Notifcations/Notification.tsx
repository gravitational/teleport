import React, { useEffect, useRef, useState } from 'react';
import styled, { css, useTheme } from 'styled-components';
import { ButtonIcon, Flex, Text } from 'design';
import { Close, Info, Warning } from 'design/Icon';
import { NotificationItem, NotificationItemContent } from './types';

interface NotificationProps {
  item: NotificationItem;

  onRemove(): void;
}

const notificationConfig: Record<
  NotificationItem['severity'],
  { Icon: React.ElementType; getColor(theme): string; isAutoRemovable: boolean }
> = {
  error: {
    Icon: Warning,
    getColor: theme => theme.colors.danger,
    isAutoRemovable: false,
  },
  warn: {
    Icon: Warning,
    getColor: theme => theme.colors.warning,
    isAutoRemovable: true,
  },
  info: {
    Icon: Info,
    getColor: theme => theme.colors.info,
    isAutoRemovable: true,
  },
};

const autoRemoveDurationMs = 5000;

export function Notification(props: NotificationProps) {
  const [isHovered, setIsHovered] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const timeoutHandler = useRef<number>();
  const config = notificationConfig[props.item.severity];
  const theme = useTheme();

  useEffect(() => {
    if (!isHovered && config.isAutoRemovable) {
      timeoutHandler.current = setTimeout(
        props.onRemove,
        autoRemoveDurationMs
      ) as unknown as number;
    }

    return () => {
      if (timeoutHandler.current) {
        clearTimeout(timeoutHandler.current);
      }
    };
  }, [isHovered]);

  function toggleIsExpanded() {
    setIsExpanded(wasExpanded => !wasExpanded);
  }

  const removeIcon = (
    <ButtonIcon
      size={0}
      ml={1}
      mr={-1}
      style={{ visibility: isHovered ? 'visible' : 'hidden' }}
    >
      <Close
        onClick={e => {
          e.stopPropagation();
          props.onRemove();
        }}
      />
    </ButtonIcon>
  );

  return (
    <Container
      py={2}
      pl={3}
      pr={2}
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
    >
      <Flex alignItems="center" mr={1} minWidth="0" width="100%">
        {config && (
          <config.Icon color={config.getColor(theme)} mr={3} fontSize={16} />
        )}
        {getRenderedContent(props.item.content, isExpanded, removeIcon)}
      </Flex>
    </Container>
  );
}

function getRenderedContent(
  content: NotificationItemContent,
  isExpanded: boolean,
  removeIcon: React.ReactNode
) {
  const longerTextCss = isExpanded ? textCss : shortTextCss;

  if (typeof content === 'string') {
    return (
      <Flex justifyContent="space-between">
        <Text
          typography="body1"
          fontSize={13}
          lineHeight={20}
          css={longerTextCss}
        >
          {content}
        </Text>
        {removeIcon}
      </Flex>
    );
  }
  if (typeof content === 'object') {
    return (
      <Flex flexDirection="column" minWidth="0" width="100%">
        <div
          css={`
            position: relative;
          `}
        >
          <Text
            fontSize={13}
            bold
            mr="30px"
            css={`
              line-height: 20px;
            `}
          >
            {content.title}
          </Text>
          <div
            css={`
              position: absolute;
              top: 0;
              right: 0;
            `}
          >
            {removeIcon}
          </div>
        </div>
        <Text
          fontSize={13}
          title={content.description}
          lineHeight={20}
          color="text.secondary"
          css={longerTextCss}
        >
          {content.description}
        </Text>
      </Flex>
    );
  }
}

const textCss = css`
  line-height: 20px;
  overflow-wrap: break-word;
`;

const shortTextCss = css`
  ${textCss};
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
`;

const Container = styled(Flex)`
  flex-direction: row;
  justify-content: space-between;
  background: ${props => props.theme.colors.primary.darker};
  min-height: 40px;
  width: 320px;
  margin-bottom: 15px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.24);
  color: ${props => props.theme.colors.text.primary};
  opacity: 0.95;
  border-radius: 4px;

  &:hover {
    opacity: 1;
    cursor: pointer;
  }
`;
