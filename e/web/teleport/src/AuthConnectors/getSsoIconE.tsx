import { Check } from 'design/Icon';
import { AuthProviderType } from 'shared/services';

import Text from 'design/Text';

import Flex from 'design/Flex';
import getSsoIcon from 'teleport/AuthConnectors/ssoIcons/getSsoIcon';

export default function getSsoIconE(
  kind: AuthProviderType,
  isFeatureLocked: boolean
) {
  switch (kind) {
    case 'github':
      const gh = getSsoIcon('github');

      return {
        ...gh,
        info: isFeatureLocked ? (
          <Flex alignItems="center">
            <Check size="small" color="#00bfa5"></Check>
            <Text ml="2">Included with Teleport Team Plan</Text>
          </Flex>
        ) : (
          'Sign in using your GitHub account'
        ),
      };
    case 'saml':
      return getSsoIcon('saml');
    case 'oidc':
    default:
      return getSsoIcon('oidc');
  }
}
