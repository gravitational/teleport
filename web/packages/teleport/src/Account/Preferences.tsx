/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

// Layout identifiers gotten from:
// https://learn.microsoft.com/en-us/globalization/windows-keyboard-layouts

import { useEffect } from 'react';
import { useTheme } from 'styled-components';

import { Box, Flex } from 'design';
import * as Icon from 'design/Icon';
import { SingleRowBox } from 'design/MultiRowBox';
import { Theme } from 'gen-proto-ts/teleport/userpreferences/v1/theme_pb';
import Select from 'shared/components/Select';
import { useAsync } from 'shared/hooks/useAsync';

import { useUser } from 'teleport/User/UserContext';

import { Header } from './Header';
import { useNotification } from './NotificationContext';
import DarkThemeIcon from './ThemeImages/dark_theme.svg';
import LightThemeIcon from './ThemeImages/light_theme.svg';
import SystemThemeIcon from './ThemeImages/system_theme.svg';

const layouts = {
  0: 'System',
  0x00000401: 'Arabic (101)',
  0x00000402: 'Bulgarian',
  0x00000404: 'Chinese (Traditional) - US Keyboard',
  0x00000405: 'Czech',
  0x00000406: 'Danish',
  0x00000407: 'German',
  0x00000408: 'Greek',
  0x00000409: 'US',
  0x0000040a: 'Spanish',
  0x0000040b: 'Finnish',
  0x0000040c: 'French',
  0x0000040d: 'Hebrew',
  0x0000040e: 'Hungarian',
  0x0000040f: 'Icelandic',
  0x00000410: 'Italian',
  0x00000411: 'Japanese',
  0x00000412: 'Korean',
  0x00000413: 'Dutch',
  0x00000414: 'Norwegian',
  0x00000415: 'Polish (Programmers)',
  0x00000416: 'Portuguese (Brazilian ABNT)',
  0x00000418: 'Romanian (Legacy)',
  0x00000419: 'Russian',
  0x0000041a: 'Croatian',
  0x0000041b: 'Slovak',
  0x0000041c: 'Albanian',
  0x0000041d: 'Swedish',
  0x0000041e: 'Thai Kedmanee',
  0x0000041f: 'Turkish Q',
  0x00000420: 'Urdu',
  0x00000422: 'Ukrainian',
  0x00000423: 'Belarusian',
  0x00000424: 'Slovenian',
  0x00000425: 'Estonian',
  0x00000426: 'Latvian',
  0x00000427: 'Lithuanian IBM',
  0x00000428: 'Tajik',
  0x00000429: 'Persian',
  0x0000042a: 'Vietnamese',
  0x0000042b: 'Armenian Eastern',
  0x0000042c: 'Azeri Latin',
  0x0000042e: 'Sorbian Standard',
  0x0000042f: 'Macedonian (FYROM)',
  0x00000437: 'Georgian',
  0x00000438: 'Faeroese',
  0x00000439: 'Devanagari-INSCRIPT',
  0x0000043a: 'Maltese 47-Key',
  0x0000043b: 'Norwegian with Sami',
  0x0000043f: 'Kazakh',
  0x00000440: 'Kyrgyz Cyrillic',
  0x00000442: 'Turkmen',
  0x00000444: 'Tatar',
  0x00000445: 'Bengali',
  0x00000446: 'Punjabi',
  0x00000447: 'Gujarati',
  0x00000448: 'Oriya',
  0x00000449: 'Tamil',
  0x0000044a: 'Telugu',
  0x0000044b: 'Kannada',
  0x0000044c: 'Malayalam',
  0x0000044d: 'ASSAMESE - INSCRIPT',
  0x0000044e: 'Marathi',
  0x00000450: 'Mongolian Cyrillic',
  0x00000451: "Tibetan (People's Republic of China)",
  0x00000452: 'United Kingdom Extended',
  0x00000453: 'Khmer',
  0x00000454: 'Lao',
  0x0000045a: 'Syriac',
  0x0000045b: 'Sinhala',
  0x00000461: 'Nepali',
  0x00000463: 'Pashto (Afghanistan)',
  0x00000465: 'Divehi Phonetic',
  0x0000046d: 'Bashkir',
  0x0000046e: 'Luxembourgish',
  0x0000046f: 'Greenlandic',
  0x00000480: 'Uighur',
  0x00000481: 'Maori',
  0x00000485: 'Yakut',
  0x00000804: 'Chinese (Simplified) - US Keyboard',
  0x00000807: 'Swiss German',
  0x00000809: 'United Kingdom',
  0x0000080a: 'Latin American',
  0x0000080c: 'Belgian French',
  0x00000813: 'Belgian (Period)',
  0x00000816: 'Portuguese',
  0x0000081a: 'Serbian (Latin)',
  0x0000082c: 'Azeri Cyrillic',
  0x0000083b: 'Swedish with Sami',
  0x00000843: 'Uzbek Cyrillic',
  0x00000850: 'Mongolian (Mongolian Script)',
  0x0000085d: 'Inuktitut - Latin',
  0x00000c0c: 'Canadian French (Legacy)',
  0x00000c1a: 'Serbian (Cyrillic)',
  0x00001009: 'Canadian French',
  0x0000100c: 'Swiss French',
  0x00001809: 'Irish',
  0x0000201a: 'Bosnian (Cyrillic)',
  0x00010401: 'Arabic (102)',
  0x00010402: 'Bulgarian (Latin)',
  0x00010405: 'Czech (QWERTY)',
  0x00010407: 'German (IBM)',
  0x00010408: 'Greek (220)',
  0x00010409: 'United States - Dvorak',
  0x0001040a: 'Spanish Variation',
  0x0001040e: 'Hungarian 101-key',
  0x00010410: 'Italian (142)',
  0x00010415: 'Polish (214)',
  0x00010416: 'Portuguese (Brazilian ABNT2)',
  0x00010418: 'Romanian (Standard)',
  0x00010419: 'Russian (Typewriter)',
  0x0001041b: 'Slovak (QWERTY)',
  0x0001041e: 'Thai Pattachote',
  0x0001041f: 'Turkish F',
  0x00010426: 'Latvian (QWERTY)',
  0x00010427: 'Lithuanian',
  0x0001042b: 'Armenian Western',
  0x0001042e: 'Sorbian Extended',
  0x0001042f: 'Macedonian (FYROM) - Standard',
  0x00010437: 'Georgian (QWERTY)',
  0x00010439: 'Hindi Traditional',
  0x0001043a: 'Maltese 48-key',
  0x0001043b: 'Sami Extended Norway',
  0x00010445: 'Bengali - INSCRIPT (Legacy)',
  0x0001045a: 'Syriac Phonetic',
  0x0001045b: 'Sinhala - wij 9',
  0x0001045d: 'Inuktitut - Naqittaut',
  0x00010465: 'Divehi Typewriter',
  0x0001080c: 'Belgian (Comma)',
  0x0001083b: 'Finnish with Sami',
  0x00011009: 'Canadian Multilingual Standard',
  0x00011809: 'Gaelic',
  0x00020401: 'Arabic (102) AZERTY',
  0x00020402: 'Bulgarian (phonetic layout)',
  0x00020405: 'Czech Programmers',
  0x00020408: 'Greek (319)',
  0x00020409: 'United States - International',
  0x00020418: 'Romanian (Programmers)',
  0x0002041e: 'Thai Kedmanee (non-ShiftLock)',
  0x00020422: 'Ukrainian (Enhanced)',
  0x00020427: 'Lithuanian New',
  0x00020437: 'Georgian (Ergonomic)',
  0x00020445: 'Bengali - INSCRIPT',
  0x0002083b: 'Sami Extended Finland-Sweden',
  0x00030402: 'Bulgarian (phonetic layout)',
  0x00030408: 'Greek (220) Latin',
  0x00030409: 'United States-Devorak for left hand',
  0x0003041e: 'Thai Pattachote (non-ShiftLock)',
  0x00040408: 'Greek (319) Latin',
  0x00040409: 'United States-Dvorak for right hand',
  0x00050409: 'Greek Latin',
  0x00060408: 'Greek Polytonic',
};

const layoutOptions = Object.keys(layouts).map(k => {
  return { label: layouts[k], value: parseInt(k) };
});

/**
 * For use by the account setting's side nav to determine which headings to show.
 * @returns Array of headings to show in the side nav.
 */
export function preferencesHeadings(): Array<{ name: string; id: string }> {
  const theme = useTheme();

  let headings = [];

  if (!theme.isCustomTheme) {
    headings.push({ name: 'Theme Preference', id: 'theme' });
  }

  headings.push({ name: 'Keyboard Layout', id: 'keyboard-layout' });

  return headings;
}

export interface PreferencesProps {
  setErrorMessage: (message: string | null) => void;
}

export function Preferences({ setErrorMessage }: PreferencesProps) {
  const { preferences, updatePreferences } = useUser();
  const theme = useTheme();
  const { addNotification } = useNotification();

  const layout = preferences.keyboardLayout;
  const layoutValue =
    layout !== undefined ? { label: layouts[layout], value: layout } : null;

  const [updatePreferenceAttempt, runUpdatePreference] =
    useAsync(updatePreferences);
  const currentTheme = preferences.theme;

  useEffect(() => {
    if (updatePreferenceAttempt.status === 'error') {
      setErrorMessage(
        `Failed to update the keyboard layout: ${updatePreferenceAttempt.statusText}`
      );
    }
  }, [updatePreferenceAttempt.status, setErrorMessage]);

  return (
    <Flex gap={4} flexDirection="column">
      {!theme.isCustomTheme && (
        <div id="theme">
          <SingleRowBox>
            <Header
              title="Theme"
              description="Choose if Teleport's appearance should be light or dark, or follow your computer's settings."
              icon={<Icon.Desktop />}
              actions={
                <Box>
                  <Flex gap={3}>
                    <label>
                      <Flex flexDirection="column" alignItems="center">
                        {themePreferenceImage(Theme.DARK, currentTheme)}
                        <Flex alignItems="center">
                          <input
                            type="radio"
                            name="theme"
                            value="dark"
                            checked={currentTheme === Theme.DARK}
                            onChange={() =>
                              runUpdatePreference({ theme: Theme.DARK })
                            }
                          />
                          <Box ml={1}>Dark</Box>
                        </Flex>
                      </Flex>
                    </label>
                    <label>
                      <Flex flexDirection="column" alignItems="center">
                        {themePreferenceImage(Theme.LIGHT, currentTheme)}
                        <Flex alignItems="center">
                          <input
                            type="radio"
                            name="theme"
                            value="light"
                            checked={currentTheme === Theme.LIGHT}
                            onChange={() =>
                              runUpdatePreference({ theme: Theme.LIGHT })
                            }
                          />
                          <Box ml={1}>Light</Box>
                        </Flex>
                      </Flex>
                    </label>
                    <label>
                      <Flex flexDirection="column" alignItems="center">
                        {themePreferenceImage(Theme.UNSPECIFIED, currentTheme)}
                        <Flex alignItems="center">
                          <input
                            type="radio"
                            name="theme"
                            value="system"
                            checked={currentTheme === Theme.UNSPECIFIED}
                            onChange={() =>
                              runUpdatePreference({ theme: Theme.UNSPECIFIED })
                            }
                          />
                          <Box ml={1}>System</Box>
                        </Flex>
                      </Flex>
                    </label>
                  </Flex>
                </Box>
              }
            ></Header>
          </SingleRowBox>
        </div>
      )}
      <div id="keyboard-layout">
        <SingleRowBox>
          <Header
            title="Windows Desktop Session Keyboard Layout"
            description={
              <>
                Choose keyboard layout for Windows Desktop sessions.
                <br />
                <br />
                Note: To maintain keyboard layout settings your agents need to
                be upgraded to Teleport 18.0.0 or later.
              </>
            }
            icon={<Icon.Keyboard />}
            actions={
              <Box minWidth="210px">
                <Select
                  onChange={selected => {
                    if (Array.isArray(selected)) {
                      selected = selected[0];
                    }
                    runUpdatePreference({
                      keyboardLayout: selected.value,
                    });
                    addNotification('success', {
                      title: 'Change saved',
                      isAutoRemovable: true,
                    });
                  }}
                  isDisabled={updatePreferenceAttempt.status === 'processing'}
                  value={layoutValue}
                  placeholder="Select Language/Country"
                  options={layoutOptions}
                  aria-label="keyboard layout select"
                />
              </Box>
            }
            actionPosition="top"
          />
        </SingleRowBox>
      </div>
    </Flex>
  );
}

function themePreferenceImage(
  themeOption: Theme,
  currentTheme: Theme
): React.ReactNode {
  const theme = useTheme();

  let altText: string;
  let imageSrc: string;
  switch (themeOption) {
    case Theme.DARK:
      altText = 'Dark Theme';
      imageSrc = DarkThemeIcon;
      break;
    case Theme.LIGHT:
      altText = 'Light Theme';
      imageSrc = LightThemeIcon;
      break;
    case Theme.UNSPECIFIED:
      altText = 'System Theme';
      imageSrc = SystemThemeIcon;
      break;
    default:
      altText = 'System Theme';
      imageSrc = SystemThemeIcon;
  }

  return (
    <Box
      p={2}
      border={
        currentTheme === themeOption
          ? `${theme.borders[2]} ${theme.colors.interactive.solid.primary.active}`
          : `${theme.borders[2]} ${theme.colors.interactive.tonal.neutral[2]}`
      }
      borderRadius={`${theme.radii[3]}px`}
      lineHeight={0}
    >
      <img
        src={imageSrc}
        alt={altText}
        width="120"
        height="148"
        style={{ lineHeight: '0' }}
      />
    </Box>
  );
}
