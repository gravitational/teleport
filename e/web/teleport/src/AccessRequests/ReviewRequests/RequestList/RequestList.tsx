import { Text, LabelState, Flex } from 'design';
import { Cell } from 'design/DataTable';
import { ArrowFatLinesUp } from 'design/Icon';

import { AccessRequest } from 'e-teleport/services/accessRequests';

export const renderUserCell = ({ user }: AccessRequest) => {
  return (
    <Cell
      style={{
        maxWidth: '100px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={user}
    >
      {user}
    </Cell>
  );
};

export const renderIdCell = ({ id }: AccessRequest) => {
  return (
    <Cell
      style={{
        maxWidth: '100px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={id}
    >
      {id.slice(-5)}
    </Cell>
  );
};

export const renderStatusCell = ({ state }: AccessRequest) => {
  if (state === 'PROMOTED') {
    return (
      <Cell>
        <Flex alignItems="center">
          <ArrowFatLinesUp size={17} color="success.main" mr={1} ml="-3px" />
          <Text typography="body2">{state}</Text>
        </Flex>
      </Cell>
    );
  }

  let kind = 'warning';
  if (state === 'APPROVED') {
    kind = 'success';
  } else if (state === 'DENIED') {
    kind = 'danger';
  }

  return (
    <Cell>
      <Flex alignItems="center">
        <LabelState
          kind={kind}
          mr={2}
          width="10px"
          p={0}
          style={{ minHeight: '10px' }}
        />
        <Text typography="body2">{state}</Text>
      </Flex>
    </Cell>
  );
};
