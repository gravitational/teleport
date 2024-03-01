/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useTheme } from 'styled-components';
import { Flex, Alert, Text, ButtonPrimary, ButtonSecondary } from 'design';
import * as icons from 'design/SVGIcon';
import { StyledTable, StyledTableWrapper } from 'design/DataTable/StyledTable';

import Document from 'teleterm/ui/Document';

import { useVnetContext } from './vnetContext';

import type * as docTypes from 'teleterm/ui/services/workspacesService';

export function DocumentVnet(props: {
  visible: boolean;
  doc: docTypes.DocumentVnet;
}) {
  const { doc } = props;
  const { status, start, startAttempt, stop, stopAttempt } = useVnetContext();
  const theme = useTheme();

  return (
    <Document visible={props.visible}>
      <Flex
        flexDirection="column"
        alignItems="flex-start"
        gap={2}
        p={4}
        maxWidth="680px"
        mx="auto"
        width="100%"
        height="fit-content"
      >
        <Flex width="100%" justifyContent="space-between" alignItems="baseline">
          <Text typography="h3">VNet</Text>
          {status === 'running' && (
            <ButtonSecondary
              onClick={() => stop(doc.rootClusterUri)}
              title="Stop VNet for teleport-local.dev"
              disabled={stopAttempt.status === 'processing'}
            >
              Stop VNet
            </ButtonSecondary>
          )}
          {status === 'stopped' && (
            <ButtonPrimary
              onClick={() => start(doc.rootClusterUri)}
              disabled={startAttempt.status === 'processing'}
            >
              Start VNet
            </ButtonPrimary>
          )}
        </Flex>

        {startAttempt.status === 'error' && (
          <Alert>{startAttempt.statusText}</Alert>
        )}
        {stopAttempt.status === 'error' && (
          <Alert>{stopAttempt.statusText}</Alert>
        )}

        {status === 'stopped' && (
          <Text>
            With VNet, connecting to an app in the cluster is as simple as
            appending <code>.internal</code> to the address of the app.
            Underneath, VNet establishes a secure tunnel to the app itself,
            meaning you don't have to pass around certificates to authenticate
            the connection — VNet does that for you.
          </Text>
        )}

        {status === 'running' && (
          <>
            <Text>
              Proxying connections made to .teleport-local.dev.internal,
              .company.private
            </Text>

            <Flex width="100%" flexDirection="column" gap={1}>
              <Text typography="h4">Recent connections</Text>
              <StyledTableWrapper borderRadius={1}>
                <StyledTable>
                  <tbody>
                    <tr>
                      <td>
                        <Flex gap={2} alignItems="center">
                          <Flex
                            width="12px"
                            height="12px"
                            bg="success.main"
                            borderRadius="50%"
                            justifyContent="center"
                            alignItems="center"
                            css={`
                              flex-shrink: 0;
                            `}
                          ></Flex>{' '}
                          httpbin.company.private
                        </Flex>
                      </td>
                      <td></td>
                    </tr>

                    <tr>
                      <td>
                        <Flex gap={2} alignItems="center">
                          <Flex
                            width="12px"
                            height="12px"
                            bg="success.main"
                            borderRadius="50%"
                            justifyContent="center"
                            alignItems="center"
                            css={`
                              flex-shrink: 0;
                            `}
                          ></Flex>{' '}
                          tcp-postgres.teleport-local.dev.internal
                        </Flex>
                      </td>
                      <td></td>
                    </tr>
                    <tr>
                      <td>
                        <Flex gap={2} alignItems="center">
                          <Flex
                            width="12px"
                            height="12px"
                            bg="error.main"
                            borderRadius="50%"
                            justifyContent="center"
                            alignItems="center"
                          >
                            {/* TODO(ravicious): Make SVGIcon support passing color strings as fill. */}
                            <icons.ErrorIcon
                              size={12}
                              fill={theme.colors.levels.surface}
                            />
                          </Flex>{' '}
                          grafana.teleport-local.dev.internal
                        </Flex>
                      </td>

                      {/*
                      TODO(ravicious): Solve this without using an arbitrary max-width if possible,
                      perhaps switch to a flexbox instead of using a table?
                    */}
                      <td
                        css={`
                          max-width: 320px;
                          overflow: hidden;
                          text-overflow: ellipsis;
                          white-space: nowrap;
                        `}
                      >
                        DNS query for "grafana.teleport-local.dev.internal" in
                        custom DNS zone failed: no matching Teleport app and
                        upstream nameserver did not respond
                      </td>
                    </tr>
                    <tr>
                      <td>
                        <Flex gap={2} alignItems="center">
                          <Flex
                            width="12px"
                            height="12px"
                            bg="unset"
                            borderRadius="50%"
                            justifyContent="center"
                            alignItems="center"
                            css={`
                              flex-shrink: 0;
                            `}
                          ></Flex>{' '}
                          dumper.teleport-local.dev.internal
                        </Flex>
                      </td>
                      <td></td>
                    </tr>
                  </tbody>
                </StyledTable>
              </StyledTableWrapper>
            </Flex>
          </>
        )}
      </Flex>
    </Document>
  );
}
