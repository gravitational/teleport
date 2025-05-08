/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { forwardRef, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import type { TransitionStatus } from 'react-transition-group';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonText,
  Flex,
  H2,
  Image,
  Input,
  Subtitle2,
  Text,
} from 'design';
import Table, { Cell } from 'design/DataTable';
import { ArrowBack } from 'design/Icon';
import useAttempt from 'shared/hooks/useAttemptNext';
import { mergeRefs } from 'shared/libs/mergeRefs';
import { pluralize } from 'shared/utils/text';

import shieldCheck from 'teleport/assets/shield-check.png';
import cfg from 'teleport/config';
import { TrashButton } from 'teleport/LocksV2/common';
import { lockService } from 'teleport/services/locks';

import {
  LockResource,
  LockResourceKind,
  LockResourceMap,
  ToggleSelectResourceFn,
} from '../common';

type Props = {
  onClose(): void;
  selectedResources: LockResourceMap;
  toggleResource: ToggleSelectResourceFn;
  selectedResourceKind: LockResourceKind;
  batchDeleteResources(resources: LockResource[]): void;
  reset(): void;
  transitionState: TransitionStatus;
};

export const LockCheckout = forwardRef<HTMLDivElement, Props>(
  (
    {
      selectedResources,
      onClose,
      reset,
      toggleResource,
      transitionState,
      batchDeleteResources,
    },
    ref
  ) => {
    const sliderRef = useRef<HTMLDivElement>(null);

    const { attempt, setAttempt } = useAttempt('');

    const [message, setMessage] = useState('');
    const [ttl, setTtl] = useState('');
    const [createdLocks, setCreatedLocks] = useState([]);

    // Format data suitable for table listing.
    const locks: LockResource[] = [];
    const resourceKeys = Object.keys(selectedResources) as LockResourceKind[];
    resourceKeys.forEach(kind => {
      Object.keys(selectedResources[kind]).forEach(targetValue =>
        locks.push({
          kind,
          targetValue,
          friendlyName: selectedResources[kind][targetValue],
        })
      );
    });

    function createLocks() {
      setAttempt({ status: 'processing' });

      // Each lock is a separate fetch request and
      // not every request is gauranteed to be successful (eg. network blip).
      // We will allow every request to run and then check for failures.
      // Any failures will be reported to user so they can attempt to create
      // locks again just for the failed ones.
      const promises = locks.map(lock => {
        return lockService.createLock({
          targets: { [lock.kind]: lock.targetValue },
          message,
          ttl,
        });
      });

      return Promise.allSettled(promises)
        .then(results => {
          const rejectedReasons: string[] = [];
          const resourcesToRemove: LockResource[] = [];

          results.forEach((res, index) => {
            if (res.status === 'fulfilled') {
              createdLocks.push(res.value);
              resourcesToRemove.push(locks[index]);
            } else {
              rejectedReasons.push(res.reason);
            }
          });

          setCreatedLocks(createdLocks);
          if (rejectedReasons.length > 0) {
            // Batch remove the ones we successfully created so users
            // don't see it from the list anymore and we don't duplicate
            // lock requests.
            batchDeleteResources(resourcesToRemove);
            setAttempt({
              status: 'failed',
              // Only show the first error, most likely the rest of the errors will be the same.
              statusText: `some resources failed to lock (see table below), try again: ${rejectedReasons[0]}`,
            });
          } else {
            reset();
            setAttempt({ status: 'success' });
          }
        })
        .catch((err: Error) => {
          // Should never reach here, but just in case.
          setAttempt({
            status: 'failed',
            statusText: err?.message || 'failed to create any locks',
          });
        });
    }

    function deleteLock(resource: LockResource) {
      setAttempt({ status: '' });
      toggleResource(resource);
    }

    const submitBtnDisabled =
      locks.length === 0 || attempt.status === 'processing';

    // Listeners are attached to enable overflow on the parent container after
    // transitioning ends (entered) or starts (exits). Enables vertical scrolling
    // when content gets too big.
    //
    // Overflow is initially hidden to prevent
    // brief flashing of horizontal scroll bar resulting from positioning
    // the container off screen to the right for the slide affect.
    useEffect(() => {
      function applyOverflowAutoStyle(e: TransitionEvent) {
        if (e.propertyName === 'right') {
          sliderRef.current.style.overflow = `auto`;
          // There will only ever be one 'end right' transition invoked event, so we remove it
          // afterwards, and listen for the 'start right' transition which is only invoked
          // when user exits this component.
          window.removeEventListener('transitionend', applyOverflowAutoStyle);
          window.addEventListener('transitionstart', applyOverflowHiddenStyle);
        }
      }

      function applyOverflowHiddenStyle(e: TransitionEvent) {
        if (e.propertyName === 'right') {
          sliderRef.current.style.overflow = `hidden`;
        }
      }

      window.addEventListener('transitionend', applyOverflowAutoStyle);

      return () => {
        window.removeEventListener('transitionend', applyOverflowAutoStyle);
        window.removeEventListener('transitionstart', applyOverflowHiddenStyle);
      };
    }, []);

    return (
      <Box
        ref={mergeRefs([sliderRef, ref])}
        css={`
          position: absolute;
          width: 100vw;
          height: 100vh;
          top: 0;
          left: 0;
          overflow: hidden;
        `}
      >
        <Dimmer className={transitionState} />
        <SidePanel className={transitionState}>
          {attempt.status === 'success' ? (
            <Box>
              <Box mt={2} mb={7} textAlign="center">
                <H2 mb={1}>Resources Locked Successfully</H2>
                <Subtitle2 color="text.secondary">
                  You've successfully locked {createdLocks.length}{' '}
                  {pluralize(createdLocks.length, 'resource')}
                </Subtitle2>
              </Box>
              <Flex justifyContent="center" mb={3}>
                <Image src={shieldCheck} width="250px" height="179px" />
              </Flex>
            </Box>
          ) : (
            <Flex mb={3} alignItems="center">
              <ArrowBack
                size="large"
                mr={3}
                onClick={onClose}
                style={{ cursor: 'pointer' }}
              />
              <Box>
                <H2>
                  {locks.length} {pluralize(locks.length, 'Target')} Added
                </H2>
              </Box>
            </Flex>
          )}
          {attempt.status === 'success' ? (
            <SuccessActionComponent
              onClose={onClose}
              reset={reset}
              locks={createdLocks}
            />
          ) : (
            <>
              {attempt.status === 'failed' && (
                <Alert kind="danger" children={attempt.statusText} />
              )}
              <StyledTable
                data={locks}
                columns={[
                  {
                    key: 'kind',
                    headerText: 'Resource Kind',
                  },
                  {
                    key: 'friendlyName',
                    headerText: 'Resource Name',
                  },
                  {
                    altKey: 'delete-btn',
                    render: lock => (
                      <Cell align="right">
                        <TrashButton
                          size="small"
                          onClick={() => deleteLock(lock)}
                          disabled={attempt.status === 'processing'}
                        />
                      </Cell>
                    ),
                  },
                ]}
                emptyText="No lock targets are selected"
              />
              <Box mt={3}>
                <Text mr={2}>Message</Text>
                <Input
                  placeholder={`Going down for maintenance`}
                  value={message}
                  onChange={e => setMessage(e.currentTarget.value)}
                />
              </Box>
              <Box mt={3}>
                <Text mr={2}>TTL</Text>
                <Input
                  placeholder={`2h45m, 5h, empty=never`}
                  value={ttl}
                  onChange={e => setTtl(e.currentTarget.value)}
                />
              </Box>

              <Box
                py={4}
                css={`
                  position: sticky;
                  bottom: 0;
                  background: ${({ theme }) => theme.colors.levels.sunken};
                `}
              >
                <ButtonPrimary
                  width="100%"
                  size="large"
                  onClick={createLocks}
                  disabled={submitBtnDisabled}
                >
                  Create Locks
                </ButtonPrimary>
              </Box>
            </>
          )}
        </SidePanel>
      </Box>
    );
  }
);

function SuccessActionComponent({ reset, onClose, locks }) {
  return (
    <Box textAlign="center">
      <ButtonPrimary
        as={Link}
        mt={5}
        mb={3}
        width="100%"
        size="large"
        to={
          locks.length
            ? {
                pathname: cfg.getLocksRoute(),
                state: { createdLocks: locks },
              }
            : cfg.getLocksRoute()
        }
      >
        Back to Locks
      </ButtonPrimary>
      <ButtonText
        onClick={() => {
          reset();
          onClose();
        }}
      >
        Make Another Request
      </ButtonText>
    </Box>
  );
}

const SidePanel = styled(Box)`
  position: absolute;
  z-index: 11;
  top: 0px;
  right: 0px;
  background: ${({ theme }) => theme.colors.levels.sunken};
  min-height: 100%;
  width: 500px;
  padding: 20px;

  &.entering {
    right: -500px;
  }
  &.entered {
    right: 0px;
    transition: right 300ms ease-out;
  }
  &.exiting {
    right: -500px;
    transition: right 300ms ease-out;
  }
  &.exited {
    right: -500px;
  }
`;

const Dimmer = styled(Box)`
  background: #000;
  opacity: 0.5;
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 10;
`;

const StyledTable = styled(Table)`
  & > tbody > tr > td {
    vertical-align: middle;
  }
  & > thead > tr > th {
    background: ${props => props.theme.colors.spotBackground[1]};
  }
  border-radius: 8px;
  box-shadow: ${props => props.theme.boxShadow[0]};
  overflow: hidden;
` as typeof Table;
