import React from 'react';
import styled from 'styled-components';
import { ButtonBorder, Box, Flex, Text } from 'design';
import * as Icons from 'design/Icon';

const OwnForm = ({ request, onCancel }: OwnFormProps) => {
  return (
    <Box mt={3} key={request.requestId}>
      <Flex alignItems="middle" justifyContent="space-between">
        <Text
          style={{
            wordBreak: 'break-word',
          }}
          mb={2}
        >{`Pending ${request.direction}: ${request.location}`}</Text>

        <ButtonBorder onClick={() => onCancel(request.requestId, false)}>
          <Icons.Cross fontSize="16px" />
        </ButtonBorder>
      </Flex>
    </Box>
  );
};

const ResponseForm = ({ request, onApprove, onDeny }: RequestFormProps) => {
  return (
    <Box mt={3} key={request.requestId}>
      <Text
        style={{
          wordBreak: 'break-word',
        }}
        mb={2}
      >{`${request.requester} is requesting ${request.direction} of file ${request.location}`}</Text>
      <Flex gap={2}>
        <ButtonBorder block onClick={() => onApprove(request.requestId, true)}>
          <Icons.Check fontSize="16px" mr={2} />
          Approve
        </ButtonBorder>
        <ButtonBorder block onClick={() => onDeny(request.requestId, false)}>
          <Icons.Cross fontSize="16px" mr={2} />
          Deny
        </ButtonBorder>
      </Flex>
    </Box>
  );
};

type RequestFormProps = {
  request: FileTransferRequest;
  onApprove: (string, bool) => void;
  onDeny: (string, bool) => void;
};

type OwnFormProps = {
  request: FileTransferRequest;
  onCancel: (string, bool) => void;
};

export const FileTransferRequests = (props: FileTransferRequestsProps) => {
  const { requests } = props;

  return (
    <Container>
      <Flex justifyContent="space-between" alignItems="baseline">
        <Text fontSize={3} bold>
          File Transfer Requests
        </Text>
      </Flex>
      {requests.map(request =>
        request.isOwnRequest ? (
          <OwnForm
            key={request.requestId}
            request={request}
            onCancel={props.onDeny}
          />
        ) : (
          <ResponseForm
            key={request.requestId}
            request={request}
            onApprove={props.onApprove}
            onDeny={props.onDeny}
          />
        )
      )}
    </Container>
  );
};

export type FileTransferRequest = {
  sid: string;
  requestId: string;
  requester: string;
  approvers: string[];
  location: string;
  filename?: string;
  size?: string;
  direction: 'download' | 'upload';
  isOwnRequest?: boolean;
};

type FileTransferRequestsProps = {
  requests: FileTransferRequest[];
  onApprove: (string, boolean) => void;
  onDeny: (string, boolean) => void;
};

const Container = styled.div`
  background: ${props =>
    props.backgroundColor || props.theme.colors.levels.surface};
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  box-sizing: border-box;
  border-radius: ${props => props.theme.radii[2]}px;
  margin-bottom: 8px;
  padding: 8px 16px 16px;
`;
