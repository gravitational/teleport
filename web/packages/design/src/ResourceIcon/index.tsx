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

import React, { ComponentProps } from 'react';

import { useTheme } from 'styled-components';

import { Image } from 'design';

import appleDark from './assets/apple-dark.svg';
import appleLight from './assets/apple-light.svg';
import application from './assets/application.svg';
import awsDark from './assets/aws-dark.svg';
import awsLight from './assets/aws-light.svg';
import azure from './assets/azure.svg';
import cockroachDark from './assets/cockroach-dark.svg';
import cockroachLight from './assets/cockroach-light.svg';
import database from './assets/database.svg';
import dynamo from './assets/dynamo.svg';
import ec2 from './assets/ec2.svg';
import eks from './assets/eks.svg';
import gcp from './assets/gcp.svg';
import grafana from './assets/grafana.svg';
import jenkins from './assets/jenkins.svg';
import kube from './assets/kube.svg';
import laptop from './assets/laptop.svg';
import linuxDark from './assets/linux-dark.svg';
import linuxLight from './assets/linux-light.svg';
import mongoDark from './assets/mongo-dark.svg';
import mongoLight from './assets/mongo-light.svg';
import mysqlLargeDark from './assets/mysql-large-dark.svg';
import mysqlLargeLight from './assets/mysql-large-light.svg';
import mysqlSmallDark from './assets/mysql-small-dark.svg';
import mysqlSmallLight from './assets/mysql-small-light.svg';
import postgres from './assets/postgres.svg';
import redshift from './assets/redshift.svg';
import server from './assets/server.svg';
import slack from './assets/slack.svg';
import snowflake from './assets/snowflake.svg';
import windowsDark from './assets/windows-dark.svg';
import windowsLight from './assets/windows-light.svg';

interface ResourceIconProps extends ComponentProps<typeof Image> {
  /**
   * Determines which icon will be displayed. See `iconSpecs` for the list of
   * available names.
   */
  name: ResourceIconName;
}

/**
 * Displays a resource icon of a given name for current theme. The icon
 * component exposes props of the underlying `Image`.
 */
export const ResourceIcon = ({ name, ...props }: ResourceIconProps) => {
  const theme = useTheme();
  const icon = iconSpecs[name]?.[theme.type];
  if (!icon) {
    return null;
  }
  return <Image src={icon} {...props} />;
};

/** Uses given icon for all themes. */
const forAllThemes = icon => ({ dark: icon, light: icon });

/** A name->theme->spec mapping of resource icons. */
const iconSpecs = {
  Apple: { dark: appleDark, light: appleLight },
  Application: forAllThemes(application),
  Aws: { dark: awsDark, light: awsLight },
  Azure: forAllThemes(azure),
  Cockroach: { dark: cockroachDark, light: cockroachLight },
  Database: forAllThemes(database),
  Dynamo: forAllThemes(dynamo),
  Ec2: forAllThemes(ec2),
  Eks: forAllThemes(eks),
  Gcp: forAllThemes(gcp),
  Grafana: forAllThemes(grafana),
  Jenkins: forAllThemes(jenkins),
  Kube: forAllThemes(kube),
  Laptop: forAllThemes(laptop),
  Linux: { dark: linuxDark, light: linuxLight },
  Mongo: { dark: mongoDark, light: mongoLight },
  MysqlLarge: { dark: mysqlLargeDark, light: mysqlLargeLight },
  MysqlSmall: { dark: mysqlSmallDark, light: mysqlSmallLight },
  Postgres: forAllThemes(postgres),
  Redshift: forAllThemes(redshift),
  SelfHosted: forAllThemes(database),
  Server: forAllThemes(server),
  Slack: forAllThemes(slack),
  Snowflake: forAllThemes(snowflake),
  Windows: { dark: windowsDark, light: windowsLight },
};

export type ResourceIconName = keyof typeof iconSpecs;

/** All icon names, exported for testing purposes. */
export const iconNames = Object.keys(iconSpecs) as ResourceIconName[];
