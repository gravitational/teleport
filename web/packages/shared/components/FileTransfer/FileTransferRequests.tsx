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

import styled from 'styled-components';

import { Box, Button, ButtonBorder, Flex, Text } from 'design';
import * as Icons from 'design/Icon';

import { useConsoleContext } from 'teleport/Console/consoleContextProvider';
import {
  FileTransferRequest,
  isOwnRequest,
} from 'teleport/Console/DocumentSsh/useFileTransfer';
import { UserContext } from 'teleport/services/user';

type FileTransferRequestsProps = {
  requests: FileTransferRequest[];
  onApprove: (requestId: string, approved: boolean) => void;
  onDeny: (requestId: string, approved: boolean) => void;
};

export const FileTransferRequests = ({
  requests,
  onApprove,
  onDeny,
}: FileTransferRequestsProps) => {
  const ctx = useConsoleContext();
  const currentUser = ctx.getStoreUser();

  if (requests.length > 0) {
    return (
      <Container>
        <Flex justifyContent="space-between" alignItems="baseline">
          <Text fontSize={3} bold>
            File Transfer Requests
          </Text>
        </Flex>
        {requests.map(request =>
          isOwnRequest(request, currentUser.username) ? (
            <OwnForm
              key={request.requestID}
              request={request}
              onCancel={onDeny}
            />
          ) : (
            <ResponseForm
              key={request.requestID}
              request={request}
              onApprove={onApprove}
              onDeny={onDeny}
              currentUser={currentUser}
            />
          )
        )}
      </Container>
    );
  }

  // don't show dialog if no requests exist
  return null;
};

type OwnFormProps = {
  request: FileTransferRequest;
  onCancel: (requestId: string, approved: boolean) => void;
};

const OwnForm = ({ request, onCancel }: OwnFormProps) => {
  return (
    <Box mt={3} key={request.requestID}>
      <Flex alignItems="middle" justifyContent="space-between">
        <Text
          style={{
            wordBreak: 'break-word',
          }}
          mb={2}
        >
          {getOwnPendingText(request)}
        </Text>
        <ButtonBorder onClick={() => onCancel(request.requestID, false)}>
          <Icons.Cross size="small" />
        </ButtonBorder>
      </Flex>
    </Box>
  );
};

const getOwnPendingText = (request: FileTransferRequest) => {
  if (request.download) {
    return `Pending download: ${request.location}`;
  }
  return `Pending upload: ${request.filename} to ${request.location}`;
};

type RequestFormProps = {
  request: FileTransferRequest;
  onApprove: (requestId: string, approved: boolean) => void;
  onDeny: (requestId: string, approved: boolean) => void;
  currentUser: UserContext;
};

const ResponseForm = ({
  request,
  onApprove,
  onDeny,
  currentUser,
}: RequestFormProps) => {
  return (
    <Box mt={3} key={request.requestID}>
      <Text
        style={{
          wordBreak: 'break-word',
        }}
        mb={2}
      >
        {getPendingText(request)}
      </Text>
      <Flex gap={2}>
        <Button
          fill="border"
          intent="success"
          disabled={request.approvers.includes(currentUser.username)}
          block
          onClick={() => onApprove(request.requestID, true)}
        >
          <Icons.Check size="small" mr={2} />
          Approve
        </Button>
        <Button
          fill="border"
          intent="danger"
          block
          onClick={() => onDeny(request.requestID, false)}
        >
          <Icons.Cross size="small" mr={2} />
          Deny
        </Button>
      </Flex>
    </Box>
  );
};

const getPendingText = (request: FileTransferRequest) => {
  if (request.download) {
    return `${request.requester} wants to download ${request.location}`;
  }
  return `${request.requester} wants to upload ${request.filename} to ${request.location}`;
};

const Container = styled.div<{ backgroundColor?: string }>`
  background: ${props =>
    props.backgroundColor || props.theme.colors.levels.surface};
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  box-sizing: border-box;
  border-radius: ${props => props.theme.radii[2]}px;
  margin-bottom: 8px;
  padding: 8px 16px 16px;
`;
