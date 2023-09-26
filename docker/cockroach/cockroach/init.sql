CREATE DATABASE IF NOT EXISTS teleport_backend;
ALTER DATABASE teleport_backend ADD REGION "aws-us-east-1";
ALTER DATABASE teleport_backend ADD REGION "aws-us-west-2";
ALTER DATABASE teleport_backend ADD REGION "aws-eu-central-1";
ALTER DATABASE teleport_backend PRIMARY REGION 'aws-us-west-2';
ALTER DATABASE teleport_backend SET SECONDARY REGION 'aws-us-east-1';
ALTER DATABASE teleport_backend SURVIVE REGION FAILURE;
ALTER TABLE kv CONFIGURE ZONE USING gc.ttlseconds = 1200;

SET CLUSTER SETTING kv.rangefeed.enabled = true;
