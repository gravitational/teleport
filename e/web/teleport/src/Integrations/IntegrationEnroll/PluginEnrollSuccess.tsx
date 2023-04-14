import React from 'react';
import { Link } from 'react-router-dom';
import { Box, ButtonPrimary, ButtonSecondary, Flex, Image, Text } from 'design';
import CardError from 'design/CardError';
import pamSuccess from 'design/assets/images/icons/success.png';

import cfg from 'e-teleport/config';

import { EnrollSuccessResponse, PluginType } from '../data';

export function PluginEnrollSuccess(props: State) {
  const { success, resolvedType } = props;

  let successData: EnrollSuccessResponse;
  try {
    successData = JSON.parse(success);
  } catch (e) {
    console.error(e);
    return <CardError>Could not parse the response.</CardError>;
  }

  return (
    <Flex flexDirection="column" alignItems="center" mt="6">
      <Image src={pamSuccess} maxWidth="120px" />
      <Text typography="h4" fontWeight="bold" my="2">
        {resolvedType.name} is integrated successfully
      </Text>
      <Box maxWidth="500px" textAlign="center">
        {resolvedType.hosted && resolvedType.NextSteps && (
          <resolvedType.NextSteps successData={successData} />
        )}
      </Box>

      <Flex gap="2" my="3">
        <Link to={cfg.oss.routes.integrations}>
          <ButtonPrimary>Go to Integration List</ButtonPrimary>
        </Link>
        <Link to={cfg.oss.getIntegrationEnrollRoute(null)}>
          <ButtonSecondary>Add Another Integration</ButtonSecondary>
        </Link>
      </Flex>
    </Flex>
  );
}

type State = {
  resolvedType: PluginType;
  success: string;
};
