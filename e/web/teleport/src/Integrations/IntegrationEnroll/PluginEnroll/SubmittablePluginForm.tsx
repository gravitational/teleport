import { FormEvent, useState } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
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
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getXCSRFToken } from 'teleport/services/api';
import { Plugin } from 'teleport/services/integrations';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { getCTAForPlugin, pluginsService } from 'e-teleport/services/plugins';

import { CleanupDialogue } from './MultiStep/Okta/CleanupDialogue'; // SubmittablePluginForm is a form that will use the default form submission event

import type { CloudHostablePlugin } from 'e-teleport/services/plugins';

// SubmittablePluginForm is a form that will use the default form submission event
// if the plugin is an `OAuth` plugin. Otherwise it will send off a conventional
// fetch request.
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
    if (!validator.validate()) {
      e.preventDefault();
      return;
    }

    if (!plugin.isOAuth) {
      // Prevent browser from reloading the page from the form submission event.
      e.preventDefault();

      // Success states will not be set because it's not required
      // to trigger a re-render (by setting the attempt to "success").
      // After setting plugin response, the outer component
      // will take user to a different state outside of this component.
      setAttempt({ status: 'processing' });

      let formData = new FormData(e.currentTarget as HTMLFormElement);

      try {
        // Currently, only the following plugins support validating and cleaning up.
        if (plugin.type === 'okta' || plugin.type === 'entra-id') {
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
        await pluginsService.createPlugin(formData).then(resp => {
          setStaticPluginResponse(resp);
        });
      } catch (e) {
        const msg = getErrMessage(e);
        setAttempt({ status: 'failed', statusText: msg });
      }
    }

    // Else let the default form submission event occur (eg: slack)
  }

  const pluginRequiresPermission =
    plugin.disabledIfNoMdmSupport &&
    !cfg.oss.entitlements.MobileDeviceManagement.enabled;
  let wrapperStyle;
  if (pluginRequiresPermission) {
    // blurs the form
    wrapperStyle = {
      filter: 'blur(2px)',
      pointerEvents: 'none',
      userSelect: 'none',
    };
  }

  const isPartOfMultiStep = setFormData && !setStaticPluginResponse;

  return (
    <Box mt={CustomTitle ? 0 : 3} style={{ position: 'relative' }}>
      {CustomTitle ? <>{CustomTitle}</> : <H1 my={3}>{plugin.fullName}</H1>}
      {plugin.Description && <plugin.Description />}
      <Box style={wrapperStyle}>
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
      {pluginRequiresPermission && (
        <StyledMessageContainer>
          Unlock {plugin.name} plugin with Teleport Enterprise{' '}
          <ButtonLockedFeature
            width="auto"
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
  box-shadow:
    0 5px 5px -3px rgba(0, 0, 0, 0.2),
    0 8px 10px 1px rgba(0, 0, 0, 0.14),
    0 3px 14px 2px rgba(0, 0, 0, 0.12);
  border-radius: 8px;
`;
