import { FormEvent, useState, type JSX } from 'react';
import { Link } from 'react-router';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H1,
  H2,
  H3,
} from 'design';
import { IconTooltip } from 'design/Tooltip';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import {
  pluginsService,
  type CloudHostablePlugin,
} from 'e-teleport/services/plugins';
import { getXCSRFToken } from 'teleport/services/api';
import { Plugin } from 'teleport/services/integrations';

import { CleanupDialogue } from './MultiStep/Okta/CleanupDialogue';

/**
 * SubmittablePluginForm will make a fetch request with form data.
 *
 * Historically, for OAuth required plugins (e.g slack) we used the
 * browser default form submission which then the backend initiated
 * a meta redirect (to the auth providers URL). Now, the backend
 * returns the URL and the client does the redirecting.
 *
 * TODO:
 * Form data was initially used to avoid creating a type for each of
 * the various plugins we have, but we should refactor so that
 * each plugin has an explicit type to avoid type errors.
 */
export function SubmittablePluginForm({
  plugin,
  eventId,
  setStaticPluginResponse,
  setFormData,
  CustomTitle,
}: {
  plugin: CloudHostablePlugin;
  eventId: string;
  /**
   * Function to call after getting a successful response after plugin
   * installation api call.
   */
  setStaticPluginResponse?(createdPlugin: Plugin): void;
  /**
   * Only required if we need to persist FormData for later use.
   * eg: okta plugin installation can have more than this step
   * if IGS is enabled.
   */
  setFormData?(formData: FormData);
  CustomTitle?: JSX.Element;
}) {
  const { attempt, setAttempt } = useAttempt(''); // only for non-oauth submissions.
  const [showCleanUpModal, setShowCleanUpModal] = useState(false);

  async function onSubmit(validator: Validator, e: FormEvent<HTMLFormElement>) {
    // Prevent browser from reloading the page from the form submission event.
    e.preventDefault();

    if (!validator.validate()) {
      return;
    }

    // Success states will not be set because it's not required
    // to trigger a re-render (by setting the attempt to "success").
    // After setting plugin response, the outer component
    // will take user to a different state outside of this component.
    setAttempt({ status: 'processing' });
    let formData = new FormData(e.currentTarget as HTMLFormElement);

    // TODO(kshi36): replace global option with formData.get(PluginConfigBase.EnrollMethod)
    if (!plugin.isOAuth) {
      try {
        // Currently, only the following plugins support validating and cleaning up.
        if (plugin.type === 'entra-id') {
          await pluginsService.validatePlugin(formData);
          const required = await pluginsService.checkPluginRequiresCleanup(
            plugin.type
          );

          if (required) {
            setShowCleanUpModal(true);
            setAttempt({ status: '' });
            return;
          }
        }

        if (setFormData) {
          setFormData(formData);
        }

        if (!setStaticPluginResponse) {
          return;
        }

        // Send off the conventional fetch request to finish plugin
        // installation.
        await pluginsService.createStaticAuthPlugin(formData).then(resp => {
          setStaticPluginResponse(resp);
        });
      } catch (e) {
        const msg = getErrMessage(e);
        setAttempt({ status: 'failed', statusText: msg });
      }
    } else {
      // Handle plugin enrollment via OAuth 2.0 flow (eg: slack)
      try {
        await pluginsService.redirectForPluginOAuth(formData);
      } catch (err) {
        setAttempt({ status: 'failed', statusText: getErrMessage(err) });
      }
      return;
    }
  }

  const isPartOfMultiStep = setFormData && !setStaticPluginResponse;

  // Integrations with their own layout & setup.
  if (plugin.customSetup && plugin.Setup) {
    return <plugin.Setup />;
  }

  return (
    <Box mt={CustomTitle ? 0 : 3} style={{ position: 'relative' }}>
      {CustomTitle ? <>{CustomTitle}</> : <H1 my={3}>{plugin.fullName}</H1>}
      {plugin.Description && <plugin.Description />}
      <Box>
        {plugin.permissions?.length && (
          <>
            <H2 my={3}>Required permissions</H2>
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
                  <H3 mb={2}>{perm.category}</H3>
                  {perm.permissions.map(p => (
                    <Flex key={`${index}${p.title}`} alignItems="center">
                      {p.title}
                      {p.description && (
                        <Flex ml={1}>
                          <IconTooltip>{p.description}</IconTooltip>
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
            <H2>Set up the integration</H2>
            <plugin.Setup />
          </>
        )}
        <Box mt={4}>
          <H2 mb={2}>Configure and connect</H2>
          {attempt.status === 'failed' && (
            <Alert kind="danger" children={attempt.statusText} mb={3} mt={3} />
          )}
          {showCleanUpModal && (
            <CleanupDialogue
              onClose={() => setShowCleanUpModal(false)}
              pluginKind={plugin.type}
            />
          )}
          <Validation>
            {({ validator }) => (
              <form onSubmit={e => onSubmit(validator, e)} method="POST">
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
                <Box>
                  {plugin.FormMixin && <plugin.FormMixin attempt={attempt} />}
                </Box>
                <Box mt={6} mb={6}>
                  <ButtonPrimary
                    type="submit"
                    mr={3}
                    disabled={
                      attempt.status === 'processing' || showCleanUpModal
                    }
                  >
                    {isPartOfMultiStep ? 'Next' : `Connect ${plugin.name}`}
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
    </Box>
  );
}
