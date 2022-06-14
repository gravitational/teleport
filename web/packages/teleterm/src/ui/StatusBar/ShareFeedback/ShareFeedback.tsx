import React, { useState } from 'react';
import { ButtonIcon, ButtonPrimary, Flex, Link, Text } from 'design';
import Validation from 'shared/components/Validation';
import { Close } from 'design/Icon';
import { ShareFeedbackForm } from './ShareFeedbackForm';
import { ShareFeedbackFormValues } from './types';
import { useAppContext } from 'teleterm/ui/appContextProvider';

interface ShareFeedbackProps {
  onClose(): void;
}

export function ShareFeedback(props: ShareFeedbackProps) {
  const ctx = useAppContext();
  ctx.workspacesService.useState();
  ctx.clustersService.useState();

  function getEmailFromUserName(): string {
    const cluster = ctx.clustersService.findCluster(
      ctx.workspacesService.getRootClusterUri()
    );
    const userName = cluster?.loggedInUser?.name;
    if (/^\S+@\S+$/.test(userName)) {
      return userName;
    }
  }

  const [formValues, setFormValues] = useState<ShareFeedbackFormValues>({
    feedback: '',
    email: getEmailFromUserName() || '',
    newsletterEnabled: false,
    salesContactEnabled: false,
  });

  return (
    <Flex bg="primary.main" p={3} borderRadius={3} maxWidth="370px">
      <Validation>
        {({ validator }) => (
          <Flex
            flexDirection="column"
            as="form"
            onSubmit={e => {
              e.preventDefault();
              if (validator.validate()) {
                console.log('submit', formValues); //TODO (gzdunek): connect to a real service
              }
            }}
          >
            <Flex justifyContent="space-between" mb={2}>
              <Text typography="h4" bold color="text.primary">
                Provide your feedback
              </Text>
              <ButtonIcon
                type="button"
                onClick={props.onClose}
                title="Close"
                color="text.secondary"
              >
                <Close fontSize={5} />
              </ButtonIcon>
            </Flex>
            <Link
              href="https://github.com/gravitational/teleport/issues/new?assignees=&labels=bug&template=bug_report.md"
              target="_blank"
            >
              Submit a Bug
            </Link>
            <Link href="https://goteleport.com/signup/" target="_blank">
              Try Teleport Cloud
            </Link>
            <ShareFeedbackForm
              formValues={formValues}
              setFormValues={setFormValues}
            />
            <ButtonPrimary block type="submit" mt={4}>
              Submit
            </ButtonPrimary>
          </Flex>
        )}
      </Validation>
    </Flex>
  );
}
