#!/bin/bash

# Enable strict mode
set -euo pipefail

# This script only tests templates - it does not update them
# To update templates, use update-version.sh

# Set up variables
CHART_DIR="./charts/headlamp"
TEST_CASES_DIR="${CHART_DIR}/tests/test_cases"
EXPECTED_TEMPLATES_DIR="${CHART_DIR}/tests/expected_templates"

# Print header information
echo "Testing Helm chart templates against expected output..."

# Function to render templates for a specific values file
render_templates() {
    values_file="$1"
    output_dir="$2"
    # Render templates
    helm template headlamp ${CHART_DIR} --values ${values_file} > "${output_dir}/rendered_templates.yaml"
    # Verify the file was created successfully
    if [ ! -s "${output_dir}/rendered_templates.yaml" ]; then
        echo "ERROR: Failed to render templates for ${values_file}"
        exit 1
    fi
}

# Clean up function to handle errors and cleanup
cleanup() {
    # Get exit code
    exit_code=$?

    # Clean up any temporary files/directories
    if [ -d "${CHART_DIR}/tests/defaultvaluetest" ]; then
        rm -rf "${CHART_DIR}/tests/defaultvaluetest"
    fi

    # Clean up test case output directories
    if [ "$(ls -A ${TEST_CASES_DIR} 2>/dev/null)" ]; then
        for values_file in ${TEST_CASES_DIR}/*; do
            case_name=$(basename "${values_file}")
            if [ -d "${CHART_DIR}/tests/${case_name}_output" ]; then
                rm -rf "${CHART_DIR}/tests/${case_name}_output"
            fi
        done
    fi

    # If exiting with error, help user understand what to do
    if [ $exit_code -ne 0 ]; then
        echo ""
        echo "============================================="
        echo "Test failed! To update expected templates to match current output:"
        echo "  1. Review the differences above carefully"
        echo "  2. If the changes are related to version, run:"
        echo "     make helm-update-template-version"
        echo "     This will update ALL expected templates with current Helm version"
        echo "  3. Verify the changes and commit them"
        echo "============================================="
    fi

    exit $exit_code
}

# Register cleanup function
trap cleanup EXIT

# Function to compare rendered templates with expected templates
compare_templates() {
    values_file="$1"
    output_dir="$2"
    expected_file="$3"

    # Compare rendered template with expected template
    if ! diff_output=$(diff -u "${output_dir}/rendered_templates.yaml" "${expected_file}" 2>&1); then
        echo "Template test FAILED for ${values_file} against ${expected_file}:"
        echo "${diff_output}"
        echo "============================================="
        echo "The rendered template does not match the expected template!"
        echo "This could be due to changes in the chart or an outdated expected template."
        echo "If this is an intentional change, update the expected template."
        echo "============================================="
        exit 1
    else
        echo "Template test PASSED for ${values_file} against ${expected_file}"
    fi
}


# Check for default values.yaml test case
mkdir -p "${CHART_DIR}/tests/defaultvaluetest"
render_templates "${CHART_DIR}/values.yaml" ${CHART_DIR}/tests/defaultvaluetest
compare_templates "${CHART_DIR}/values.yaml" ${CHART_DIR}/tests/defaultvaluetest "${EXPECTED_TEMPLATES_DIR}/default.yaml"
# Cleanup is handled by the cleanup function

# Check if TEST_CASES_DIR is not empty
if [ "$(ls -A ${TEST_CASES_DIR})" ]; then
    # Iterate over each test case
    for values_file in ${TEST_CASES_DIR}/*; do
        case_name=$(basename "${values_file}")
        output_dir="${CHART_DIR}/tests/${case_name}_output"
        expected_file="${EXPECTED_TEMPLATES_DIR}/${case_name}"

        # Check if expected template exists for the current test case
        if [ -f "${expected_file}" ]; then
            # Create output directory for the current test case
            mkdir -p "${output_dir}"
            # Render templates for the current test case
            render_templates "${values_file}" "${output_dir}"
            # Compare rendered templates with expected templates for the current test case
            compare_templates "${values_file}" "${output_dir}" "${expected_file}"
            # Cleanup is handled by the cleanup function
        else
            echo "No expected template found for ${values_file}. Skipping template testing."
        fi
    done
else
    echo "No test cases found in ${TEST_CASES_DIR}. Skipping template testing."
fi

echo "Template testing completed."
