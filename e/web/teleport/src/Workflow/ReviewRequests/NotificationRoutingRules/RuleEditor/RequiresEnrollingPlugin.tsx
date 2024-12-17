import { Link as InternalLink } from 'react-router-dom';
import { Text, ButtonPrimary, Link as ExternalLink, Flex } from 'design';
import { Info } from 'design/Alert/Alert';
import cfg from 'teleport/config';

export function RequiresEnrollingPlugin() {
  return (
    <Info
      css={`
        flex-direction: column;
        a.external-link {
          color: ${({ theme }) => theme.colors.buttons.link.default};
        }
      `}
    >
      <Text>
        Access Request Notification Rules are only supported for hosted Slack,
        Mattermost, Email, Microsoft, and Datadog. Support for more integrations
        is coming soon. Check out our{' '}
        <ExternalLink
          href="https://goteleport.com/docs/upcoming-releases/"
          target="_blank"
          className="external-link"
        >
          release
        </ExternalLink>{' '}
        page for future updates.
      </Text>
      <Flex gap={2} alignItems="center" mt={2}>
        <ButtonPrimary
          as={InternalLink}
          to={cfg.getIntegrationEnrollRoute()}
          size="large"
          my={2}
        >
          Enroll an Integration
        </ButtonPrimary>
        or
        <ExternalLink
          href="https://github.com/gravitational/teleport/issues/new?assignees=&labels=feature-request&template=feature_request.md"
          target="_blank"
          className="external-link"
        >
          Request a feature
        </ExternalLink>
      </Flex>
    </Info>
  );
}
