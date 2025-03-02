import cfg from 'teleport/config';
import { Overview } from 'teleport/Discover/Shared/Overview/types';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

const oidcPermissions = () => (
  <ul>
    <li>
      <code>iam:CreateOpenIDConnectProvider</code>
    </li>
    <li>
      <code>iam:TagOpenIDConnectProvider</code>
    </li>
    <li>
      <code>iam:CreateRole</code>
    </li>
    <li>
      <code>iam:CreatePolicy</code>
    </li>
    <li>
      <code>iam:AttachRolePolicy</code>
    </li>
  </ul>
);

// We don't know if there is a connector yet in this step, if we can make
// that available then we can take the AWS OIDC connector block out of the permissions list.
// That's the biggest one and would clean it up a lot.
// Same goes for EKS and RDS
const ecsPermissions = () => (
  <div>
    <li>
      If you do not have an existing AWS OIDC connector set up, you will need
      the following permissions in your AWS account:
    </li>
    {oidcPermissions()}
    <li>
      You will also need the following permissions to create the ECS cluster:
    </li>
    <ul>
      <li>
        <code>ecs:CreateCluster</code>
      </li>
      <li>
        <code>ecs:CreateService</code>
      </li>
      <li>
        <code>ecs:CreateTaskSet</code>
      </li>
      <li>
        <code>ecs:TagResource</code>
      </li>
    </ul>
  </div>
);

const rdsPrerequisites = () => (
  <ul>
    <li>The VPC of the RDS databases you want to enroll.</li>
    <li>
      At least one subnet in the VPC with a route to an internet gateway, and
      allows communication with the subnets of the RDS databases.
    </li>
    <li>
      At least one security group that allows egress to the Teleport cluster,
      and communication with the RDS databases.
    </li>
    <li>
      Ability to create a new user for Teleport to connect as, and to grant
      roles to that user.
    </li>
    <li>
      List of database names within the database server you want to connect to.
    </li>
    {ecsPermissions()}
  </ul>
);

const rdsOverview = () => (
  <div>
    <li>
      In this guide you will set up auto-enrollment of one db or more RDS
      databases in a region. You will deploy a Database Service in AWS ECS that
      proxies to the databases. We provide options for doing this automatically
      or manually.
    </li>
    {!cfg.isCloud ? (
      <li>
        If you have not done so previously, you will set up a server within your
        network running the Teleport binary, to act as a Discovery service
      </li>
    ) : null}
  </div>
);

const ec2Permissions = () => (
  <div>
    <li>
      If you do not have an existing AWS OIDC connector set up, you will need
      the following permissions in your AWS account:
    </li>
    {oidcPermissions()}
    <li>
      You will also need the following permissions to enable EC2 Auto Discovery:
    </li>
    <ul>
      <li>
        <code>iam:AddRoleToInstanceProfile</code>
      </li>
      <li>
        <code>iam:PutRolePolicy</code>
      </li>
      <li>
        <code>ssm:CreateDocument</code>
      </li>
    </ul>
  </div>
);

const eksPermissions = () => (
  <div>
    <li>
      If you do not have an existing AWS OIDC connector set up, you will need
      the following permissions in your AWS account:
    </li>
    {oidcPermissions()}
    <li>
      You will also need the following permissions to enable EKS Auto Discovery:
    </li>
    <ul>
      <li>
        <code>iam:PutRolePolicy</code>
      </li>
    </ul>
  </div>
);

const awsConsolePermissions = () => (
  <div>
    <li>
      If you do not have an existing AWS OIDC connector set up, you will need
      the following permissions in your AWS account:
    </li>
    {oidcPermissions()}
    <li>
      You will also need the following permissions to enable AWS Console access:
    </li>
    <ul>
      <li>
        <code>iam:PutRolePolicy</code>
      </li>
    </ul>
  </div>
);

const serverOverview = () => (
  <p>
    In this guide you will set up a single server in your Teleport cluster for
    SSH access by running a script to install an agent binary and a
    teleport.yaml config file. It uses a short-lived, randomly generated join
    token with Node permissions. It is good for getting quickly started with
    server access.
  </p>
);

const linuxPrerequisites = () => (
  <ul>
    <li>
      SSH access to the server, and root or sudo privileges to run the script.
    </li>
    <li>List of OS users you want to be able to connect as.</li>
    <li>A supported OS version:</li>
    {linuxSupportedVersions()}
  </ul>
);

const linuxSupportedVersions = () => (
  <ul>
    <li>Debian 10+</li>
    <li>Ubuntu 18.04+</li>
    <li>RHEL/CentOS Stream 9+</li>
    <li>Amazon Linux 2/2023+</li>
  </ul>
);

const macPrerequisites = () => (
  <ul>
    <li>
      SSH access to the server, and root or sudo privileges to run the script.
    </li>
    <li>List of OS users you want to be able to connect as.</li>
    <li>MacOS ?+</li>
  </ul>
);

export const content: { [key in DiscoverGuideId]?: Overview } = {
  [DiscoverGuideId.Kubernetes]: {
    OverviewContent: () => (
      <ul>
        <li>
          This guide uses Helm to install the Teleport agent into a cluster, and
          by default turns on auto-discovery of all apps in the cluster.
        </li>
      </ul>
    ),
    PrerequisiteContent: () => (
      <ul>
        <li>Network egress from your Kubernetes cluster to Teleport.</li>
        <li>Helm installed on your local machine.</li>
        <li>Kubernetes API access to install the Helm chart.</li>
      </ul>
    ),
  },
  [DiscoverGuideId.DatabasePostgresSql]: {
    OverviewContent: () => (
      <ul>
        <li>
          If you have not done so previously, you will configure and run a
          Teleport binary to act as a proxy to the database. This can either on
          the same server as the database, or you can set up a new server that
          can proxy multiple database servers.
        </li>
        <li>
          You will configure mTLS between the Teleport proxy and the database.
        </li>
        <li>
          You will configure the database users and database names you want
          Teleport to be able to use.
        </li>
        <li>You will test your connection.</li>
      </ul>
    ),
    PrerequisiteContent: () => (
      <ul>
        <li>
          List of database names within the database server you want to connect
          to.
        </li>
        <li>List of database users you want to be able to connect as.</li>
        <li>Database hostname and port.</li>
        <li>
          Copy of the database CA certificate if it is signed by a third-party
          or private CA.
        </li>
        <li>Ability to modify the pg_auth file on the database server.</li>
        <li>
          SSH or tsh access to the server running the database, and ability to
          either SCP files, or run a command to retrieve TLS certificates from
          the Teleport cluster.
        </li>
        <li>
          Proxy server must have network egress to the Teleport cluster, and
          ability to reach the database via a hostname and port.
        </li>
      </ul>
    ),
  },
  [DiscoverGuideId.DatabaseMysql]: {
    OverviewContent: () => (
      <ul>
        <li>
          If you have not done so previously, you will configure and run a
          Teleport binary to act as a proxy to the database. This can either on
          the same server as the database, or you can set up a new server that
          can proxy multiple database servers.
        </li>
        <li>
          You will configure mTLS between the Teleport proxy and the database.
        </li>
        <li>
          You will configure the database users and database names you want
          Teleport to be able to use.
        </li>
        <li>You will test your connection.</li>
      </ul>
    ),
    PrerequisiteContent: () => (
      <ul>
        <li>
          List of database names within the database server you want to connect
          to.
        </li>
        <li>List of database users you want to be able to connect as.</li>
        <li>Database hostname and port.</li>
        <li>
          Copy of the database CA certificate if it is signed by a third-party
          or private CA.
        </li>
        <li>Ability to modify the client certificate acceptance per user.</li>
        <li>
          SSH or tsh access to the server running the database, and ability to
          either SCP files, or run a command to retrieve TLS certificates from
          the Teleport cluster.
        </li>
        <li>
          Proxy server must have network egress to the Teleport cluster, and
          ability to reach the database via a hostname and port.
        </li>
      </ul>
    ),
  },
  [DiscoverGuideId.ServerLinuxAmazon]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => linuxPrerequisites(),
  },
  [DiscoverGuideId.ServerLinuxDebian]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => linuxPrerequisites(),
  },
  [DiscoverGuideId.ServerLinuxUbuntu]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => linuxPrerequisites(),
  },
  [DiscoverGuideId.ServerLinuxRhelCentos]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => linuxPrerequisites(),
  },
  [DiscoverGuideId.ServerMac]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => macPrerequisites(),
  },
  [DiscoverGuideId.ApplicationWebHttpProxy]: {
    OverviewContent: () => (
      <p>
        In this guide you will set up access to a http application by running a
        script that installs an agent binary and a teleport.yaml config file. It
        uses a short-lived join token with App permissions.
      </p>
    ),
    PrerequisiteContent: () => (
      <ul>
        <li>
          SSH access to the server, and root or sudo privileges to run the
          script.
        </li>
        <li>
          If teleport is already running to connect the node as a protected
          resource, you will need to make manual changes: add “app” to token
          roles, add the app config, and restart the teleport service.
        </li>
      </ul>
    ),
  },
  [DiscoverGuideId.KubernetesAwsEks]: {
    OverviewContent: () => (
      <div>
        <p>
          In this guide you will set up auto-enrollment of one or more EKS
          clusters with Teleport by running scripts in AWS CloudShell.
          {!cfg.isCloud ? (
            <div>
              If you have not done so previously, you will set up a server
              within your network running the Teleport binary to act as a
              Discovery service.
            </div>
          ) : null}
        </p>
      </div>
    ),
    PrerequisiteContent: () => (
      <ul>
        <li>
          Your clusters must have access entries authentication mode enabled.
        </li>
        <li>
          The <code>access</code> role or another role that will allow you to
          access the EKS cluster based on the labels attached to it.
        </li>
        <li>The region and cluster names you want to enroll in Teleport.</li>
        <li>
          The Kubernetes users and groups you want Teleport users to be able to
          authenticate as.
        </li>
        {eksPermissions()}
      </ul>
    ),
  },
  [DiscoverGuideId.ApplicationAwsCliConsole]: {
    OverviewContent: () => (
      <ul>
        <li>
          If you have not already set up an OIDC connection to the AWS account
          you want to use, you will be led through that process.
        </li>
        <li>
          You will run a script in AWS CloudShell to configure your OIDC policy
          to allow console access.
        </li>
        <li>You will test your connection.</li>
      </ul>
    ),
    PrerequisiteContent: () => (
      <ul>
        <li>List of AWS roles you want to be able to authenticate as.</li>
        {awsConsolePermissions()}
      </ul>
    ),
  },
  [DiscoverGuideId.ServerAwsEc2Auto]: {
    OverviewContent: () => (
      <ul>
        <li>
          This guide is used to enroll <i>all</i> EC2 instances controlled by
          Systems/Fleet Manager in a single geographical region in your AWS
          account.
        </li>
        {!cfg.isCloud ? (
          <li>
            If you have not done so previously, you will set up a server within
            your network running the Teleport binary, to act as a Discovery
            service.
          </li>
        ) : null}
      </ul>
    ),
    PrerequisiteContent: () => (
      <ul>
        <li>
          List of OS users you want to be able to connect as (i.e. root, ubuntu,
          etc).
        </li>
        <li>
          Your EC2 instances must have the SSM Agent running, and have the
          AmazonSSMManagedInstanceCore policy attached to their IAM profile.
        </li>
        {ec2Permissions()}
      </ul>
    ),
  },
  [DiscoverGuideId.DatabaseAwsRdsAuroraMysql]: {
    OverviewContent: () => (
      <ul>
        {rdsOverview()}
        <li>
          You will add the RDS authentication plugin to a database user, and
          potentially change grants for that user. This can be an existing user,
          but that user will no longer be able to authenticate any other way, so
          we recommend creating a new user for this step. If you cannot access
          the database directly, doing it in a CloudShell instance running in a
          VPC is a good alternative.
        </li>
      </ul>
    ),
    PrerequisiteContent: () => <div>{rdsPrerequisites()}</div>,
  },
  [DiscoverGuideId.DatabaseAwsRdsAuroraPostgresSql]: {
    OverviewContent: () => (
      <ul>
        {rdsOverview()}
        <li>
          You will grant the role rds_iam to a user in the database server. This
          can be an existing user, but that user will no longer be able to
          authenticate any other way, so we recommend creating a new user for
          this step. If you cannot access the database directly, doing it in a
          CloudShell instance running in a VPC is a good alternative.
        </li>
      </ul>
    ),
    PrerequisiteContent: () => <div>{rdsPrerequisites()}</div>,
  },
  [DiscoverGuideId.DatabaseAwsRdsPostgresSql]: {
    OverviewContent: () => (
      <ul>
        {rdsOverview()}
        <li>
          You will grant the role rds_iam to a user in the database server. This
          can be an existing user, but that user will no longer be able to
          authenticate any other way, so we recommend creating a new user for
          this step. If you cannot access the database directly, doing it in a
          CloudShell instance running in a VPC is a good alternative.
        </li>
      </ul>
    ),
    PrerequisiteContent: () => <div>{rdsPrerequisites()}</div>,
  },
  [DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb]: {
    OverviewContent: () => (
      <ul>
        {rdsOverview()}
        <li>
          You will add the RDS authentication plugin to a database user, and
          potentially change grants for that user. This can be an existing user,
          but that user will no longer be able to authenticate any other way, so
          we recommend creating a new user for this step. If you cannot access
          the database directly, doing it in a CloudShell instance running in a
          VPC is a good alternative.
        </li>
      </ul>
    ),
    PrerequisiteContent: () => <div>{rdsPrerequisites()}</div>,
  },
};
