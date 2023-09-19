import React from 'react';
import { Link } from 'react-router-dom';

import { Text, Box, Flex, ButtonPrimary, ButtonBorder } from 'design';

import cfg from 'e-teleport/config';

export function EmptyState() {
  return (
    <Box
      p={8}
      pt={7}
      as={Flex}
      width="100%"
      mx="auto"
      alignItems="center"
      justifyContent="center"
    >
      <Box maxWidth={600}>
        <Box mb={4} textAlign="center">
          <Text typography="h5" mb={2} fontWeight={700} fontSize={24}>
            Add your first Access List
          </Text>
          <Text fontWeight={400} fontSize={14} style={{ opacity: '0.6' }}>
            Access Lists allows Teleport users to request longer lived access to
            resources with audit trails that describe access to those resources.
          </Text>
        </Box>
        <Box textAlign="center">
          <ButtonPrimary width="240px" as={Link} to={cfg.routes.accessListNew}>
            Create an Access List
          </ButtonPrimary>
          <ButtonBorder
            size="medium"
            as="a"
            // TODO(lisa): replace with non version prefixed link when updating
            // empty state to latest design.
            href="https://goteleport.com/docs/ver/14.x/access-controls/access-lists/"
            target="_blank"
            width="224px"
            ml={4}
            rel="noreferrer"
          >
            View Documentation
          </ButtonBorder>
        </Box>
      </Box>
    </Box>
  );
}
