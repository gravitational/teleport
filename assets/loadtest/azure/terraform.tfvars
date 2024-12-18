# these locations should support HA zone-redundant Postgres and have ~200 DSv3
# cpus in our quota, ymmv in other locations
# https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/overview#azure-regions
# https://portal.azure.com/#view/Microsoft_Azure_Capacity/QuotaMenuBlade/~/myQuotas
location = "northeurope" # ireland
## location = "eastus" # virginia
## location = "westus" # california

# this will result in a Teleport cluster name of loadtest.az.teleportdemo.net
cluster_prefix = "loadtest"

# already exists
dns_zone    = "az.teleportdemo.net"
dns_zone_rg = "teleportdemo-dns"

teleport_version = "17.0.0-alpha.2"
deploy_teleport  = true
