#!/usr/bin/env python3
"""Corrects how the generated OpenAPI document describes google.type.Date.

gnostic's protoc-gen-openapi hard-codes google.type.Date to `type: string,
format: date` (see reflector.go, schemaOrReferenceForMessage). Protobuf JSON
serialises it as a `{year, month, day}` object, which is what grpc-gateway
actually sends, so the generated document describes responses the service does
not produce. There is no generator option to turn that mapping off.

Rewriting the affected properties here keeps both halves honest: the protos
keep google.type.Date, which AIP-142 requires and api-linter enforces, and the
published specification describes the real payload. If gnostic ever fixes the
mapping this script becomes a no-op and can be dropped.
"""

from __future__ import annotations

import sys
from pathlib import Path

import yaml

DATE_SCHEMA_NAME = "GoogleTypeDate"
DATE_SCHEMA = {
    "type": "object",
    "description": (
        "A calendar date, as google.type.Date. A year on its own, or a year and "
        "month with a zero day, denotes a whole year or month."
    ),
    "properties": {
        "year": {"type": "integer", "format": "int32", "description": "Year of the date."},
        "month": {"type": "integer", "format": "int32", "description": "Month of the year, 1 to 12."},
        "day": {"type": "integer", "format": "int32", "description": "Day of the month, 1 to 31."},
    },
}


def is_generated_date(node: object) -> bool:
    """Reports whether a schema node is one gnostic produced for a Date field."""
    return isinstance(node, dict) and node.get("type") == "string" and node.get("format") == "date"


def rewrite(node: object) -> int:
    """Replaces every generated Date schema in the tree, returning the count."""
    replaced = 0
    if isinstance(node, dict):
        for key, value in node.items():
            if is_generated_date(value):
                description = value.get("description")
                node[key] = {"allOf": [{"$ref": f"#/components/schemas/{DATE_SCHEMA_NAME}"}]}
                if description:
                    node[key]["description"] = description
                replaced += 1
            else:
                replaced += rewrite(value)
    elif isinstance(node, list):
        for item in node:
            replaced += rewrite(item)
    return replaced


def main() -> int:
    path = Path(sys.argv[1] if len(sys.argv) > 1 else "libs/apiserver/openapi/openapi.yaml")
    document = yaml.safe_load(path.read_text())

    replaced = rewrite(document.get("paths", {})) + rewrite(document.get("components", {}).get("schemas", {}))
    if replaced == 0:
        # Either gnostic changed its mapping or no Date field survives. Either
        # way, silently emitting an unchanged document would hide the change.
        print("fix-openapi-dates: no google.type.Date schemas found; check whether this is still needed", file=sys.stderr)
        return 0

    document.setdefault("components", {}).setdefault("schemas", {})[DATE_SCHEMA_NAME] = DATE_SCHEMA
    header = (
        "# Generated with protoc-gen-openapi, then corrected by\n"
        "# scripts/fix-openapi-dates.py; see that script for why.\n"
    )
    path.write_text(header + yaml.safe_dump(document, sort_keys=False, width=100, allow_unicode=True))
    print(f"fix-openapi-dates: corrected {replaced} google.type.Date schemas")
    return 0


if __name__ == "__main__":
    sys.exit(main())
