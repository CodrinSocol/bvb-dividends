#!/bin/bash

TEMPLATES_DIR="env/templates"
RESOLVED_DIR="env/resolved"

rm -rf env/resolved/*

for template_file in "${TEMPLATES_DIR}"/*; do
    base_template_file_name=$(basename "${template_file}")
    resolved_file_name="${RESOLVED_DIR}"/"${base_template_file_name}"

    touch "${resolved_file_name}"

    while IFS='=' read -r key value; do
        if [ -z "$key" ] || [ -z "$value" ]; then
            continue
        fi

        secret_value=$(hcp vault-secrets secrets open "$value" --format=json | jq -r '.static_version.value')

        echo "${key}=${secret_value}" >> "${resolved_file_name}"
    done < "${template_file}"
done
