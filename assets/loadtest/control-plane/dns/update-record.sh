#!/bin/bash

set -euo pipefail

source vars.env

if [[ "$TELEPORT_BACKEND" == "firestore" ]]; then
  exit 0
fi

action="${1}"

case "$action" in
    CREATE)
        ;;
    UPSERT)
        ;;
    DELETE)
        ;;
    *)
        echo "invalid dns record update action '${action}', expected CREATE|UPSERT|DELETE" >&2
        exit 1
esac

records_json="$STATE_DIR/records.json"

NAMESPACE='teleport'
RELEASE_NAME='teleport'
MYZONE_DNS="${ROUTE53_ZONE}"
MYDNS="${CLUSTER_NAME}.${ROUTE53_ZONE}"
MY_CLUSTER_REGION="${REGION}"
MYZONE="$(aws route53 list-hosted-zones-by-name --dns-name="${MYZONE_DNS?}" | jq -r '.HostedZones[0].Id' | sed s_/hostedzone/__)"
MYELB="$(kubectl --namespace "${NAMESPACE?}" get "service/${RELEASE_NAME?}" -o jsonpath='{.status.loadBalancer.ingress[*].hostname}')"
MYELB_NAME="${MYELB%%-*}"
MYELB_ZONE="$(aws elbv2 describe-load-balancers --region "${MY_CLUSTER_REGION?}" --names "${MYELB_NAME?}" | jq -r '.LoadBalancers[0].CanonicalHostedZoneId')"
jq -n --arg dns "${MYDNS?}" --arg elb "${MYELB?}" --arg elbz "${MYELB_ZONE?}" --arg act "$action" \
    '{
        "Comment": "Change records",
        "Changes": [
          {
            "Action": $act,
            "ResourceRecordSet": {
              "Name": $dns,
              "Type": "A",
              "AliasTarget": {
                "HostedZoneId": $elbz,
                "DNSName": ("dualstack." + $elb),
                "EvaluateTargetHealth": false
              }
            }
          },
          {
            "Action": $act,
            "ResourceRecordSet": {
              "Name": ("*." + $dns),
              "Type": "A",
              "AliasTarget": {
                "HostedZoneId": $elbz,
                "DNSName": ("dualstack." + $elb),
                "EvaluateTargetHealth": false
              }
            }
          }
      ]
    }' > "$records_json"
jq < "$records_json"
CHANGEID="$(aws route53 change-resource-record-sets --hosted-zone-id "${MYZONE?}" --change-batch "file://${records_json}" | jq -r '.ChangeInfo.Id')"
aws route53 get-change --id "${CHANGEID?}" | jq '.ChangeInfo.Status'
