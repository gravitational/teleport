import { Link } from 'react-router';

import { Alert, Box, ButtonPrimary, ButtonSecondary, Flex, H1 } from 'design';
import { ArrowBack } from 'design/Icon';
import Validation from 'shared/components/Validation';

import cfg from 'e-teleport/config';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { NoAccessState } from '../../NoAccessState';
import {
  FeatureLimitReached,
  featureLimitReachedBlurCss,
} from '../../Shared/FeatureLimitReached';
import { useCreateAccessList } from '../CreateAccessListContextProvider';
import { Finished } from '../Finished';
import { MembersSection } from '../MemberSection';
import { OwnersSection } from '../OwnerSection';
import { SpecSection } from '../SpecSection';

/**
 * CustomAccessList is a single step flow allowing
 * user to manually enter all fields however they like.
 */
export function CustomAccessList() {
  const { createAttempt } = useCreateAccessList();
  return (
    <FeatureBox>
      {createAttempt.status === 'success' ? (
        <Finished />
      ) : (
        <>
          <FeatureHeader alignItems="center" justifyContent="space-between">
            <FeatureHeaderTitle>
              <Flex alignItems="center">
                <ArrowBack
                  as={Link}
                  mr={2}
                  size="large"
                  color="text.main"
                  to={cfg.getAccessListManagementRoute()}
                />
                <H1>Create a New Access List</H1>
              </Flex>
            </FeatureHeaderTitle>
          </FeatureHeader>
          <MainContent />
        </>
      )}
    </FeatureBox>
  );
}

function MainContent() {
  const { onCreate, createAttempt, featureLimitReached, canCreateAccessList } =
    useCreateAccessList();

  if (!canCreateAccessList) {
    return <NoAccessState action="create" />;
  }

  return (
    <>
      {featureLimitReached && <FeatureLimitReached />}
      <Validation>
        {({ validator }) => (
          <Box
            width="540px"
            style={featureLimitReached ? featureLimitReachedBlurCss : null}
          >
            <Box mb={8}>
              <SpecSection />
            </Box>
            <Box mb={8}>
              <OwnersSection />
            </Box>
            <Box>
              <MembersSection />
            </Box>
            {createAttempt.status === 'failed' && (
              <Alert>{createAttempt.statusText}</Alert>
            )}
            <Box mt={4} mb={8}>
              <ButtonPrimary
                onClick={() => onCreate(validator)}
                mr={3}
                disabled={createAttempt.status === 'processing'}
              >
                Create Access List
              </ButtonPrimary>
              <ButtonSecondary
                as={Link}
                mt={3}
                to={cfg.getAccessListManagementRoute()}
              >
                Cancel
              </ButtonSecondary>
            </Box>
          </Box>
        )}
      </Validation>
    </>
  );
}
