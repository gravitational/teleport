import { generatePath } from 'react-router';
import { Flex, Text, Image, ButtonPrimary, ButtonSecondary, H2 } from 'design';

import cfg from 'teleport/config';
import history from 'teleport/services/history';

import celebratePamPng from 'teleport/Discover/Shared/Finished/celebrate-pam.png';

import { useDiscover } from 'teleport/Discover/useDiscover';
import { encodeUrlQueryParams } from 'teleport/components/hooks/useUrlFiltering';

export function Finished() {
  const { exitFlow, agentMeta, isUpdateFlow } = useDiscover();

  const statusText = isUpdateFlow ? 'Updated' : 'Added';
  let resourceText;
  if (agentMeta?.resourceName) {
    resourceText = `SAML Application [${agentMeta.resourceName}] has been successfully ${statusText.toString().toLowerCase()} to
        this Teleport Cluster.`;
  }

  return (
    <Flex
      width="600px"
      flexDirection="column"
      alignItems="center"
      css={`
        margin: 0 auto;
        text-align: center;
      `}
    >
      <Image width="120px" height="120px" src={celebratePamPng} />
      <H2 mt={3} mb={2}>
        SAML Application Successfully {statusText}.
      </H2>
      {!isUpdateFlow && (
        <>
          <Text mb={3}>
            {resourceText} You can now use Teleport as an identity provider to
            log into it.
          </Text>
          <Flex>
            <ButtonPrimary
              width="270px"
              size="large"
              onClick={() =>
                history.push(
                  encodeUrlQueryParams({
                    pathname: generatePath(cfg.routes.unifiedResources, {
                      clusterId: cfg.proxyCluster,
                    }),
                    kinds: ['app'],
                  }),
                  true
                )
              }
              mr={3}
            >
              Browse Applications
            </ButtonPrimary>
            <ButtonSecondary
              width="270px"
              size="large"
              onClick={() => exitFlow()}
            >
              Add Another Resource
            </ButtonSecondary>
          </Flex>
        </>
      )}
    </Flex>
  );
}
