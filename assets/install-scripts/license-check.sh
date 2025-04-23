#!/bin/sh
# Copyright 2024 Gravitational, Inc

# This script is used to validate if a Teleport license is valid or deprecated.
#
# Usage: ./check-deprecated-license <LICENSE_PATH>
# If omited, LICENSE_PATH will be the default Teleport license file path:
# /var/lib/teleport/license.pem

# The script is wrapped inside a function to protect against the connection being interrupted
# in the middle of the stream.
set -eu

print_invalid_license() {
    echo "Error: Outdated License File"
    echo ""
    echo "This Teleport Enterprise cluster is currently using an outdated license file. To resolve this issue, please follow these steps:"
    echo ""
    echo "1. Navigate to https://teleport.sh/ to download your updated license file."
    echo "2. Refer to our documentation at https://goteleport.com/r/license for detailed instructions on updating your license."
    echo "3. If you have any questions or need assistance, please reach out to our support team at support@goteleport.com."
    echo ""
    echo "Thank you for using Teleport."
}

main() {
    if [ ! -f "$LICENSE_PATH" ]; then
        echo "License not found in $LICENSE_PATH. Please pass in a valid license path."
        exit 1
    fi

    # Check if OpenSSL is installed
    if ! command -v openssl >/dev/null 2>&1; then
        echo "Error: openssl is not installed. Install it using your package manager (e.g., 'apt-get install openssl')."
        exit 1
    fi

    START_DATE=$(openssl x509 -startdate -noout -in "$LICENSE_PATH" | sed -e "s/^notBefore=//" | sed 's/ GMT$//')
    END_DATE=$(openssl x509 -enddate -noout -in "$LICENSE_PATH" | sed -e "s/^notAfter=//" | sed 's/ GMT$//')

    if [ "$(uname)" = "Darwin" ]; then
        START_TIMESTAMP=$(date -jf "%b %e %T %Y" "$START_DATE" "+%s")
        END_TIMESTAMP=$(date -jf "%b %e %T %Y" "$END_DATE" "+%s")
        JAN_1_2024_TIMESTAMP=$(date -jf "%b %e %T %Y" "Jan 1 00:00:00 2024" "+%s")
    else
        START_TIMESTAMP=$(date -d "$START_DATE" "+%s")
        END_TIMESTAMP=$(date -d "$END_DATE" "+%s")
        JAN_1_2024_TIMESTAMP=$(date -d "Jan 1 00:00:00 2024" "+%s")
    fi

    # Check if the license is issued before Jan 01 2024
    OLDLICENSE=0
    if [ "$START_TIMESTAMP" -lt "$JAN_1_2024_TIMESTAMP" ]; then
        OLDLICENSE=1
    fi

    # Check license valid period
    FOUR_YEARS_SECONDS=$((((4 * 365) + 1) * 24 * 60 * 60))
    INTERVAL=$((END_TIMESTAMP - START_TIMESTAMP))

    # Check if the license validity exceeds four years and was issued before the cutoff
    if [ "$INTERVAL" -ge "$FOUR_YEARS_SECONDS" ] && [ "$OLDLICENSE" -eq 1 ]; then
        print_invalid_license
        exit 1
    fi

    echo "Your license is valid, no actions are necessary. Thank you for using Teleport."
}

LICENSE_PATH=/var/lib/teleport/license.pem
if [ $# -ge 1 ] && [ -n "$1" ]; then
    LICENSE_PATH=$1
fi

main
