#!/bin/bash

TEMPLATES_DIR="env/templates"
RESOLVED_DIR="env/resolved"

rm -rf env/resolved/*

install_hcp() {
    echo "hcp not found. Installing hcp CLI..."

    sudo apt-get update && sudo apt-get install wget gpg coreutils
    wget -O- https://apt.releases.hashicorp.com/gpg | sudo gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/hashicorp.list
    sudo apt-get update && sudo apt-get install hcp

    echo "hcp CLI installed successfully."
}

if ! command -v hcp &> /dev/null; then
    install_hcp
fi

hcp auth login
hcp profile init --vault-secrets

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
