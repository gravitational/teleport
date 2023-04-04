import React from 'react';
import styled from 'styled-components';
import { ButtonPrimary, ButtonSecondary, Box, Flex, Text } from 'design';

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
        <Box mt={3} key={request.id}>
          <Text
            style={{
              wordBreak: 'break-word',
            }}
            mb={2}
          >{`${request.requester} is requesting ${request.direction} of file ${request.location}`}</Text>
          <Flex gap={2}>
            <ButtonPrimary block onClick={() => console.log('approve')}>
              Approve
            </ButtonPrimary>
            <ButtonSecondary block onClick={() => console.log('deny')}>
              Deny
            </ButtonSecondary>
          </Flex>
        </Box>
      ))}
    </Container>
  );
};

export type FileTransferRequest = {
  id: string;
  requester: string;
  approvers: string[];
  shellCmd: string;
  location: string;
  direction: string;
};

type FileTransferRequestsProps = {
  requests: FileTransferRequest[];
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
