#!/usr/bin/env python3
"""
Convert template pack from object format (v0.13) to array format (v0.14+).

The new format expects:
- object_type_schemas: array of {name, label, description, properties}
- relationship_type_schemas: array of {name, label, description, sourceTypes, targetTypes}

Old format had these as objects with type names as keys.
"""

import json
import sys
from pathlib import Path


def convert_object_schemas(old_schemas):
    """Convert object_type_schemas from dict to array format."""
    if not old_schemas:
        return []
    
    result = []
    for name, schema in old_schemas.items():
        new_schema = {
            "name": name,
        }
        
        # Extract label, description, properties
        if "label" in schema:
            new_schema["label"] = schema["label"]
        if "description" in schema:
            new_schema["description"] = schema["description"]
        if "properties" in schema:
            new_schema["properties"] = schema["properties"]
        
        result.append(new_schema)
    
    return result


def convert_relationship_schemas(old_schemas):
    """Convert relationship_type_schemas from dict to array format."""
    if not old_schemas:
        return []
    
    result = []
    for name, schema in old_schemas.items():
        new_schema = {
            "name": name,
        }
        
        # Extract all fields
        if "label" in schema:
            new_schema["label"] = schema["label"]
        if "description" in schema:
            new_schema["description"] = schema["description"]
        if "sourceTypes" in schema:
            new_schema["sourceTypes"] = schema["sourceTypes"]
        if "targetTypes" in schema:
            new_schema["targetTypes"] = schema["targetTypes"]
        
        result.append(new_schema)
    
    return result


def convert_pack(input_path, output_path):
    """Convert a template pack file to new format."""
    print(f"Reading {input_path}...")
    with open(input_path, 'r') as f:
        pack = json.load(f)
    
    print(f"Converting object_type_schemas...")
    old_obj_schemas = pack.get("object_type_schemas", {})
    if isinstance(old_obj_schemas, dict):
        pack["object_type_schemas"] = convert_object_schemas(old_obj_schemas)
        print(f"  Converted {len(pack['object_type_schemas'])} object types")
    else:
        print(f"  Already in array format, skipping")
    
    print(f"Converting relationship_type_schemas...")
    old_rel_schemas = pack.get("relationship_type_schemas", {})
    if isinstance(old_rel_schemas, dict):
        pack["relationship_type_schemas"] = convert_relationship_schemas(old_rel_schemas)
        print(f"  Converted {len(pack['relationship_type_schemas'])} relationship types")
    else:
        print(f"  Already in array format, skipping")
    
    print(f"Writing {output_path}...")
    with open(output_path, 'w') as f:
        json.dump(pack, f, indent=2)
    
    print(f"✓ Conversion complete")
    return pack


def main():
    if len(sys.argv) < 2:
        print("Usage: convert-pack-format.py <input.json> [output.json]")
        sys.exit(1)
    
    input_path = Path(sys.argv[1])
    if len(sys.argv) > 2:
        output_path = Path(sys.argv[2])
    else:
        output_path = input_path.parent / f"{input_path.stem}-converted.json"
    
    if not input_path.exists():
        print(f"Error: {input_path} not found")
        sys.exit(1)
    
    converted = convert_pack(input_path, output_path)
    
    # Validation
    print("\nValidation:")
    print(f"  Name: {converted['name']}")
    print(f"  Version: {converted['version']}")
    print(f"  Object types: {len(converted['object_type_schemas'])}")
    print(f"  Relationship types: {len(converted['relationship_type_schemas'])}")


if __name__ == "__main__":
    main()
