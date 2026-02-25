#!/bin/bash

# Enable strict mode
set -euo pipefail

# Set up variables
CHART_DIR="./charts/headlamp"
TEST_CASES_DIR="${CHART_DIR}/tests/test_cases"
EXPECTED_TEMPLATES_DIR="${CHART_DIR}/tests/expected_templates"

# Get the current chart, app and image version
CURRENT_CHART_VERSION=$(grep '^version:' ${CHART_DIR}/Chart.yaml | awk '{print $2}')
CURRENT_APP_VERSION=$(grep '^appVersion:' ${CHART_DIR}/Chart.yaml | awk '{print $2}')
CURRENT_IMAGE_VERSION=$(grep '^appVersion:' ${CHART_DIR}/Chart.yaml | awk '{print $2}')

echo "Checking and updating template versions..."
echo "Using chart version: ${CURRENT_CHART_VERSION}, app version: ${CURRENT_APP_VERSION}, image version: ${CURRENT_IMAGE_VERSION}"

# Function to render templates for a specific values file
render_templates() {
    values_file="$1"
    output_dir="$2"
    # Render templates
    helm template headlamp ${CHART_DIR} --values ${values_file} > "${output_dir}/rendered_templates.yaml"
    if [ ! -s "${output_dir}/rendered_templates.yaml" ]; then
        echo "ERROR: Failed to render templates for ${values_file}"
        exit 1
    fi
}

# Check if versions need updating and update the expected template if needed
check_and_update_template() {
    case_name="$1"
    values_file="$2"
    
    expected_file="${EXPECTED_TEMPLATES_DIR}/${case_name}"
    needs_update=false
    
    # Check if versions need updating
    if [ -f "${expected_file}" ]; then
        # Check chart version
        if ! grep -q "helm.sh/chart: headlamp-${CURRENT_CHART_VERSION}" "${expected_file}"; then
            needs_update=true
        fi
        
        # Check app version
        if ! grep -q "app.kubernetes.io/version: \"${CURRENT_APP_VERSION}\"" "${expected_file}"; then
            needs_update=true
        fi
        
        # Check image version
        if ! grep -q "ghcr.io/headlamp-k8s/headlamp:v${CURRENT_IMAGE_VERSION}" "${expected_file}"; then
            needs_update=true
        fi
    else
        # File doesn't exist, so it needs to be created
        needs_update=true
    fi
    
    if [ "$needs_update" = true ]; then
        echo "${case_name}: Updating to version ${CURRENT_CHART_VERSION}..."
        
        # Create temporary output directory
        output_dir="${CHART_DIR}/tests/update_${case_name}"
        mkdir -p "${output_dir}"
        
        # Render the template
        render_templates "${values_file}" "${output_dir}"
        
        # Update the expected template
        cp "${output_dir}/rendered_templates.yaml" "${expected_file}"
        
        # Clean up
        rm -rf "${output_dir}"
    else
        echo "${case_name}: Version ${CURRENT_CHART_VERSION} already up to date"  
    fi
}

# Check and update default template
check_and_update_template "default.yaml" "${CHART_DIR}/values.yaml"

# Check and update templates for each test case
if [ "$(ls -A ${TEST_CASES_DIR})" ]; then
    for values_file in ${TEST_CASES_DIR}/*; do
        case_name=$(basename "${values_file}")
        check_and_update_template "${case_name}" "${values_file}"
    done
else
    echo "No test cases found in ${TEST_CASES_DIR}."
fi

echo "Version check complete. Please review any changes before committing."
