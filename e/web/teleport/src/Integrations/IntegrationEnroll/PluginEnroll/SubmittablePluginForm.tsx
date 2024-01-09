import React, { FormEvent } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import { ButtonPrimary, ButtonSecondary, Box, Flex, Text, Alert } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';
import Validation, { Validator } from 'shared/components/Validation';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getXCSRFToken } from 'teleport/services/api';
import { Plugin } from 'teleport/services/integrations';

import cfg from 'e-teleport/config';
import { pluginsService, getCTAForPlugin } from 'e-teleport/services/plugins';

import { CloudHostablePlugin } from './plugins';

// SubmittablePluginForm is a form that will use the default form submission event
// if the plugin is an `OAuth` plugin. Otherwise it will send off a conventional
// fetch request.
export function SubmittablePluginForm({
  plugin,
  eventId,
  setStaticPluginResponse,
}: {
  plugin: CloudHostablePlugin;
  eventId: string;
  setStaticPluginResponse(createdPlugin: Plugin): void;
}) {
  const { attempt, setAttempt } = useAttempt(''); // only for non-oauth submissions.

  function onSubmit(validator: Validator, e: FormEvent<HTMLFormElement>) {
    if (!validator.validate()) {
      e.preventDefault();
      return;
    }

    if (!plugin.isOAuth) {
      // Prevent browser from reloading the page from the form submission event.
      e.preventDefault();

      setAttempt({ status: 'processing' });

      // Read the form data.
      const formData = new FormData(e.currentTarget as HTMLFormElement);

      // Send off the conventional fetch request.
      pluginsService
        .createPlugin(formData)
        // No need to trigger a re-render by setting the attempt to "success".
        // After setting of a plugin response, it will re-render to the
        // success state.
        .then(setStaticPluginResponse)
        .catch((err: Error) => {
          setAttempt({ status: 'failed', statusText: err.message });
        });
      return;
    }

    // Else let the default form submission event occur.
  }

  const pluginRequiresEnterprise = plugin.disableForTeam && cfg.oss.isTeam;
  let wrapperStyle;
  if (pluginRequiresEnterprise) {
    // blurs the form
    wrapperStyle = {
      filter: 'blur(2px)',
      pointerEvents: 'none',
      userSelect: 'none',
    };
  }

  return (
    <Box mt={3} style={{ position: 'relative' }}>
      <Text my={1} fontSize={4} bold>
        {plugin.fullName}
      </Text>
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} mb={3} mt={3} />
      )}
      {plugin.Description && <plugin.Description />}
      <Box style={wrapperStyle}>
        {plugin.permissions?.length && (
          <>
            <Text fontSize={2} bold>
              Required permissions:
            </Text>
            <Flex
              gap={6}
              p={4}
              bg="levels.surface"
              borderRadius={2}
              mt={1}
              mb={4}
            >
              {plugin.permissions.map((perm, index) => (
                <Box key={index}>
                  <Text fontWeight="bold" typography="h6" mb={2}>
                    {perm.category}
                  </Text>
                  {perm.permissions.map(p => (
                    <Flex key={`${index}${p.title}`} alignItems="center">
                      {p.title}{' '}
                      {p.description && (
                        <Flex ml={1}>
                          <ToolTipInfo>{p.description}</ToolTipInfo>
                        </Flex>
                      )}
                    </Flex>
                  ))}
                </Box>
              ))}
            </Flex>
          </>
        )}
        {plugin.Setup && (
          <>
            <Text fontSize={4} bold>
              Set up the plugin
            </Text>
            <plugin.Setup />
          </>
        )}
        <Box mt={4}>
          <Text mb={2} fontSize={4} bold>
            Configure and connect
          </Text>
          <Validation>
            {/* A "normal" HTTP form is used here instead of an AJAX request,
        since the user needs to be redirected to the OAuth provider after submitting. */}
            {({ validator }) => (
              <form
                action={cfg.getPluginUrl()}
                onSubmit={e => onSubmit(validator, e)}
                method="POST"
              >
                <input
                  type="hidden"
                  name="csrf_token"
                  value={getXCSRFToken()}
                />
                <input type="hidden" name="event_id" value={eventId} />
                <input
                  type="hidden"
                  name="name"
                  value={`${plugin.type}-default`}
                />
                <input type="hidden" name="type" value={plugin.type} />
                <Box>{plugin.FormMixin && <plugin.FormMixin />}</Box>
                <Box mt={6} mb={6}>
                  <ButtonPrimary
                    type="submit"
                    mr={3}
                    disabled={attempt.status === 'processing'}
                  >
                    Connect {plugin.name}
                  </ButtonPrimary>
                  <ButtonSecondary
                    as={Link}
                    to={cfg.oss.getIntegrationEnrollRoute()}
                  >
                    Back
                  </ButtonSecondary>
                </Box>
              </form>
            )}
          </Validation>
        </Box>
      </Box>
      {pluginRequiresEnterprise && (
        <StyledMessageContainer>
          Unlock {plugin.name} plugin with Teleport Enterprise{' '}
          <ButtonLockedFeature
            width="165px"
            mt={2}
            mb={1}
            event={getCTAForPlugin(plugin.type)}
          >
            Contact Sales
          </ButtonLockedFeature>
        </StyledMessageContainer>
      )}
    </Box>
  );
}

const StyledMessageContainer = styled(Flex)`
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background-color: ${({ theme }) => theme.colors.levels.elevated};
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 24px;
  gap: 24px;
  width: 600px;
  box-shadow: 0 5px 5px -3px rgba(0, 0, 0, 0.2),
    0 8px 10px 1px rgba(0, 0, 0, 0.14), 0 3px 14px 2px rgba(0, 0, 0, 0.12);
  border-radius: 8px;
`;
