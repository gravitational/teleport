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
      This guide will allow you to automatically enroll one or more RDS
      databases within a single VPC in your AWS account.
    </li>
    <li>
      If you have not already set up an OIDC connection to the AWS account you
      want to use, you will be led through that process.
    </li>
    {!cfg.isCloud ? (
      <li>
        If you have not done so previously, you will set up a server within your
        network running the Teleport binary, to act as a Discovery service
      </li>
    ) : null}
    <li>
      You will run a script in AWS CloudShell to create a role allowing Teleport
      to deploy a service in ECS.
    </li>
    <li>
      If you have not done so previously for this VPC, you will deploy an ECS
      service running Teleport to act as the Discovery service.
    </li>
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
  <ul>
    <li>
      This guide will enroll a single server in your Teleport cluster for SSH
      access.
    </li>
    <li>
      You will run a bash script on the server that will install the Teleport
      binary and configure it using a short-lived, randomly generated join
      token.
    </li>
  </ul>
);

const serverPrerequisites = () => (
  <ul>
    <li>
      SSH access to the server, and root or sudo privileges to run the script.
    </li>
    <li>List of OS users you want to be able to connect as.</li>
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
    PrerequisiteContent: () => serverPrerequisites(),
  },
  [DiscoverGuideId.ServerLinuxDebian]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => serverPrerequisites(),
  },
  [DiscoverGuideId.ServerLinuxUbuntu]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => serverPrerequisites(),
  },
  [DiscoverGuideId.ServerLinuxRhelCentos]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => serverPrerequisites(),
  },
  [DiscoverGuideId.ServerMac]: {
    OverviewContent: () => serverOverview(),
    PrerequisiteContent: () => serverPrerequisites(),
  },
  [DiscoverGuideId.ApplicationWebHttpProxy]: {
    OverviewContent: () => <div></div>,
    PrerequisiteContent: () => <div></div>,
  },
  [DiscoverGuideId.KubernetesAwsEks]: {
    OverviewContent: () => (
      <ul>
        <li>
          If you have not already set up an OIDC connection to the AWS account
          you want to use, you will be led through that process.
        </li>
        {!cfg.isCloud ? (
          <li>
            If you have not done so previously, you will set up a server within
            your network running the Teleport binary, to act as a Discovery
            service.
          </li>
        ) : null}
        <li>
          You will run a script in AWS CloudShell to configure your OIDC policy
          to allow EKS access.
        </li>
        <li>
          You will pick the region and clusters you want to enroll in Teleport.
        </li>
        <li>
          You will add the Kubernetes users and groups you want Teleport users
          to be able to authenticate as.
        </li>
        <li>You will test your connection.</li>
      </ul>
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
        <li>
          If you have not already set up an OIDC connection to the AWS account
          you want to use, you will be led through that process.
        </li>
        {!cfg.isCloud ? (
          <li>
            If you have not done so previously, you will set up a server within
            your network running the Teleport binary, to act as a Discovery
            service.
          </li>
        ) : null}
        <li>
          You will run a script in AWS CloudShell to configure your OIDC policy
          to allow EC2 Auto Discovery.
        </li>
        <li>You will test your connection.</li>
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
        <li>
          If you have not already set up an OIDC connection to the AWS account
          you want to use, you will be led through that process.
        </li>
        {rdsOverview()}
        <li>
          You will add the RDS authentication plugin to a database user, and
          potentially change grants for that user. This can be an existing user,
          but that user will no longer be able to authenticate any other way, so
          we recommend creating a new user for this step. If you cannot access
          the database directly, doing it in a CloudShell instance running in a
          VPC is a good alternative.
        </li>
        <li>You will test your connection.</li>
      </ul>
    ),
    PrerequisiteContent: () => <div>{rdsPrerequisites()}</div>,
  },
  [DiscoverGuideId.DatabaseAwsRdsAuroraPostgresSql]: {
    OverviewContent: () => (
      <ul>
        <li>
          If you have not already set up an OIDC connection to the AWS account
          you want to use, you will be led through that process.
        </li>
        {rdsOverview()}
        <li>
          You will grant the role rds_iam to a user in the database server. This
          can be an existing user, but that user will no longer be able to
          authenticate any other way, so we recommend creating a new user for
          this step. If you cannot access the database directly, doing it in a
          CloudShell instance running in a VPC is a good alternative.
        </li>
        <li>You will test your connection.</li>
      </ul>
    ),
    PrerequisiteContent: () => <div>{rdsPrerequisites()}</div>,
  },
  [DiscoverGuideId.DatabaseAwsRdsPostgresSql]: {
    OverviewContent: () => (
      <ul>
        <li>
          If you have not already set up an OIDC connection to the AWS account
          you want to use, you will be led through that process.
        </li>
        {rdsOverview()}
        <li>
          You will grant the role rds_iam to a user in the database server. This
          can be an existing user, but that user will no longer be able to
          authenticate any other way, so we recommend creating a new user for
          this step. If you cannot access the database directly, doing it in a
          CloudShell instance running in a VPC is a good alternative.
        </li>
        <li>You will test your connection.</li>
      </ul>
    ),
    PrerequisiteContent: () => <div>{rdsPrerequisites()}</div>,
  },
  [DiscoverGuideId.DatabaseAwsRdsMysqlMariaDb]: {
    OverviewContent: () => (
      <ul>
        <li>
          If you have not already set up an OIDC connection to the AWS account
          you want to use, you will be led through that process.
        </li>
        {rdsOverview()}
        <li>
          You will add the RDS authentication plugin to a database user, and
          potentially change grants for that user. This can be an existing user,
          but that user will no longer be able to authenticate any other way, so
          we recommend creating a new user for this step. If you cannot access
          the database directly, doing it in a CloudShell instance running in a
          VPC is a good alternative.
        </li>
        <li>You will test your connection.</li>
      </ul>
    ),
    PrerequisiteContent: () => <div>{rdsPrerequisites()}</div>,
  },
};
