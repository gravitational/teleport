import React from 'react';
import styled from 'styled-components';
import { ButtonBorder, Box, Flex, Text } from 'design';
import * as Icons from 'design/Icon';

const OwnForm = ({ request, onCancel }: OwnFormProps) => {
  return (
    <Box mt={3} key={request.requestID}>
      <Flex alignItems="middle" justifyContent="space-between">
        <Text
          style={{
            wordBreak: 'break-word',
          }}
          mb={2}
        >{`Pending ${request.download ? 'download' : 'upload'}: ${
          request.location
        }${request.filename}`}</Text>

        <ButtonBorder onClick={() => onCancel(request.requestID, false)}>
          <Icons.Cross fontSize="16px" />
        </ButtonBorder>
      </Flex>
    </Box>
  );
};

const ResponseForm = ({ request, onApprove, onDeny }: RequestFormProps) => {
  return (
    <Box mt={3} key={request.requestID}>
      <Text
        style={{
          wordBreak: 'break-word',
        }}
        mb={2}
      >{`${request.requester} is requesting ${
        request.download ? 'download' : 'upload'
      } of file ${request.location}${request.filename}`}</Text>
      <Flex gap={2}>
        <ButtonBorder block onClick={() => onApprove(request.requestID, true)}>
          <Icons.Check fontSize="16px" mr={2} />
          Approve
        </ButtonBorder>
        <ButtonBorder block onClick={() => onDeny(request.requestID, false)}>
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
            key={request.requestID}
            request={request}
            onCancel={props.onDeny}
          />
        ) : (
          <ResponseForm
            key={request.requestID}
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
  requestID: string;
  requester: string;
  approvers: string[];
  location: string;
  filename?: string;
  size?: string;
  download: boolean;
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
