import { Link } from 'react-router-dom';

import { Box, ButtonPrimary, ButtonSecondary, Flex, H2, Image } from 'design';
import pamSuccess from 'design/assets/images/icons/success.png';

import cfg from 'e-teleport/config';

import { useCreateAccessList } from './CreateAccessListContextProvider';

export function Finished() {
  const { reset } = useCreateAccessList();

  return (
    <Flex flexDirection="column" alignItems="center" mt={6} gap={2}>
      <Image src={pamSuccess} maxWidth="120px" />
      <H2 mt={3} mb={2}>
        Access List Successfully Created!
      </H2>
      <Box maxWidth="450px" textAlign="center" mb={1}>
        The users added to this access list will receive permission grants upon
        their next log in.
      </Box>
      <Flex gap={3}>
        <ButtonPrimary as={Link} to={cfg.getAccessListManagementRoute()}>
          Browse Access Lists
        </ButtonPrimary>
        <ButtonSecondary onClick={reset}>
          Add Another Access List
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}
