#!/bin/bash

# If someone runs a debug GitHub Actions task
if [ -n "$RUNNER_DEBUG" ]; then
  set -x
fi

set -euo pipefail

info() {
    printf "\e[1;32m%s\e[0m\n" "$1"
}
error() {
    printf "\e[1;31m%s\e[0m\n" "$1"
    exit 1
}
# convert_tf_docs_comment converts comments like <!-- BEGIN_TF_DOCS --> to an mdx comment like {/* BEGIN_TF_DOCS */}
convert_tf_docs_comment() {
  sed 's#<!-- \(.*\) -->#{/\* \1 \*/}#'
}

check_file() {
    local file="$1"
    if [[ ! -f "${file}" ]]; then
        error "${file} is missing"
    fi
    if grep -q -- "TODO" "${file}"; then
        error "${file} contains TODO"
    fi
}

VERSION="$1"; shift

if [ -z "$VERSION" ]; then
  error "Version arg is required"
fi

PUBLISHED_TF_MODULES=( "$@" )

if [[ ${#PUBLISHED_TF_MODULES[@]} -eq 0 ]]; then
  error "Published modules args are required"
fi

DOCSDIR="$(pwd)/../../docs/pages/reference/infrastructure-as-code/terraform-modules"
TMPDIR="$(mktemp -d)"
MODULES_DOC_INDEX="${TMPDIR}/terraform-modules.mdx"
MODULES_ROOT_DIR="$(git rev-parse --show-prefix)" # the relative path from the git repo to the modules dir, e.g., "integrations/terraform-modules" at time of writing.
MODULES_ROOT_DIR="${MODULES_ROOT_DIR%%/}" # trim trailing slash
SOURCE_URI="github.com/gravitational/teleport/tree/master/${MODULES_ROOT_DIR}"

info "Rendering modules reference index"
cat <<EOF > "${MODULES_DOC_INDEX}"
---
title: Teleport Terraform Modules Reference
sidebar_label: Terraform Modules
description: Reference documentation for the Teleport Terraform modules.
tags:
 - infrastructure-as-code
 - reference
 - platform-wide
---

{/*
    Auto-generated file.
    Do not edit this directly in the docs/pages tree.
    Instead, edit ${MODULES_ROOT_DIR}/gen/docs.sh
    Then, regenerate the docs with \`make -C ${MODULES_ROOT_DIR} docs\`.
*/}

Teleport publishes the following Terraform modules:

EOF

for module in "${PUBLISHED_TF_MODULES[@]}"; do
    module_name="${module//\//-}"

    # inject a link to the module into the modules index page
    info "Adding ${module_name} to modules reference index"
    cat <<EOF >> "${MODULES_DOC_INDEX}"
- [\`${module_name}\`](./${module_name}/${module_name}.mdx)
EOF

    info "Rendering module ${module_name} reference doc"
    module_docs_dir=${TMPDIR}/${module_name}
    mkdir -p "${module_docs_dir}"
    module_index_doc="${module_docs_dir}/${module_name}.mdx"

    remote_system="${module_name##*-}" # trim everything up to the last dash, which is the "system" component, e.g., "aws".
    case "${remote_system}" in
      azure) remote_system="Azure" ;;
      *)     remote_system=$(echo "${remote_system}" | tr '[:lower:]' '[:upper:]') ;; # uppercase it, e.g., "aws" -> "AWS".
    esac

    # inject a generated docs header
    cat <<EOF > "${module_index_doc}"
---
title: Reference for the ${module_name} Terraform module
sidebar_label: ${module_name}
description: This page describes the ${module_name} Terraform module.
---

{/*
    Auto-generated file.
    Do not edit this directly in the docs/pages tree.
    Instead, edit ${MODULES_ROOT_DIR}/${module}/README.md
    Then, regenerate the docs with \`make -C ${MODULES_ROOT_DIR} docs\`.
*/}

Source Code: [${SOURCE_URI}/${module}](https://${SOURCE_URI}/${module})

EOF
    module_readme="${module}/README.md"
    check_file "${module_readme}"
    convert_tf_docs_comment < "${module_readme}" >> "${module_index_doc}"

    # handle examples
    module_examples_docs_dir="${module_docs_dir}/examples"
    mkdir -p "${module_examples_docs_dir}"
    module_examples_index="${module_examples_docs_dir}/examples.mdx"
    info "Rendering module ${module_name} examples index"
    cat <<EOF > "${module_examples_index}"
---
title: Terraform Module ${module_name} Examples
sidebar_label: examples
description: Index of all the examples for the ${module_name} Terraform module.
---

{/*
    Auto-generated file.
    Do not edit this directly in the docs/pages tree.
    Instead, edit ${MODULES_ROOT_DIR}/gen/docs.sh
    Then, regenerate the docs with \`make -C ${MODULES_ROOT_DIR} docs\`.
*/}

<DocCardList />

EOF
    examples="$(find "${module}/examples" -maxdepth 1 -mindepth 1 -type d)"
    for example in ${examples}; do
        example_name="$(basename "${example}")"
        info "Rendering module ${module_name} example ${example_name} reference doc"
        example_doc="${module_examples_docs_dir}/${example_name}.mdx"
        if [[ ! -f "${example}/docs_title" ]]; then
            error "${example}/docs_title is missing — run gen/example.sh to scaffold"
        fi
        if [[ ! -f "${example}/docs_description" ]]; then
            error "${example}/docs_description is missing — run gen/example.sh to scaffold"
        fi
        example_readme="${example}/README.md"
        check_file "${example_readme}"
        check_file "${example}/docs_title"
        check_file "${example}/docs_description"

        example_title="$(head -1 "${example}/docs_title")"
        example_description="$(head -1 "${example}/docs_description")"

        # inject header
        cat <<EOF > "${example_doc}"
---
title: ${example_title}
sidebar_label: ${example_name}
description: ${example_description}
---

{/*
    Auto-generated file.
    Do not edit this directly in the docs/pages tree.
    Instead, edit ${example}/README.md
    Then, regenerate the docs with \`make -C ${MODULES_ROOT_DIR} docs\`.
*/}

Source Code: [${SOURCE_URI}/${example}](https://${SOURCE_URI}/${example})

EOF
        convert_tf_docs_comment < "${example_readme}" >> "${example_doc}"
    done
done

rm -rf "${DOCSDIR}"
cp -r "${TMPDIR}" "${DOCSDIR}"
