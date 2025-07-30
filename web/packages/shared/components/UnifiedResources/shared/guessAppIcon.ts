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

import {
  iconNames,
  ResourceIconName,
  resourceIconSpecs,
} from 'design/ResourceIcon';
import { AppSubKind } from 'shared/services';

import { UnifiedResourceApp } from '../types';

export function guessAppIcon(resource: UnifiedResourceApp): ResourceIconName {
  const { awsConsole = false, name, friendlyName, labels, subKind } = resource;

  // Label matching takes precedence and we can assume it can be a direct lookup
  // since we expect a certain format.
  const labelIconValue = labels?.find(l => l.name === 'teleport.icon')?.value;
  if (labelIconValue === 'default') {
    // Allow opting out of a specific icon.
    return 'application';
  }
  if (labelIconValue && resourceIconSpecs[labelIconValue]) {
    return labelIconValue as ResourceIconName;
  }

  if (awsConsole) {
    return 'aws';
  }
  if (subKind === AppSubKind.AwsIcAccount) {
    return 'awsaccount';
  }

  const app = {
    name: withoutWhiteSpaces(name)?.toLocaleLowerCase(),
    friendlyName: withoutWhiteSpaces(friendlyName)?.toLocaleLowerCase(),
  };

  // Try a direct lookup first.
  if (resourceIconSpecs[app.name]) {
    return app.name as ResourceIconName;
  }
  if (app.friendlyName && resourceIconSpecs[app.friendlyName]) {
    return app.friendlyName as ResourceIconName;
  }

  // Help match brands with sub brands:
  if (match('adobe', app)) {
    if (match('creative', app)) return 'adobecreativecloud';
    if (match('marketo', app)) return 'adobemarketo';
    return 'adobe'; // generic
  }
  if (match('atlassian', app)) {
    if (match('bitbucket', app)) return 'atlassianbitbucket';
    if (match('jiraservice', app)) return 'atlassianjiraservice';
    if (match('status', app)) return 'atlassianstatus';
    return 'atlassian'; // generic
  }
  if (match('google', app)) {
    if (match('analytic', app)) return 'googleanalytics';
    if (match('calendar', app)) return 'googlecalendar';
    if (match('cloud', app)) return 'googlecloud';
    if (match('drive', app)) return 'googledrive';
    if (match('tag', app)) return 'googletag';
    if (match('voice', app)) return 'googlevoice';
    return 'google'; // generic
  }
  if (match('microsoft', app)) {
    if (match('excel', app)) return 'microsoftexcel';
    if (match('drive', app)) return 'microsoftonedrive';
    if (match('note', app)) return 'microsoftonenote';
    if (match('outlook', app)) return 'microsoftoutlook';
    if (match('powerpoint', app)) return 'microsoftpowerpoint';
    if (match('team', app)) return 'microsoftteams';
    if (match('word', app)) return 'microsoftword';
    return 'microsoft'; // generic
  }
  if (match('gcp', app)) {
    return 'googlecloud';
  }
  if (match('azure', app)) {
    return 'azure';
  }

  // Try matching by iterating through all the icon names
  const matchingIcon = iconNames.find(iconName => match(iconName, app));

  return matchingIcon || 'application'; // couldn't match anything
}

function match(
  target: string,
  {
    name,
    friendlyName,
  }: {
    name: string;
    friendlyName?: string;
  }
) {
  return name?.includes(target) || friendlyName?.includes(target);
}

/**
 * Strips characters like dashes and white space and strips
 * paranthesis and brackets and whatever words were inside of them.
 *
 * - Dashes may be a common separator for the app `name` field.
 * - White spaces may be a common separator for `friendlyName` field.
 * - Words inside paranthesis/brackets may contain other unrelated
 *   keywords eg: "Clearfeed (Google Auth)"
 */
function withoutWhiteSpaces(text?: string) {
  if (!text) {
    return '';
  }

  // Remove paranthesis and brackets and words inside them.
  let modifiedText = text.replace(/\[[^\]]*\]|\([^)]*\)/g, '');
  // If for whatever reason the whole text begain
  // with a paranthesis or bracket.
  if (!modifiedText) {
    modifiedText = text;
  }
  // Remove rest of characters.
  return modifiedText.replace(/-|\s/g, '');
}
