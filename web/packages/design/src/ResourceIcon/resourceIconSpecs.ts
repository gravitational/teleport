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

import * as i from './icons';

/** Uses given icon for all themes. */
const forAllThemes = (icon: string): IconSpec => ({ dark: icon, light: icon });

/**
 * A map of icon name -> theme -> spec mapping of resource icons.
 *
 * Use all lowercap for naming the icon keys.
 *
 * Since the icon key names will be used against
 * matchers (eg: function guessAppIcon), name the icon
 * as close to the real names as possible.
 *
 * In case of duplicate icon names, name the keys with
 * their brand domain extension eg: apollo -> 'apollo.io'
 */
export const resourceIconSpecs = {
  activemq: forAllThemes(i.activemq),
  adobe: forAllThemes(i.adobe),
  adobecreativecloud: {
    dark: i.adobecreativecloudDark,
    light: i.adobecreativecloudLight,
  },
  adobemarketo: forAllThemes(i.adobemarketo),
  airbase: forAllThemes(i.airbase),
  airtable: forAllThemes(i.airtable),
  algolia: { dark: i.algoliaDark, light: i.algoliaLight },
  altisales: { dark: i.altisalesDark, light: i.altisalesLight },
  ansible: { dark: i.ansibleDark, light: i.ansibleLight },
  anthem: forAllThemes(i.anthem),
  'apollo.io': { dark: i.apolloIoDark, light: i.apolloIoLight },
  apple: { dark: i.appleDark, light: i.appleLight },
  application: forAllThemes(i.application),
  argocd: forAllThemes(i.argocd),
  asana: forAllThemes(i.asana),
  assemble: { dark: i.assembleDark, light: i.assembleLight },
  atlassian: forAllThemes(i.atlassian),
  atlassianbitbucket: forAllThemes(i.atlassianbitbucket),
  atlassianjiraservice: forAllThemes(i.atlassianjiraservicemanagement),
  atlassianstatus: forAllThemes(i.atlassianstatuspage),
  auth0: { dark: i.auth0Dark, light: i.auth0Light },
  avalara: forAllThemes(i.avalara),
  aws: { dark: i.awsDark, light: i.awsLight },
  awsaccount: forAllThemes(i.awsAccount),
  azure: forAllThemes(i.azure),

  bill: forAllThemes(i.bill),
  bonusly: forAllThemes(i.bonusly),
  box: forAllThemes(i.box),
  browserstack: forAllThemes(i.browserstack),

  calendly: forAllThemes(i.calendly),
  calm: forAllThemes(i.calm),
  captivateiq: { dark: i.captivateiqDark, light: i.captivateiqLight },
  careerminds: { dark: i.careermindsDark, light: i.careermindsLight },
  carta: { dark: i.cartaDark, light: i.cartaLight },
  checkly: forAllThemes(i.checkly),
  checkr: forAllThemes(i.checkr),
  circleci: { dark: i.circleciDark, light: i.circleciLight },
  clari: forAllThemes(i.clari),
  clearbit: forAllThemes(i.clearbit),
  clearfeed: forAllThemes(i.clearfeed),
  clickhouse: { dark: i.clickhouseDark, light: i.clickhouseLight },
  cloudflare: forAllThemes(i.cloudflare),
  cloudzero: forAllThemes(i.cloudzero),
  cockroach: { dark: i.cockroachDark, light: i.cockroachLight },
  coefficient: { dark: i.coefficientDark, light: i.coefficientLight },
  conveyor: forAllThemes(i.conveyor),
  cronitor: forAllThemes(i.cronitor),
  cultureamp: { dark: i.cultureampDark, light: i.cultureampLight },

  database: forAllThemes(i.database),
  datadog: { dark: i.datadogDark, light: i.datadogLight },
  dealhub: forAllThemes(i.dealhub),
  deel: { dark: i.deelDark, light: i.deelLight },
  desktop: forAllThemes(i.desktop),
  digicert: { dark: i.digicertDark, light: i.digicertLight },
  digitalocean: forAllThemes(i.digitalocean),
  discord: forAllThemes(i.discord),
  dmarcian: forAllThemes(i.dmarcian),
  docker: { dark: i.dockerDark, light: i.dockerLight },
  docusign: { dark: i.docusignDark, light: i.docusignLight },
  donut: forAllThemes(i.donut),
  drata: { dark: i.drataDark, light: i.drataLight },
  drift: { dark: i.driftDark, light: i.driftLight },
  dropbox: forAllThemes(i.dropbox),
  duo: { dark: i.duoDark, light: i.duoLight },
  dynamo: forAllThemes(i.dynamo),

  ec2: forAllThemes(i.ec2),
  eks: forAllThemes(i.eks),
  elastic: forAllThemes(i.elastic),
  email: { dark: i.emailDark, light: i.emailLight },
  entraid: forAllThemes(i.entraid),
  eventbrite: forAllThemes(i.eventbrite),
  excalidraw: forAllThemes(i.excalidraw),

  figma: forAllThemes(i.figma),
  fontawesome: forAllThemes(i.fontawesome),
  foqal: forAllThemes(i.foqal),
  fossa: forAllThemes(i.fossa),
  'frame.io': { dark: i.frameIoDark, light: i.frameIoLight },

  g2: forAllThemes(i.g2),
  gable: forAllThemes(i.gable),
  gem: { dark: i.gemDark, light: i.gemLight },
  git: { dark: i.gitDark, light: i.gitLight },
  github: { dark: i.githubDark, light: i.githubLight },
  gitlab: forAllThemes(i.gitlab),
  gmail: forAllThemes(i.gmail),
  go1: { dark: i.go1Dark, light: i.go1Light },
  goldcast: forAllThemes(i.goldcast),
  google: forAllThemes(i.google),
  googleanalytics: forAllThemes(i.googleanalytics),
  googlecalendar: forAllThemes(i.googlecalendar),
  googlecloud: forAllThemes(i.googlecloud),
  googledrive: forAllThemes(i.googledrive),
  googletag: forAllThemes(i.googletag),
  googlevoice: forAllThemes(i.googlevoice),
  grafana: forAllThemes(i.grafana),
  grammarly: forAllThemes(i.grammarly),
  grubhub: forAllThemes(i.grubhub),
  guideline: { dark: i.guidelineDark, light: i.guidelineLight },

  hackerone: { dark: i.hackeroneDark, light: i.hackeroneLight },
  headliner: forAllThemes(i.headliner),
  hootsuite: forAllThemes(i.hootsuite),
  cilium: { dark: i.ciliumDark, light: i.ciliumLight },

  ibm: { dark: i.ibmDark, light: i.ibmLight },
  inkeep: forAllThemes(i.inkeep),
  instruqt: { dark: i.instruqtDark, light: i.instruqtLight },
  intellimize: forAllThemes(i.intellimize),
  ipstack: forAllThemes(i.ipstack),

  jamf: forAllThemes(i.jamf),
  jenkins: forAllThemes(i.jenkins),
  jetbrains: forAllThemes(i.jetbrains),
  jira: forAllThemes(i.jira),

  kaiser: { dark: i.kaiserDark, light: i.kaiserLight },
  kisi: { dark: i.kisiDark, light: i.kisiLight },
  kollide: forAllThemes(i.kollide),
  kube: forAllThemes(i.kube),
  kubeserver: forAllThemes(i.kubeserver),

  laptop: forAllThemes(i.laptop),
  leadiq: forAllThemes(i.leadiq),
  leandata: forAllThemes(i.leandata),
  lever: forAllThemes(i.lever),
  linkedin: { dark: i.linkedinDark, light: i.linkedinLight },
  linux: { dark: i.linuxDark, light: i.linuxLight },
  loom: forAllThemes(i.loom),
  'lucid.co': { dark: i.lucidCoDark, light: i.lucidCoLight },
  lusha: { dark: i.lushaDark, light: i.lushaLight },

  mailgun: forAllThemes(i.mailgun),
  mariadb: { dark: i.mariadbDark, light: i.mariadbLight },
  mattermost: { dark: i.mattermostDark, light: i.mattermostLight },
  maxio: { dark: i.maxioDark, light: i.maxioLight },
  metabase: forAllThemes(i.metabase),
  microsoft: forAllThemes(i.microsoft),
  microsoftexcel: forAllThemes(i.microsoftexcel),
  microsoftonedrive: forAllThemes(i.microsoftonedrive),
  microsoftonenote: forAllThemes(i.microsoftonenote),
  microsoftoutlook: forAllThemes(i.microsoftoutlook),
  microsoftpowerpoint: forAllThemes(i.microsoftpowerpoint),
  microsoftteams: forAllThemes(i.microsoftteams),
  microsoftword: forAllThemes(i.microsoftword),
  mongo: { dark: i.mongoDark, light: i.mongoLight },
  mysqllarge: { dark: i.mysqlLargeDark, light: i.mysqlLargeLight },
  mysqlsmall: { dark: i.mysqlSmallDark, light: i.mysqlSmallLight },

  namecheap: forAllThemes(i.namecheap),
  navan: { dark: i.navanDark, light: i.navanLight },
  neverbounce: { dark: i.neverbounceDark, light: i.neverbounceLight },
  notion: forAllThemes(i.notion),

  oasisopen: forAllThemes(i.oasisopen),
  okta: { dark: i.oktaDark, light: i.oktaLight },
  oktaAlt: forAllThemes(i.oktaAlt),
  '101domain': forAllThemes(i.onehundredonedomain),
  onelogin: { dark: i.oneloginDark, light: i.oneloginLight },
  '1password': { dark: i.onepasswordDark, light: i.onepasswordLight },
  opencomp: forAllThemes(i.opencomp),
  openid: forAllThemes(i.openid),
  opsgenie: forAllThemes(i.opsgenie),
  'orbit.love': forAllThemes(i.orbitLove),
  orcasecurity: { dark: i.orcasecurityDark, light: i.orcasecurityLight },
  'outreach.io': forAllThemes(i.outreachIo),

  pagerduty: forAllThemes(i.pagerduty),
  panther: { dark: i.pantherDark, light: i.pantherLight },
  parallels: forAllThemes(i.parallels),
  pingdom: forAllThemes(i.pingdom),
  podigee: forAllThemes(i.podigee),
  polleverywhere: forAllThemes(i.polleverywhere),
  portswigger: forAllThemes(i.portswigger),
  postgres: forAllThemes(i.postgres),
  posthog: { dark: i.posthogDark, light: i.posthogLight },
  productboard: forAllThemes(i.productboard),
  prometheus: forAllThemes(i.prometheus),

  qualified: forAllThemes(i.qualified),
  quickbooks: forAllThemes(i.quickbooks),

  rabbitmq: forAllThemes(i.rabbitmq),
  rds: forAllThemes(i.rds),
  redhat: forAllThemes(i.redhat),
  redshift: forAllThemes(i.redshift),
  ringlead: forAllThemes(i.ringlead),
  rippling: { dark: i.ripplingDark, light: i.ripplingLight },

  salesforce: forAllThemes(i.salesforce),
  sanity: forAllThemes(i.sanity),
  scim: { dark: i.scimDark, light: i.scimLight },
  securecodewarrior: {
    dark: i.securecodewarriorDark,
    light: i.securecodewarriorLight,
  },
  semrush: forAllThemes(i.semrush),
  sendgrid: forAllThemes(i.sendgrid),
  sentry: { dark: i.sentryDark, light: i.sentryLight },
  sequoia: { dark: i.sequoiaDark, light: i.sequoiaLight },
  selfhosted: forAllThemes(i.database),
  server: forAllThemes(i.server),
  servicenow: forAllThemes(i.servicenow),
  shopify: forAllThemes(i.shopify),
  '6sense': { dark: i.sixsenseDark, light: i.sixsenseLight },
  skype: forAllThemes(i.skype),
  slab: forAllThemes(i.slab),
  slack: forAllThemes(i.slack),
  snowflake: forAllThemes(i.snowflake),
  spacelift: { dark: i.spaceliftDark, light: i.spaceliftLight },
  sparrow: forAllThemes(i.sparrow),
  stripe: { dark: i.stripeDark, light: i.stripeLight },

  tableau: forAllThemes(i.tableau),
  terraform: forAllThemes(i.terraform),
  torq: { dark: i.torqDark, light: i.torqLight },
  'troops.ai': { dark: i.troopsAiDark, light: i.troopsAiLight },
  twilio: forAllThemes(i.twilio),
  twitter: { dark: i.twitterDark, light: i.twitterLight },

  userorbit: { dark: i.userorbitDark, light: i.userorbitLight },

  validity: forAllThemes(i.validity),
  valimail: forAllThemes(i.valimail),
  varicent: { dark: i.varicentDark, light: i.varicentLight },
  vendr: forAllThemes(i.vendr),
  vercel: { dark: i.vercelDark, light: i.vercelLight },
  weavegitops: forAllThemes(i.weavegitops),

  windows: { dark: i.windowsDark, light: i.windowsLight },
  wiz: { dark: i.wizDark, light: i.wizLight },
  workshop: { dark: i.workshopDark, light: i.workshopLight },

  youtube: forAllThemes(i.youtube),

  zapier: forAllThemes(i.zapier),
  zendesk: { dark: i.zendeskDark, light: i.zendeskLight },
  zoom: forAllThemes(i.zoom),
  zoominfo: forAllThemes(i.zoominfo),
};

type IconSpec = {
  // svg icon for dark theme
  dark: string;
  // svg icon for light theme
  light: string;
};

export type ResourceIconName = keyof typeof resourceIconSpecs;

export const iconNames = Object.keys(resourceIconSpecs) as ResourceIconName[];
