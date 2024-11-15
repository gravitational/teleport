import { Warning } from 'design/Alert';
import Link from 'design/Link';
import { useTheme } from 'styled-components';

export function UserAccountWarning({
  inIdentitySourceScreen,
}: {
  inIdentitySourceScreen?: boolean;
}) {
  const theme = useTheme();
  return (
    <Warning linkColor={theme.colors.text.main}>
      {inIdentitySourceScreen ? identitySourceScreenCopy : firstScreenCopy}
    </Warning>
  );
}

const firstScreenCopy = (
  <>
    AWS IAM Identity Center integration requires setting up Teleport as an
    external identity source. To avoid access interruption, we recommend
    ensuring that all AWS IAM Identity Center users have access to this cluster
    before turning on the integration. If your AWS IAM Identity Center instance
    uses an external identity source (such as Okta), you can configure the
    corresponding{' '}
    <Link
      href="https://goteleport.com/docs/admin-guides/access-controls/sso/sso/"
      target="_blank"
    >
      SSO connector
    </Link>{' '}
    in Teleport.
  </>
);

const identitySourceScreenCopy = (
  <>
    To avoid access interruption, we recommend ensuring that all the AWS IAM
    Identity Center users have access to this cluster. If your AWS IAM Identity
    Center instance uses an external identity source (such as Okta), you can
    configure the corresponding{' '}
    <Link
      href="https://goteleport.com/docs/admin-guides/access-controls/sso/sso/"
      target="_blank"
    >
      SSO connector
    </Link>{' '}
    in Teleport.
  </>
);
