/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Image } from 'design';
import * as Icons from 'design/Icon';

import aws from './assets/aws.png';
import azure from './assets/azure.png';
import cockroach from './assets/cockroach.png';
import database from './assets/database.png';
import gcp from './assets/gcp.png';
import mongo from './assets/mongo.png';
import windows from './assets/windows.png';
import selfhosted from './assets/self-hosted.png';
import postgres from './assets/postgres.png';
import dynamo from './assets/dynamo.png';
import ec2 from './assets/ec2.png';
import eks from './assets/eks.png';
import jenkins from './assets/jenkins.png';
import linux from './assets/linux.png';
import mysql from './assets/mysql.png';
import redshift from './assets/redshift.png';
import slack from './assets/slack.png';
import snowflake from './assets/snowflake.png';
import k8s from './assets/kubernetes.png';
import server from './assets/server.png';
import application from './assets/application.png';
import grafana from './assets/grafana.png';

export const icons = {
  Apple: <Icons.Apple fontSize={22} />,
  Application: <Image src={application} width="23.9px" height="24px" />,
  Database: <Image src={database} width="23.9px" height="24px" />,
  Aws: <Image src={aws} width="24.65px" height="14.36px" />,
  Azure: <Image src={azure} width="23.9px" height="24px" />,
  Cockroach: <Image src={cockroach} width="23.9px" height="24px" />,
  Dynamo: <Image src={dynamo} width="23.9px" height="24px" />,
  Ec2: <Image src={ec2} width="23.9px" height="24px" />,
  Eks: <Image src={eks} width="23.9px" height="24px" />,
  Gcp: <Image src={gcp} width="23.9px" height="24px" />,
  Grafana: <Image src={grafana} width="23.9px" height="24px" />,
  Jenkins: <Image src={jenkins} width="23.9px" height="24px" />,
  Linux: <Image src={linux} width="23.9px" height="24px" />,
  Kube: <Image src={k8s} width="23.9px" height="24px" />,
  Mongo: <Image src={mongo} width="23.9px" height="24px" />,
  Mysql: <Image src={mysql} width="23.9px" height="24px" />,
  Postgres: <Image src={postgres} width="23.9px" height="24px" />,
  Redshift: <Image src={redshift} width="23.9px" height="24px" />,
  SelfHosted: <Image src={selfhosted} width="23.9px" height="24px" />,
  Server: <Image src={server} width="23.9px" height="24px" />,
  Slack: <Image src={slack} width="23.9px" height="24px" />,
  Snowflake: <Image src={snowflake} width="23.9px" height="24px" />,
  Windows: <Image src={windows} width="23.9px" height="24px" />,
  Unknown: <Icons.Question fontSize={22} />,
};

export type ResourceIconName = keyof typeof icons;
