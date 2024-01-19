import React, { useEffect } from 'react';
import Box from 'design/Box';
import Flex from 'design/Flex';
import Select, { Option } from 'shared/components/Select';

import Indicator from 'design/Indicator';

import Alert from 'design/Alert';

import Text from 'design/Text';

import Link from 'design/Link';

import { Attempt } from 'shared/hooks/useAttemptNext';

import cfg from 'e-teleport/config';

import { GETTING_STARTED_LINK } from '../Downloads';

import { ReleasesList } from './ReleasesList';
import { OsToggle } from './OsToggle';

import type { Asset, Kind, OS, Release } from 'e-teleport/services/downloads';

type TeleportReleasesProps = {
  canDownloadReleaseAssets: boolean;
  releases: Release[];
  attempt: Attempt;
  availableVersions: string[];
  selectedVersion: string;
  setSelectedVersion: (version: string) => void;
  selectedOS: OS;
  setSelectedOS: (os: OS) => void;
  selectedKind: Kind;
  setSelectedKind: (kind: Kind) => void;
};

type KindOption = Option<Kind>;

const kindOptions: KindOption[] = [
  { value: 'Teleport', label: 'Teleport' },
  {
    value: 'Teleport Connect',
    label: 'Teleport Connect',
  },
  { value: 'tsh client', label: 'tsh client' },
];

export const TeleportReleases = ({
  canDownloadReleaseAssets,
  releases,
  attempt,
  availableVersions,
  selectedVersion,
  setSelectedVersion,
  selectedOS,
  setSelectedOS,
  selectedKind,
  setSelectedKind,
}: TeleportReleasesProps) => {
  const versionOptions = availableVersions.map(makeOption);

  function hasAssets(kind: Kind): boolean {
    return applyFilter(releases, selectedOS, selectedVersion, kind).length > 0;
  }

  const availableOptions = kindOptions.filter((option: KindOption) =>
    hasAssets(option.value)
  );

  const displayAssets = applyFilter(
    releases,
    selectedOS,
    selectedVersion,
    selectedKind
  );

  // when the select OS changes, update the selected option
  // if the current (option, os) pair doesn't have any binaries.
  useEffect(() => {
    if (displayAssets.length == 0 && availableOptions.length > 0) {
      setSelectedKind(availableOptions[0].value);
    }
  }, [selectedOS]);

  return (
    <>
      <Box>
        <Text bold typography="h5">
          Download Teleport
        </Text>
        {!cfg.oss.isCloud && (
          <Text my={3}>
            You will also need the binaries below for{' '}
            <Link href={GETTING_STARTED_LINK} color="text.main" about="_blank">
              Getting Started with Teleport Enterprise
            </Link>
            {':'}
          </Text>
        )}
        {canDownloadReleaseAssets && (
          <>
            {attempt.status === 'processing' && (
              <Box textAlign="center" m={10}>
                <Indicator />
              </Box>
            )}
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}

            {attempt.status === 'success' && (
              <>
                <Flex alignItems="center" mt={2}>
                  <Box width="210px">
                    <Select
                      isSearchable={false}
                      options={availableOptions}
                      onChange={option =>
                        setSelectedKind(
                          Array.isArray(option) ? null : option.value
                        )
                      }
                      value={{ label: selectedKind, value: selectedKind }}
                    />
                  </Box>
                  <Box width="210px" ml={3}>
                    <Select
                      isSearchable={false}
                      options={versionOptions}
                      onChange={(option: Option) =>
                        setSelectedVersion(option.value)
                      }
                      value={{ label: selectedVersion, value: selectedVersion }}
                    />
                  </Box>
                  <Flex ml="auto">
                    <OsToggle onClick={setSelectedOS} selectedOS={selectedOS} />
                  </Flex>
                </Flex>
                <Box mt={3}>
                  <ReleasesList assets={displayAssets} />
                </Box>
              </>
            )}
          </>
        )}
      </Box>

      {!canDownloadReleaseAssets && (
        <Box mt={3}>
          <ReleasesList
            assets={[]}
            emptyText={
              'User has no permission to download Teleport Enterprise binaries'
            }
          />
        </Box>
      )}
    </>
  );
};

const makeOption = val => ({ label: val, value: val });

const applyFilter = (
  releases: Release[],
  os: OS,
  version: string,
  kind: Kind
): Asset[] => {
  const versionReleases = releases.filter(
    release => release.version === version
  );

  const assets = versionReleases.flatMap(release => release.assets);

  return assets.filter(asset => asset.os === os && asset.kind === kind);
};
