import { makeDescription, makeKind, makeOS } from './make';
import type { Kind, OS } from './types';

type MakeDescriptionInput = {
  description: string;
  kind: Kind;
  os: OS;
  url: string;
};

const makeDescriptionCases: [MakeDescriptionInput, string][] = [
  [
    {
      description: 'Linux ARMv7 (32-bit)',
      kind: 'Teleport',
      os: 'Linux',
      url: '',
    },
    'Linux ARMv7 (32-bit)',
  ],
  [
    {
      description: 'Linux 64-bit (FedRAMP/FIPS)',
      kind: 'Teleport',
      os: 'Linux',
      url: '',
    },
    'Linux 64-bit (FedRAMP/FIPS)',
  ],
  [
    {
      description: 'Windows 64-bit (tsh client only)',
      kind: 'tsh client',
      os: 'Windows',
      url: '',
    },
    'Windows 64-bit (tsh client only)',
  ],
  [
    {
      description: 'MacOS Intel',
      kind: 'Teleport',
      os: 'macOS',
      url: '',
    },
    'MacOS Intel',
  ],
  [
    {
      description: 'Teleport Connect',
      kind: 'Teleport Connect',
      os: 'Linux',
      url: ' https://cdn.cloud.gravitational.io/teleport-connect-10.2.2.x86_64.rpm',
    },
    'Teleport Connect 64-bit RPM',
  ],
  [
    {
      description: 'Teleport Connect',
      kind: 'Teleport Connect',
      os: 'Linux',
      url: ' https://cdn.cloud.gravitational.io/teleport-connect_10.3.1_amd64.deb',
    },
    'Teleport Connect 64-bit DEB',
  ],
];

test.each(makeDescriptionCases)(
  'makeDescription(%s) should be %s',
  (input, expected) => {
    const { description, kind, os, url } = input;
    expect(makeDescription(description, kind, os, url)).toBe(expected);
  }
);

test('makeOS', () => {
  expect(makeOS('linux')).toBe('Linux');
  expect(makeOS('windows')).toBe('Windows');
  expect(makeOS('macos')).toBe('macOS');
  expect(makeOS('darwin')).toBe('macOS');
  expect(makeOS('unknown')).toBe('Linux');
});

test('makeKind', () => {
  expect(makeKind('', 'Windows')).toBe('Teleport');
  expect(makeKind('tsh-client', 'Linux')).toBe('tsh client');
  expect(makeKind('Teleport Connect', 'Windows')).toBe('Teleport Connect');
  expect(makeKind('teleport-connect', 'Linux')).toBe('Teleport Connect');
  expect(makeKind('anything', 'Windows')).not.toBe('Teleport');
  expect(makeKind('teleport-connect', 'Windows')).toBe('Teleport Connect');
});
