import { Fragment } from 'react';
import styled from 'styled-components';

import { Box, Flex, H2, H3, Stack, Text } from 'design';

import {
  AwsIcRoleConditions,
  StandardRoleConditions,
} from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/conditions';
import { desktopIdentities } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/resources/desktop';
import { gitHubIdentities } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/resources/github';
import { linuxDesktopIdentities } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/resources/linux_desktop';
import { serverIdentities } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/resources/server';
import { Labels } from 'teleport/services/resources';

import { definableResourceAccessFields } from '../../GuideEditor/Preset/role/listaccess';
import { appIdentityFieldNames } from '../../GuideEditor/Preset/role/resources/app';
import { dbIdentities } from '../../GuideEditor/Preset/role/resources/db';
import { kubeIdentities } from '../../GuideEditor/Preset/role/resources/kube';
import { Pills } from './Pills';
import { OutlineBox, SmallHeader } from './Shared';

export function AccessSection({
  definedAccessFields,
  awsIcRoleConditions,
  standardRoleConditions,
}: {
  definedAccessFields: typeof definableResourceAccessFields;
  awsIcRoleConditions: AwsIcRoleConditions;
  standardRoleConditions: StandardRoleConditions;
}) {
  if (definedAccessFields.length === 0) {
    return null;
  }

  const accessSection = definedAccessFields.map(field => {
    switch (field) {
      case 'app_labels': {
        const {
          app_labels,
          aws_role_arns,
          azure_identities,
          gcp_service_accounts,
          mcp,
        } = standardRoleConditions;
        return (
          <OutlineBox key={field}>
            <AccessDefinitionHeader>Application Access</AccessDefinitionHeader>

            <AccessContainer>
              {renderLabels(app_labels)}

              {appIdentityFieldNames.map(field => {
                switch (field) {
                  case 'aws_role_arns':
                    if (!aws_role_arns?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>AWS Role ARNs</SmallHeader>
                        <Pills texts={aws_role_arns} />
                      </Box>
                    );

                  case 'azure_identities':
                    if (!azure_identities?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Azure Identities</SmallHeader>
                        <Pills texts={azure_identities} />
                      </Box>
                    );

                  case 'gcp_service_accounts':
                    if (!gcp_service_accounts?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>GCP Service Accounts</SmallHeader>
                        <Pills texts={gcp_service_accounts} />
                      </Box>
                    );

                  case 'mcp':
                    if (!mcp?.tools.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>MCP Tools</SmallHeader>
                        <Pills texts={mcp?.tools} />
                      </Box>
                    );

                  default:
                    field satisfies never;
                }
              })}
            </AccessContainer>
          </OutlineBox>
        );
      }

      case 'awsIc': {
        const { account } = awsIcRoleConditions;
        if (!account || account.size === 0) {
          return null;
        }
        return (
          <OutlineBox key={field}>
            <AccessDefinitionHeader>
              AWS Identity Center Access
            </AccessDefinitionHeader>

            <AwsIcGrid>
              <Text bold fontSize={1}>
                AWS Account ID
              </Text>
              <Text bold fontSize={1}>
                Permission Sets
              </Text>
              {Array.from(account.entries()).map(([accountId, arns]) => (
                <Fragment key={accountId}>
                  <Text>{accountId}</Text>
                  <Pills texts={Array.from(arns)} />
                </Fragment>
              ))}
            </AwsIcGrid>
          </OutlineBox>
        );
      }

      case 'db_labels': {
        const { db_labels, db_names, db_users } = standardRoleConditions;
        return (
          <OutlineBox key={field}>
            <AccessDefinitionHeader>Database Access</AccessDefinitionHeader>

            <AccessContainer>
              {renderLabels(db_labels)}

              {dbIdentities.map(field => {
                switch (field) {
                  case 'db_names':
                    if (!db_names?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Names</SmallHeader>
                        <Pills texts={db_names} />
                      </Box>
                    );
                  case 'db_users':
                    if (!db_users?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Users</SmallHeader>
                        <Pills texts={db_users} />
                      </Box>
                    );
                  default:
                    field satisfies never;
                }
              })}
            </AccessContainer>
          </OutlineBox>
        );
      }

      case 'github_permissions': {
        const { github_permissions } = standardRoleConditions;
        return (
          <OutlineBox key={field}>
            <AccessDefinitionHeader>GitHub Access</AccessDefinitionHeader>

            <SmallHeader>Organizations</SmallHeader>
            {gitHubIdentities.map(field => {
              switch (field) {
                case 'github_permissions':
                  return (
                    <Pills
                      key={field}
                      texts={github_permissions.flatMap(gh => gh.orgs ?? [])}
                    />
                  );
                default:
                  field satisfies never;
              }
            })}
          </OutlineBox>
        );
      }

      case 'kubernetes_labels': {
        const {
          kubernetes_groups,
          kubernetes_labels,
          kubernetes_resources,
          kubernetes_users,
        } = standardRoleConditions;

        return (
          <OutlineBox key={field}>
            <AccessDefinitionHeader>Kubernetes Access</AccessDefinitionHeader>
            <AccessContainer>
              {renderLabels(kubernetes_labels)}

              {kubeIdentities.map(field => {
                switch (field) {
                  case 'kubernetes_groups':
                    if (!kubernetes_groups?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Groups</SmallHeader>
                        <Pills texts={kubernetes_groups} />
                      </Box>
                    );

                  case 'kubernetes_users':
                    if (!kubernetes_users?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Users</SmallHeader>
                        <Pills texts={kubernetes_users} />
                      </Box>
                    );

                  case 'kubernetes_resources':
                    if (!kubernetes_resources?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Resources</SmallHeader>
                        <Stack gap={2}>
                          {kubernetes_resources.map((kr, i) => (
                            <KubeResourceGrid key={i}>
                              <Text>
                                <BoldSpan>kind:</BoldSpan> {kr.kind}
                              </Text>
                              <Text>
                                <BoldSpan>namespace:</BoldSpan> {kr.namespace}
                              </Text>
                              <Text>
                                <BoldSpan>name:</BoldSpan> {kr.name}
                              </Text>
                              <Text>
                                <BoldSpan>api_group:</BoldSpan> {kr.api_group}
                              </Text>
                              <KubeVerbs>
                                <BoldSpan>verbs:</BoldSpan>
                                <Pills texts={kr.verbs} />
                              </KubeVerbs>
                            </KubeResourceGrid>
                          ))}
                        </Stack>
                      </Box>
                    );
                  default:
                    field satisfies never;
                }
              })}
            </AccessContainer>
          </OutlineBox>
        );
      }

      case 'node_labels': {
        const { node_labels, logins } = standardRoleConditions;
        return (
          <OutlineBox key={field}>
            <AccessDefinitionHeader>Server Access</AccessDefinitionHeader>

            <AccessContainer>
              {renderLabels(node_labels)}

              {serverIdentities.map(field => {
                switch (field) {
                  case 'logins':
                    if (!logins?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Logins</SmallHeader>
                        <Pills texts={logins} />
                      </Box>
                    );
                  default:
                    field satisfies never;
                }
              })}
            </AccessContainer>
          </OutlineBox>
        );
      }

      case 'windows_desktop_labels': {
        const { windows_desktop_labels, windows_desktop_logins } =
          standardRoleConditions;

        return (
          <OutlineBox key={field}>
            <AccessDefinitionHeader>
              Windows Desktop Access
            </AccessDefinitionHeader>

            <AccessContainer>
              {renderLabels(windows_desktop_labels)}

              {desktopIdentities.map(field => {
                switch (field) {
                  case 'windows_desktop_logins':
                    if (!windows_desktop_logins?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Logins</SmallHeader>
                        <Pills texts={windows_desktop_logins} />
                      </Box>
                    );
                  default:
                    field satisfies never;
                }
              })}
            </AccessContainer>
          </OutlineBox>
        );
      }

      case 'linux_desktop_labels': {
        const { linux_desktop_labels, linux_desktop_logins } =
          standardRoleConditions;

        return (
          <OutlineBox key={field}>
            <AccessDefinitionHeader>
              Linux Desktop Access
            </AccessDefinitionHeader>

            <AccessContainer>
              {renderLabels(linux_desktop_labels)}

              {linuxDesktopIdentities.map(field => {
                switch (field) {
                  case 'linux_desktop_logins':
                    if (!linux_desktop_logins?.length) {
                      return null;
                    }
                    return (
                      <Box key={field}>
                        <SmallHeader>Logins</SmallHeader>
                        <Pills texts={linux_desktop_logins} />
                      </Box>
                    );
                  default:
                    field satisfies never;
                }
              })}
            </AccessContainer>
          </OutlineBox>
        );
      }

      default:
        field satisfies never;
    }
  });

  return (
    <Stack>
      <H2>Access Definition</H2>

      <Flex flexDirection={'column'} gap={2}>
        {accessSection}
      </Flex>
    </Stack>
  );
}

const BoldSpan = styled.span`
  font-weight: ${p => p.theme.fontWeights.bold};
  color: ${p => p.theme.colors.text.slightlyMuted};
  font-size: ${p => p.theme.fontSizes[1]}px;
`;

const AwsIcGrid = styled(Box)`
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  align-items: center;
`;

const KubeResourceGrid = styled(Box)`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: ${p => p.theme.space[1]}px ${p => p.theme.space[3]}px;
  padding: ${p => p.theme.space[2]}px;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border-radius: 4px;
  width: 100%;
`;

const KubeVerbs = styled(Flex)`
  grid-column: span 2;
  align-items: center;
  gap: ${p => p.theme.space[1]}px ${p => p.theme.space[2]}px;
`;

const AccessContainer = styled(Flex)`
  flex-direction: column;
  gap: ${p => p.theme.space[3]}px;
`;

const AccessDefinitionHeader = styled(H3)`
  margin-bottom: ${p => p.theme.space[2]}px;
`;

function makeLabelText(labelKey: string, labelVals: string[]) {
  return `${labelKey}: ${labelVals.join(' OR ')}`;
}

function getLabelTexts(labels: Labels) {
  return Object.keys(labels).map(labelKey => {
    const labelVal = labels[labelKey];
    if (Array.isArray(labelVal)) {
      return makeLabelText(labelKey, labelVal);
    } else {
      return makeLabelText(labelKey, [labelVal]);
    }
  });
}

function renderLabels(labels: Labels) {
  return (
    <Box>
      <SmallHeader>Labels</SmallHeader>
      <Flex gap={2}>
        <Pills texts={getLabelTexts(labels)} />
      </Flex>
    </Box>
  );
}
