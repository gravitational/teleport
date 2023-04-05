import React from 'react';
import styled from 'styled-components';
import { ButtonBorder, Box, Flex, Text } from 'design';
import * as Icons from 'design/Icon';

export const FileTransferRequests = (props: FileTransferRequestsProps) => {
  const { requests } = props;

  return (
    <Container>
      <Flex justifyContent="space-between" alignItems="baseline">
        <Text fontSize={3} bold>
          File Transfer Requests
        </Text>
      </Flex>
      {requests.map(request => (
        <Box mt={3} key={request.requestId}>
          <Text
            style={{
              wordBreak: 'break-word',
            }}
            mb={2}
          >{`${request.requester} is requesting ${request.direction} of file ${request.location}`}</Text>
          <Flex gap={2}>
            <ButtonBorder
              block
              onClick={() => props.onApprove(request.requestId, true)}
            >
              <Icons.Check fontSize="16px" mr={2} />
              Approve
            </ButtonBorder>
            <ButtonBorder
              block
              onClick={() => props.onDeny(request.requestId, false)}
            >
              <Icons.Cross fontSize="16px" mr={2} />
              Deny
            </ButtonBorder>
          </Flex>
        </Box>
      ))}
    </Container>
  );
};

export type FileTransferRequest = {
  requestId: string;
  requester: string;
  approvers: string[];
  shellCmd: string;
  location: string;
  direction: string;
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
