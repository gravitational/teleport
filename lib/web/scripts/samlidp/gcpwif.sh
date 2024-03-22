#!/usr/bin/env bash
set -euo pipefail

ORGANIZATION_ID="{{.OrgID}}"
PROVIDER_ID="{{.PoolProviderName}}"
WORKFORCE_POOL_ID="{{.PoolName}}"
DEFAULT_SESSION_DURATION="3600s"
DESCRIPTION="Workforce pool created by Teleport"
TELEPORT_PROXY_URL="{{.MetadataEndpoint}}"


function create_pool() {
gcloud iam workforce-pools create $WORKFORCE_POOL_ID \
    --display-name=$WORKFORCE_POOL_ID \
    --organization=$ORGANIZATION_ID \
    --description="$DESCRIPTION" \
    --session-duration=$DEFAULT_SESSION_DURATION \
    --location=global
}

ATTRIBUTE_MAPPING="google.subject=assertion.subject,google.groups=assertion.attributes.roles"
DESCRIPTION2="Teleport Workforce pool identity provider"
ATTRIBUTE_CONDITION=""

WORKING_DIR=$(pwd)

METADATA_FILE_NAME="teleport-saml-idp-metadata.xml"
echo "Downloading Teleport SAML IdP Metadata."
curl "$TELEPORT_PROXY_URL" -o $METADATA_FILE_NAME
echo "Done."

function create_pool_provider() {
gcloud iam workforce-pools providers create-saml $PROVIDER_ID \
    --workforce-pool=$WORKFORCE_POOL_ID \
    --display-name=$PROVIDER_ID \
    --description="$DESCRIPTION2" \
    --idp-metadata-path="$WORKING_DIR/$METADATA_FILE_NAME" \
    --attribute-mapping=$ATTRIBUTE_MAPPING \
    --attribute-condition="$ATTRIBUTE_CONDITION" \
    --location=global
}


echo "Creating Workforce Identity Pool."
create_pool
echo "Done."
echo

echo "Creating Workforce Identity Pool Provider."
create_pool_provider
echo "Done."