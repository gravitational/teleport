---

authors: Marek Smoliński (marek@goteleport.com)
state: draft
---
## Required Approvers

- Engineering: `@r0mant`
- Product: `@klizhentas || @xinding33`
- Security: `@reedloden || @jentfoo`

# RFD 0115 - Teleport Oracle Access Integration

## What


This RFD outlines the scope of Oracle integration with Teleport Database Access.
Teleport Database Oracle Access integration will leverage the details about Oracle Protocol shared in Official Oracle GH repository  https://github.com/oracle/python-oracledb/tree/main/src/oracledb/impl released under Apache License, Version 2.0.

## Why

Adding Oracle support to Teleport Database Access would allow our shared customer bases to take advantage of Teleport's access controls and audit logging capabilities when accessing their Oracle databases.

* Administrators will be able to control access to their entire fleet of database servers through their identity provider with SSO.
* Users will be able to connect to Oracle databases with the tools they're familiar with without having to deal with passwords or shared secrets.
* Auditors will be able to view the database activity and tie it to a particular user identity within an organization.


# Scope of Integration
-- **Teleport as Oracle Access Proxy**: Teleport should be able to act like a proxy between the incoming Oracle client connection and connection to Oracle Server where the Teleport will terminate the incoming TLS connection and establish a new TLS connection to the Oracle Server using a new TLS Certificate and forward the traffic between Oracle client and server.
- **Audit Logging**: Teleport needs to be able to inspect the Oracle wire protocol to provide Teleport audit logs and audit client interaction with Oracle database.


### TLS Termination of Incoming connection:
Teleport needs to be able to TLS-terminate incoming Oracle client connection and reestablish a new TLS connection that uses Teleport-signed client certificate to the Oracle Server. Oracle database server will need to be configured to trust Teleport's certificate authority to be able to validate client connections.



## Details
### Oracle Server versions requirement
Oracle Server in different version seems to have different Server TLS Certs validation
where integrate with current for of  Teleport DatabaseCA was successfully tested on following Oracle Server Versions:
- Oracle Database 21c (21.3.0) Enterprise Edition and Express Edition (XE)
- Oracle Database 19c (19.3.0) Enterprise Edition
- Oracle Database 18c (18.3.0) Express Edition (XE)


Following Oracle Server versions was not compatible with Teleport DatabaseCA:
- Oracle Database 12c Release 2 (12.2.0.2) Enterprise Edition
- Oracle Database 12c Release 1 (12.1.0.2) Enterprise Edition
 
The incompatibility with DatabaseCA seems to be related to our [GenerateSelfSignedCAWithConfig](https://github.com/gravitational/teleport/blob/master/lib/tlsca/parsegen.go#L91) logic where the `Entity.SerialNumber` is added to Issuer and Subject cert fields.

### Teleport Database Access Configuration:
 
#### Oracle Client:

Oracle clients support TLS connections to the Oracle Server by using a custom container called [Oracle Wallet](https://docs.oracle.com/cd/E92519_02/pt856pbr3/eng/pt/tsvt/concept_UnderstandingOracleWallet.html#:~:text=Oracle%20Wallet%20is%20a%20container,is%20used%20for%20security%20credentials.
) that stores authentication credentials and certificates.
Teleport `tsh db login` command for Oracle database will generate cert in Oracle Wallet format allowing to configure the wallet in Oracle database clients like [sqlcli](https://www.oracle.com/pl/database/sqldeveloper/technologies/sqlcl/) or [SQL Oracle Developer](https://www.oracle.com/database/sqldeveloper/)

##### UX:

Teleport will integrate with Oracle in the same way as other databases.

* `tsh db connect` - would start [sqlcli](https://www.oracle.com/pl/database/sqldeveloper/technologies/sqlcl/) Oracle CLI.
   ```
  $ tsh db connect --db-user=alice --db-name=XE oracle

    SQLcl: Release 22.4 Production on Fri Mar 17 15:03:14 2023

    Copyright (c) 1982, 2023, Oracle.  All rights reserved.

    Connected to:
    Oracle Database 21c Express Edition Release 21.0.0.0.0 - Production
    Version 21.3.0.0.0

    SQL>
  ```
* `tsh proxy db` - would start proxy for 3rd party GUI clients like  [SQL Oracle Developer](https://www.oracle.com/database/sqldeveloper/)
    ```bash
    $ tsh proxy db oracle  --db-user=alice --db-name=XE --tunnel
    Started authenticated tunnel for the Oracle database "oracle" in cluster "ice-berg.dev" on 127.0.0.1:51584.
    To avoid port randomization, you can choose the listening port using the --port flag.

    Use the following command to connect to the Oracle database server using CLI:
      $ sql -L jdbc:oracle:thin:@tcps://localhost:51584/XE?TNS_ADMIN=/Users/marek/.tsh/keys/ice-berg.dev/marek-db/ice-berg.dev/oracle-wallet

    or using following Oracle JDBC connection string in order to connect with other GUI/CLI clients:
      jdbc:oracle:thin:@tcps://localhost:51584/XE?TNS_ADMIN=/Users/marek/.tsh/keys/ice-berg.dev/marek-db/ice-berg.dev/oracle-wallet

    ```


* `tctl auth sign` with oracle format:
 
    The new `tctl auth sign` `--format=oracle` sign format will be introduced where Teleport certificate authority and generated certificate/key pair will be stored in Oracle Wallet SSO autologin format:

    Oracle Server Wallet uses special format and there is not any OSS library that provides ability to Create Oracle Wallet from PEM keypair.
    During `tctl  auth sign --format=oracle` execution we will try to detect if the [orapki](https://docs.oracle.com/database/121/DBSEG/asoappf.htm#DBSEG610) tool that allow to manage Oracle wallet is available in use environment.
    If the `tctl` will auto generate `cwallet.sso` autologin Oracle Wallet.
    ```
    $ tctl  auth sign --format=oracle --host=localhost --out=certs/server --ttl=2190h


    To enable mutual TLS on your Oracle server, add the following settings to oracle sqlnet.ora configuration file:

    WALLET_LOCATION = (SOURCE = (METHOD = FILE)(METHOD_DATA = (DIRECTORY = /path/to/oracleWalletDir)))
    SSL_CLIENT_AUTHENTICATION = TRUE
    SQLNET.AUTHENTICATION_SERVICES = (TCPS)


    To enable mutual TLS on your Oracle server, add the following TCPS entreis to its listener.ora configuration file:

    LISTENER =
      (DESCRIPTION_LIST =
        (DESCRIPTION =
          (ADDRESS = (PROTOCOL = TCPS)(HOST = 0.0.0.0)(PORT = 2484))
        )
      )

    WALLET_LOCATION = (SOURCE = (METHOD = FILE)(METHOD_DATA = (DIRECTORY = /path/to/oracleWalletDir)))
    SSL_CLIENT_AUTHENTICATION = TRUE
    ```
    otherwise if orapki tool is not available during `tctl  auth sign --format=oracle` the help command will guide user how to convert certs to Oracle Wallet on the Oracle Server Instance:
    ```bash
    $ tctl  auth sign --format=oracle --host=localhost --out=certs/server --ttl=2190h

    Orapki binary was not found. Please create oracle wallet file manually by running following commands on the Oracle server:

    orapki wallet create -wallet certs -auto_login_only
    orapki wallet import_pkcs12 -wallet certs -auto_login_only -pkcs12file certs/server.p12 -pkcs12pwd b0010fc2b57d190a60f0dd154e2a7d0a6bde6e6637aed27e0085b2e0c0edab49
    orapki wallet add -wallet certs -trusted_cert -auto_login_only -cert certs/server.crt

    To enable mutual TLS on your Oracle server, add the following settings to oracle sqlnet.ora configuration file:

    WALLET_LOCATION = (SOURCE = (METHOD = FILE)(METHOD_DATA = (DIRECTORY = /path/to/oracleWalletDir)))
    SSL_CLIENT_AUTHENTICATION = TRUE
    SQLNET.AUTHENTICATION_SERVICES = (TCPS)


    To enable mutual TLS on your Oracle server, add the following TCPS entreis to its listener.ora configuration file:

    LISTENER =
      (DESCRIPTION_LIST =
        (DESCRIPTION =
          (ADDRESS = (PROTOCOL = TCPS)(HOST = 0.0.0.0)(PORT = 2484))
        )
      )

    WALLET_LOCATION = (SOURCE = (METHOD = FILE)(METHOD_DATA = (DIRECTORY = /path/to/oracleWalletDir)))
    SSL_CLIENT_AUTHENTICATION = TRUE

    ```


#### Oracle Server Setup:
##### Other orapki tool that allows to create Oracle Server Wallet  `tctl` flow will authoritatively convert PEM cert into oracle wallet:
Generated Oracle Wallet will be used in Oracle server [sqlnet.ora](https://docs.oracle.com/cd/E11882_01/network.112/e10835/sqlnet.htm#NETRF416) configuration file:
```
SSL_CLIENT_AUTHENTICATION = TRUE
SQLNET.AUTHENTICATION_SERVICES = (TCPS)
WALLET_LOCATION =
  (SOURCE =
    (METHOD = FILE)
    (METHOD_DATA =
      (DIRECTORY = /path/to/server/wallet)
    )
  )
```

and in [listener.ora](https://docs.oracle.com/database/121/NETRF/listener.htm#NETRF008) configuration file:
```
SSL_CLIENT_AUTHENTICATION = TRUE
WALLET_LOCATION =
  (SOURCE =
    (METHOD = FILE)
    (METHOD_DATA =
      (DIRECTORY =  /path/to/server/wallet)
    )
  )

LISTENER =
   (DESCRIPTION_LIST =
     (DESCRIPTION =
       (ADDRESS = (PROTOCOL = TCPS)(HOST = 0.0.0.0)(PORT = 2484))
     )
   )
```

Additionally, the following server parameters to will be set to enable TLS authentication on the server side:
\
[SQLNET.AUTHENTICATION_SERVICES](https://docs.oracle.com/cd/E11882_01/network.112/e10835/sqlnet.htm#NETRF2035)
\
[SSL_CLIENT_AUTHENTICATION](https://docs.oracle.com/cd/E11882_01/network.112/e10835/sqlnet.htm#NETRF233)

#### Create a OracleDB User wth TLS x509 DN Authentication:
Oracle server allows to authenticate database user based on the certificate CN field:

```
CREATE USER alice IDENTIFIED EXTERNALLY AS 'CN=alice';
```
Ref: [Configuring Authentication Using PKI Certificates for Centrally Managed Users](https://docs.oracle.com/en/database/oracle/oracle-database/19/dbseg/integrating_mads_with_oracle_database.html#GUID-1EF17156-3FA4-4EDD-8DFF-F98EB3A926BF)

## Security
Teleport Oracle Database access will not differ from other supported database protocols in terms of security.
The connection between Teleport Database Agent and Oracle Server will be secured by TLS 1.2 and mutual TLS authentication.
