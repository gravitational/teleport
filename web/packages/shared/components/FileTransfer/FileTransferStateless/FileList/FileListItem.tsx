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

import { FC, PropsWithChildren, useEffect } from 'react';
import styled from 'styled-components';

import { ButtonIcon, Flex, Text } from 'design';
import { CircleCheck, Cross, Warning } from 'design/Icon';

import { TransferredFile } from '../types';

type FileListItemProps = {
  file: TransferredFile;
  onCancel(id: string): void;
};

export function FileListItem(props: FileListItemProps) {
  const { name, transferState, id } = props.file;

  useEffect(() => {
    return () => props.onCancel(id);
  }, [props.onCancel]);

  return (
    <Li>
      <Flex justifyContent="space-between" alignItems="center">
        <Flex alignItems="center">
          <Text
            typography="body3"
            css={`
              word-break: break-all;
            `}
          >
            {name}
          </Text>
          {transferState.type === 'completed' && (
            <CircleCheck
              ml={2}
              size="medium"
              color="progressBarColor"
              title="Transfer completed"
            />
          )}
        </Flex>
        {transferState.type === 'processing' && (
          <ButtonIcon
            title="Cancel"
            size={0}
            // prevents the icon from changing the height of the line
            mt="-4px"
            mb="-4px"
            onClick={() => props.onCancel(id)}
          >
            <Cross size="small" />
          </ButtonIcon>
        )}
      </Flex>
      {(transferState.type === 'processing' ||
        transferState.type === 'error') && (
        <Flex alignItems="baseline" mt={1}>
          <ProgressPercentage mr={1}>
            {transferState.progress}%
          </ProgressPercentage>
          <ProgressBackground>
            <ProgressIndicator
              progress={transferState.progress}
              isFailure={transferState.type === 'error'}
            />
          </ProgressBackground>
        </Flex>
      )}
      {transferState.type === 'error' && (
        <Error>{transferState.error.message}</Error>
      )}
    </Li>
  );
}

const Error: FC<PropsWithChildren> = props => {
  return (
    <Flex alignItems="center" mt={1}>
      <Warning size="small" mr={1} color="inherit" />
      <Text color="error.hover" typography="body3">
        {props.children}
      </Text>
    </Flex>
  );
};

const ProgressPercentage = styled(Text)`
  line-height: 16px;
  width: 36px;
`;

const Li = styled.li`
  list-style: none;
  margin-top: ${props => props.theme.space[3]}px;
  font-size: ${props => props.theme.fontSizes[1]}px;
`;

const ProgressBackground = styled.div`
  border-radius: 50px;
  background: ${props => props.theme.colors.spotBackground[0]};
  width: 100%;
`;

const ProgressIndicator = styled.div<{ progress: number; isFailure?: boolean }>`
  border-radius: 50px;
  background: ${props =>
    props.isFailure
      ? props.theme.colors.disabled
      : props.theme.colors.progressBarColor};

  height: 8px;
  width: ${props => props.progress}%;
`;
