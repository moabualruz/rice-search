#!/usr/bin/env python3
"""
Initialize a new store in the Local Code Search platform.

Usage:
    python init_store.py my-store [--description "Description"] [--api-url http://localhost:8080]
"""

import argparse
import json
import sys
import urllib.request
import urllib.error


def create_store(api_url: str, name: str, description: str = "") -> dict:
    """Create a new store via API."""
    url = f"{api_url}/v1/stores"
    data = json.dumps({"name": name, "description": description}).encode("utf-8")

    req = urllib.request.Request(
        url, data=data, headers={"Content-Type": "application/json"}, method="POST"
    )

    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8") if e.fp else str(e)
        raise RuntimeError(f"HTTP {e.code}: {error_body}")


def get_store(api_url: str, name: str) -> dict | None:
    """Get store details."""
    url = f"{api_url}/v1/stores/{name}"

    try:
        with urllib.request.urlopen(url, timeout=10) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return None
        raise


def list_stores(api_url: str) -> list:
    """List all stores."""
    url = f"{api_url}/v1/stores"

    with urllib.request.urlopen(url, timeout=10) as response:
        return json.loads(response.read().decode("utf-8"))


def main():
    parser = argparse.ArgumentParser(
        description="Initialize a store in Local Code Search"
    )
    parser.add_argument("name", nargs="?", help="Store name to create")
    parser.add_argument("--description", "-d", default="", help="Store description")
    parser.add_argument("--api-url", default="http://localhost:8080", help="API URL")
    parser.add_argument(
        "--list", "-l", action="store_true", help="List existing stores"
    )

    args = parser.parse_args()

    if args.list:
        print("Existing stores:")
        try:
            stores = list_stores(args.api_url)
            if not stores:
                print("  (none)")
            for store in stores:
                print(f"  - {store}")
        except Exception as e:
            print(f"Error listing stores: {e}", file=sys.stderr)
            sys.exit(1)
        return

    if not args.name:
        parser.print_help()
        sys.exit(1)

    print(f"Creating store: {args.name}")
    print(f"API: {args.api_url}")

    try:
        # Check if store exists
        existing = get_store(args.api_url, args.name)
        if existing:
            print(f"Store '{args.name}' already exists.")
            return

        # Create store
        result = create_store(args.api_url, args.name, args.description)
        print(f"Store created successfully!")
        print(f"  Name: {result.get('name', args.name)}")
        if args.description:
            print(f"  Description: {args.description}")
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
