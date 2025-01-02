import { Box, Indicator, Text } from 'design';

import ErrorMessage from 'teleport/components/AgentErrorMessage';

import { Edit } from './Edit';

export default {
  title: 'TeleportE/SamlApplication/components/Edit',
};

export const Default = () => {
  const attempt = {
    status: 'success',
    data: null,
    statusText: '',
    error: '',
  };
  return <Edit {...props} Content={() => EditContent(attempt)} />;
};

export const Processing = () => {
  const attempt = {
    status: 'processing',
    data: null,
    statusText: '',
    error: 'err',
  };
  return <Edit {...props} Content={() => EditContent(attempt)} />;
};

export const Error = () => {
  const attempt = {
    status: 'error',
    data: null,
    statusText: 'Error while fetching SAML resource',
    error: 'err',
  };

  return <Edit {...props} Content={() => EditContent(attempt)} />;
};

function EditContent(attempt) {
  switch (attempt.status) {
    case '':
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'error':
      return <ErrorMessage message={attempt.statusText} />;
    case 'success':
      return (
        <Box textAlign="center" m={10}>
          <Text>I render inside Edit Dialog</Text>
        </Box>
      );
  }
}

const props = {
  open: true,
  onClose: () => null,
};
