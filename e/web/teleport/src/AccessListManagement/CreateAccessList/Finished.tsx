import { Link } from 'react-router';

import {
  Box,
  ButtonBorder,
  ButtonPrimary,
  Flex,
  H2,
  Image,
  Text,
} from 'design';
import pamSuccess from 'design/assets/images/icons/success.png';

import cfg from 'e-teleport/config';

import { useCreateAccessList } from './CreateAccessListContextProvider';

export function Finished() {
  const createContext = useCreateAccessList();
  const { createdAccessList } = createContext;

  return (
    <Flex flexDirection="column" alignItems="center" mt={6} gap={2}>
      <Image src={pamSuccess} maxWidth="120px" />
      <H2 mt={3} mb={2}>
        {createdAccessList.title} Successfully Created!
      </H2>
      <Box maxWidth="550px" textAlign="center" mb={1}>
        <Text>
          Access list{' '}
          <Text bold as="span">
            {createdAccessList.title}
          </Text>{' '}
          is configured and ready to use.
        </Text>
        <Text>
          Note: Users must re-authenticate to gain the access granted by this
          list.
        </Text>
      </Box>
      <Flex gap={3}>
        <ButtonPrimary
          as={Link}
          to={cfg.getAccessListManagementRoute(createdAccessList.id)}
        >
          View Created List
        </ButtonPrimary>
        <ButtonBorder
          intent="primary"
          as={Link}
          to={cfg.getAccessListManagementRoute()}
        >
          Browse Access Lists
        </ButtonBorder>
      </Flex>
    </Flex>
  );
}
