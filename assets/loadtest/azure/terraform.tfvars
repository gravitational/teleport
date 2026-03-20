# these locations should support HA zone-redundant Postgres and have ~200 DSv3
# cpus in our quota, ymmv in other locations
# https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/overview#azure-regions
# https://portal.azure.com/#view/Microsoft_Azure_Capacity/QuotaMenuBlade/~/myQuotas
location = "italynorth" # milan
## location = "eastus" # virginia
## location = "westus" # california

# this will result in a Teleport cluster name of loadtest.az.teleportdemo.net
cluster_prefix = "loadtest"

# already exists
dns_zone    = "az.teleportdemo.net"
dns_zone_rg = "teleportdemo-dns"

teleport_version = "999.0.0-alpha.1"
deploy_teleport  = false

# az account show -o tsv --query id
subscription_id = "12345678-1234-5678-1234-567812345678"
