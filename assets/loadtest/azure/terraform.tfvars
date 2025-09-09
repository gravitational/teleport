# these locations should support HA zone-redundant Postgres and have ~200 DSv3
# cpus in our quota, ymmv in other locations
# https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/overview#azure-regions
# https://portal.azure.com/#view/Microsoft_Azure_Capacity/QuotaMenuBlade/~/myQuotas
# location = "northeurope" # ireland
## location = "eastus" # virginia
location = "westus2" # california

# this will result in a Teleport cluster name of loadtest.az.teleportdemo.net
cluster_prefix = "loadtest"

# already exists
dns_zone    = "az.teleportdemo.net"
dns_zone_rg = "teleportdemo-dns"

teleport_version = "18.1.8"
deploy_teleport  = true
subscription_id = "060a97ea-3a57-4218-9be5-dba3f19ff2b5"