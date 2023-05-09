import React from 'react';
import styled from 'styled-components';
import { Github, Check } from 'design/Icon';
import { AuthProviderType } from 'shared/services';

import Text from 'design/Text';

import Flex from 'design/Flex';

import Box from 'design/Box';
import Image from 'design/Image';

import iconAuth0 from './assets/saml-auth0.svg';
import iconAzureAD from './assets/saml-azuread.svg';
import iconOkta from './assets/saml-okta.svg';
import iconOneLogin from './assets/saml-one.svg';

import iconAmazon from './assets/oidc-amazon.svg';
import iconGitlab from './assets/oidc-gitlab.svg';
import iconGoogle from './assets/oidc-google.svg';
import iconWindows from './assets/oidc-windows.svg';

export default function getSsoIcon(
  kind: AuthProviderType,
  isFeatureLocked: boolean
) {
  const desc = formatConnectorTypeDesc(kind);
  if (kind === 'github') {
    return {
      SsoIcon: props => (
        <Github
          style={{ textAlign: 'center' }}
          fontSize="50px"
          color="text.main"
          {...props}
        />
      ),
      desc,
      info: isFeatureLocked && (
        <Flex alignItems="center">
          <Check fontSize="16px" color="#00bfa5"></Check>
          <Text ml="2">Included with Teleport Team Plan</Text>
        </Flex>
      ),
    };
  }

  if (kind === 'saml') {
    return {
      SsoIcon: () => (
        <MultiIconContainer>
          <SmIcon>
            <Image src={iconOneLogin} />
          </SmIcon>
          <SmIcon>
            <Image src={iconOkta} />
          </SmIcon>
          <SmIcon mt="1">
            <Image src={iconAuth0} />
          </SmIcon>
          <SmIcon mt="1">
            <Image src={iconAzureAD} />
          </SmIcon>
        </MultiIconContainer>
      ),
      desc,
      info: 'Okta, OneLogin, Azure Active Directory, etc.',
    };
  }

  // default is OIDC icon
  return {
    SsoIcon: () => (
      <MultiIconContainer>
        <SmIcon>
          <Image src={iconAmazon} />
        </SmIcon>
        <SmIcon>
          <Image src={iconGoogle} />
        </SmIcon>
        <SmIcon mt="1">
          <Image src={iconGitlab} />
        </SmIcon>
        <SmIcon mt="1">
          <Image src={iconWindows} />
        </SmIcon>
      </MultiIconContainer>
    ),
    desc,
    info: 'Google, GitLab, Amazon and more',
  };
}

function formatConnectorTypeDesc(kind) {
  kind = kind || '';
  if (kind == 'github') {
    return `GitHub Connector`;
  }
  kind = kind.toUpperCase();
  return `${kind} Connector`;
}

const MultiIconContainer = styled(Flex)`
  width: 67px;
  flex-wrap: wrap;
  gap: 3px;
  padding: 7px;
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 8px;
`;

const SmIcon = styled(Box)`
  width: 24px;
  height: 24px;
  line-height: 24px;
  background: white;
  color: black;
  border-radius: 50%;
`;
