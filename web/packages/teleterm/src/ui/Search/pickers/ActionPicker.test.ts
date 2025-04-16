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

import { makeSuccessAttempt } from 'shared/hooks/useAsync';

import {
  makeRetryableError,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { ResourceSearchError } from 'teleterm/ui/services/resources';

import { getActionPickerStatus } from './ActionPicker';

describe('getActionPickerStatus', () => {
  describe('some-input search mode', () => {
    it('partitions resource search errors into clusters with expired certs and non-retryable errors', () => {
      const retryableError = new ResourceSearchError(
        '/clusters/foo',
        makeRetryableError()
      );

      const nonRetryableError = new ResourceSearchError(
        '/clusters/bar',
        new Error('whoops')
      );

      const status = getActionPickerStatus({
        inputValue: 'foo',
        filters: [],
        filterActions: [],
        allClusters: [],
        resourceSearchAttempt: makeSuccessAttempt({
          errors: [retryableError, nonRetryableError],
          results: [],
          search: 'foo',
        }),
      });

      expect(status.inputState).toBe('some-input');

      const { clustersWithExpiredCerts, nonRetryableResourceSearchErrors } =
        status.inputState === 'some-input' && status;

      expect([...clustersWithExpiredCerts]).toEqual([
        retryableError.clusterUri,
      ]);
      expect(nonRetryableResourceSearchErrors).toEqual([nonRetryableError]);
    });

    it('merges non-connected clusters with clusters that returned retryable errors', () => {
      const offlineCluster = makeRootCluster({ connected: false });
      const retryableError = new ResourceSearchError(
        '/clusters/foo',
        makeRetryableError()
      );

      const status = getActionPickerStatus({
        inputValue: 'foo',
        filters: [],
        filterActions: [],
        allClusters: [offlineCluster],
        resourceSearchAttempt: makeSuccessAttempt({
          errors: [retryableError],
          results: [],
          search: 'foo',
        }),
      });

      expect(status.inputState).toBe('some-input');
      const { clustersWithExpiredCerts } =
        status.inputState === 'some-input' && status;

      expect(clustersWithExpiredCerts.size).toBe(2);
      expect(clustersWithExpiredCerts).toContain(offlineCluster.uri);
      expect(clustersWithExpiredCerts).toContain(retryableError.clusterUri);
    });

    it('includes a cluster with expired cert only once even if multiple requests fail with retryable errors', () => {
      const retryableErrors = [
        new ResourceSearchError('/clusters/foo', makeRetryableError()),
        new ResourceSearchError('/clusters/foo', makeRetryableError()),
      ];
      const status = getActionPickerStatus({
        inputValue: 'foo',
        filters: [],
        filterActions: [],
        allClusters: [],
        resourceSearchAttempt: makeSuccessAttempt({
          errors: retryableErrors,
          results: [],
          search: 'foo',
        }),
      });

      expect(status.inputState).toBe('some-input');
      const { clustersWithExpiredCerts } =
        status.inputState === 'some-input' && status;
      expect([...clustersWithExpiredCerts]).toEqual(['/clusters/foo']);
    });

    describe('when there are no results', () => {
      it('lists only the filtered offline cluster if a cluster filter is selected and the filtered cluster is offline', () => {
        const filteredCluster = makeRootCluster({
          connected: false,
          uri: '/clusters/filtered-cluster',
        });
        const otherOfflineCluster = makeRootCluster({
          connected: false,
          uri: '/clusters/other-offline-cluster',
        });
        const status = getActionPickerStatus({
          inputValue: 'foo',
          filters: [{ filter: 'cluster', clusterUri: filteredCluster.uri }],
          filterActions: [],
          allClusters: [filteredCluster, otherOfflineCluster],
          resourceSearchAttempt: makeSuccessAttempt({
            errors: [],
            results: [],
            search: 'foo',
          }),
        });

        expect(status.inputState).toBe('some-input');
        const { clustersWithExpiredCerts } =
          status.inputState === 'some-input' && status;
        expect([...clustersWithExpiredCerts]).toEqual([filteredCluster.uri]);
      });

      it('does not list offline clusters if a cluster filter is selected and that cluster is online and there are no results', () => {
        const filteredCluster = makeRootCluster({
          connected: true,
          uri: '/clusters/filtered-cluster',
        });
        const otherOfflineCluster = makeRootCluster({
          connected: false,
          uri: '/clusters/other-offline-cluster',
        });
        const status = getActionPickerStatus({
          inputValue: 'foo',
          filters: [{ filter: 'cluster', clusterUri: filteredCluster.uri }],
          filterActions: [],
          allClusters: [filteredCluster, otherOfflineCluster],
          resourceSearchAttempt: makeSuccessAttempt({
            errors: [],
            results: [],
            search: 'foo',
          }),
        });

        expect(status.inputState).toBe('some-input');
        const { clustersWithExpiredCerts } =
          status.inputState === 'some-input' && status;
        expect([...clustersWithExpiredCerts]).toHaveLength(0);
      });
    });
  });

  describe('no-input search mode', () => {
    it('returns non-retryable errors when fetching a preview after selecting a filter fails', () => {
      const nonRetryableError = new ResourceSearchError(
        '/clusters/bar',
        new Error('non-retryable error')
      );
      const resourceSearchErrors = [
        new ResourceSearchError('/clusters/foo', makeRetryableError()),
        nonRetryableError,
      ];
      const status = getActionPickerStatus({
        inputValue: '',
        filters: [{ filter: 'resource-type', resourceType: 'node' }],
        filterActions: [],
        allClusters: [],
        resourceSearchAttempt: makeSuccessAttempt({
          errors: resourceSearchErrors,
          results: [],
          search: '',
        }),
      });

      expect(status.inputState).toBe('no-input');

      const { searchMode } = status.inputState === 'no-input' && status;
      expect(searchMode.kind).toBe('preview');

      const { nonRetryableResourceSearchErrors } =
        searchMode.kind === 'preview' && searchMode;
      expect(nonRetryableResourceSearchErrors).toEqual([nonRetryableError]);
    });
  });
});
