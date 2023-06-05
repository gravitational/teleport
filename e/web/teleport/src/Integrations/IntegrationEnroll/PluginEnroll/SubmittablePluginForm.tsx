import React, { FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { ButtonPrimary, ButtonSecondary, Box, Flex, Text, Alert } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getXCSRFToken } from 'teleport/services/api';
import { Plugin } from 'teleport/services/integrations';

import cfg from 'e-teleport/config';
import { pluginsService } from 'e-teleport/services/plugins';

import { HostedPlugin } from './plugins';

// SubmittablePluginForm is a form that will use the default form submission event
// if the plugin is an `OAuth` plugin. Otherwise it will send off a conventional
// fetch request.
export function SubmittablePluginForm({
  plugin,
  eventId,
  setStaticPluginResponse,
}: {
  plugin: HostedPlugin;
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

  return (
    <Box mt={3}>
      <Text fontWeight="bold" typography="h4">
        {plugin.fullName}
      </Text>
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} mb={3} mt={3} />
      )}
      {plugin.Description && <plugin.Description />}
      {plugin.permissions?.length && (
        <Flex gap={6} p={4} bg="levels.surface" borderRadius={2} mt={3} mb={4}>
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
      )}
      <Box mt={3}>
        <Validation>
          {/* A "normal" HTTP form is used here instead of an AJAX request,
          since the user needs to be redirected to the OAuth provider after submitting. */}
          {({ validator }) => (
            <form
              action={cfg.getPluginUrl()}
              onSubmit={e => onSubmit(validator, e)}
              method="POST"
            >
              <input type="hidden" name="csrf_token" value={getXCSRFToken()} />
              <input type="hidden" name="event_id" value={eventId} />
              {/* TODO: make this a proper input and validate against existing instances
          when we start allowing multiple instances per type */}
              <input
                type="hidden"
                name="name"
                value={`${plugin.type}-default`}
              />
              <input type="hidden" name="type" value={plugin.type} />
              <Box>{plugin.FormMixin && <plugin.FormMixin />}</Box>
              <Box mt={6}>
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
  );
}
