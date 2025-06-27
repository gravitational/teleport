import { ButtonBorder, ButtonPrimary } from 'design/Button';
import { CardTile } from 'design/CardTile';
import Flex from 'design/Flex';
import { AmazonAws } from 'design/Icon';
import { H3, P2 } from 'design/Text';

import { rolesAnywhereCreateProfile } from 'teleport/Integrations/Enroll/awsLinks';

export function ProfilesEmptyState() {
  return (
    <CardTile alignItems="center" gap={4}>
      <AmazonAws />
      {/*todo mberg add Company: AWS IAM Identity-and-Access-Management icon*/}
      <Flex flexDirection="column" alignItems="center">
        <H3 mb={1}>No AWS IAM Roles Anywhere Profiles Found</H3>
        <P2>Create AWS IAM Roles Anywhere Profiles in your AWS console</P2>
      </Flex>
      <Flex gap={3}>
        <ButtonPrimary as="a" target="blank" href={rolesAnywhereCreateProfile}>
          Create AWS Roles Anywhere Profiles
        </ButtonPrimary>
        <ButtonBorder intent="primary">
          Refresh AWS Roles Anywhere Profiles
        </ButtonBorder>
      </Flex>
    </CardTile>
  );
}
