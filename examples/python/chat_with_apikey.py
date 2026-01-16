#!/usr/bin/env python3
"""
Direct API Key Usage Example for LLM Router Platform

This is the simplest way to use LLM Router - just use your API key directly!
No login required.

Get your API key from the dashboard: http://localhost:5173/settings

Usage:
  python chat_with_apikey.py                           # Interactive mode
  python chat_with_apikey.py --provider google         # Use Google provider
  python chat_with_apikey.py --model gemini-2.0-flash  # Use specific model
  python chat_with_apikey.py --list                    # List available providers and models
"""

import argparse
import os
import requests
import sys
from typing import Dict, List, Optional, Tuple

# Configuration
BASE_URL = os.getenv("LLM_ROUTER_URL", "http://localhost:8080")
API_KEY = os.getenv("LLM_ROUTER_API_KEY", "your-api-key-here")


def get_headers():
    """Return headers with API key."""
    return {
        "Content-Type": "application/json",
        "X-API-Key": API_KEY,
    }


def fetch_providers() -> List[Dict]:
    """Fetch available providers from LLM Router."""
    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/models/providers",
            headers=get_headers(),
            timeout=15,  # Server has 3s timeout per provider, so this should be enough
        )
        response.raise_for_status()
        data = response.json()
        return data.get("data", [])
    except requests.exceptions.RequestException as e:
        print(f"❌ Failed to fetch providers: {e}")
        return []


def fetch_models() -> List[Dict]:
    """Fetch available models from LLM Router."""
    try:
        response = requests.get(
            f"{BASE_URL}/api/v1/models",
            headers=get_headers(),
            timeout=15,  # Server has 3s timeout per provider, so this should be enough
        )
        response.raise_for_status()
        data = response.json()
        return data.get("data", [])
    except requests.exceptions.RequestException as e:
        print(f"❌ Failed to fetch models: {e}")
        return []


def chat(messages: list, model: str) -> dict:
    """Send a chat completion request."""
    response = requests.post(
        f"{BASE_URL}/api/v1/chat/completions",
        json={
            "model": model,
            "messages": messages,
        },
        headers=get_headers(),
        timeout=120,
    )
    response.raise_for_status()
    return response.json()


def list_providers():
    """List all available providers and models from LLM Router."""
    print("\n📋 Fetching providers from LLM Router...\n")
    
    providers = fetch_providers()
    if not providers:
        print("No providers found. Make sure providers are configured in LLM Router.")
        return
    
    print("Available Providers and Models:\n")
    for p in providers:
        status = "✅" if p.get("is_active") else "❌"
        print(f"  {status} {p['name']}")
        models = p.get("models", [])
        if models:
            for model in models:
                print(f"      - {model}")
        else:
            print("      (no models configured)")
        print()


def select_provider_interactive(providers: List[Dict]) -> Tuple[str, str]:
    """Interactively select provider and model."""
    # Filter active providers
    active_providers = [p for p in providers if p.get("is_active")]
    if not active_providers:
        print("❌ No active providers available.")
        sys.exit(1)
    
    print("\n🔧 Select Provider:\n")
    for i, p in enumerate(active_providers, 1):
        models_count = len(p.get("models", []))
        print(f"  {i}. {p['name']} ({models_count} models)")
    
    while True:
        try:
            choice = input(f"\nEnter number (1-{len(active_providers)}): ")
            idx = int(choice) - 1
            if 0 <= idx < len(active_providers):
                selected_provider = active_providers[idx]
                break
            print("Invalid choice, try again.")
        except ValueError:
            print("Please enter a number.")
    
    models = selected_provider.get("models", [])
    if not models:
        print(f"❌ No models available for {selected_provider['name']}")
        sys.exit(1)
    
    print(f"\n🎯 Select Model for {selected_provider['name']}:\n")
    for i, model in enumerate(models, 1):
        print(f"  {i}. {model}")
    
    while True:
        try:
            choice = input(f"\nEnter number (1-{len(models)}) or press Enter for first: ")
            if choice == "":
                return selected_provider["name"], models[0]
            idx = int(choice) - 1
            if 0 <= idx < len(models):
                return selected_provider["name"], models[idx]
            print("Invalid choice, try again.")
        except ValueError:
            print("Please enter a number.")


def find_model_provider(providers: List[Dict], model_name: str) -> Optional[str]:
    """Find which provider a model belongs to."""
    for p in providers:
        if model_name in p.get("models", []):
            return p["name"]
    return None


def main():
    parser = argparse.ArgumentParser(
        description="Chat with LLM Router using different providers and models"
    )
    parser.add_argument(
        "--provider", "-p",
        help="Provider to use (e.g., google, openai, anthropic)"
    )
    parser.add_argument(
        "--model", "-m",
        help="Model to use (e.g., gemini-2.0-flash, gpt-4o-mini)"
    )
    parser.add_argument(
        "--message", "-msg",
        default="Hello! Tell me a fun fact in one sentence.",
        help="Message to send"
    )
    parser.add_argument(
        "--list", "-l",
        action="store_true",
        help="List available providers and models"
    )
    parser.add_argument(
        "--interactive", "-i",
        action="store_true",
        help="Interactive mode to select provider and model"
    )
    
    args = parser.parse_args()
    
    if API_KEY == "your-api-key-here":
        print("⚠️  Please set your API key!")
        print()
        print("Option 1: Set environment variable")
        print("  export LLM_ROUTER_API_KEY=llm_xxxxxxxxxxxxxxxx")
        print()
        print("Option 2: Edit this file and replace 'your-api-key-here'")
        print()
        print("Get your API key from: http://localhost:5173/settings")
        return
    
    if args.list:
        list_providers()
        return

    # Fetch providers for selection
    providers = fetch_providers()
    if not providers:
        print("❌ Cannot proceed without providers.")
        return

    # Determine model to use
    if args.interactive:
        provider_name, model = select_provider_interactive(providers)
    elif args.model:
        model = args.model
        provider_name = find_model_provider(providers, model)
        if not provider_name:
            provider_name = "Unknown"
    elif args.provider:
        # Find the provider and use its first model
        provider_data = next((p for p in providers if p["name"] == args.provider), None)
        if not provider_data:
            print(f"❌ Provider '{args.provider}' not found.")
            print("Available providers:", ", ".join(p["name"] for p in providers))
            return
        models = provider_data.get("models", [])
        if not models:
            print(f"❌ No models configured for provider '{args.provider}'")
            return
        model = models[0]
        provider_name = args.provider
    else:
        # Default: use first active provider's first model
        active = [p for p in providers if p.get("is_active") and p.get("models")]
        if not active:
            print("❌ No active providers with models available.")
            return
        provider_name = active[0]["name"]
        model = active[0]["models"][0]

    print(f"\n🚀 Sending request to {provider_name}...")
    print(f"   Model: {model}")
    print(f"   Message: {args.message[:50]}{'...' if len(args.message) > 50 else ''}")
    print()
    
    try:
        response = chat([
            {"role": "user", "content": args.message}
        ], model=model)
        
        content = response["choices"][0]["message"]["content"]
        print("💬 Response:")
        print("-" * 40)
        print(content)
        print("-" * 40)
        
        if "usage" in response:
            usage = response["usage"]
            print(f"\n📊 Tokens: {usage.get('prompt_tokens', 0)} prompt + {usage.get('completion_tokens', 0)} completion = {usage.get('total_tokens', 0)} total")
            
    except requests.exceptions.HTTPError as e:
        print(f"❌ HTTP Error: {e}")
        if e.response is not None:
            print(f"   Response: {e.response.text}")
    except requests.exceptions.ConnectionError:
        print(f"❌ Connection Error: Cannot connect to {BASE_URL}")
        print("   Make sure the LLM Router server is running.")
    except Exception as e:
        print(f"❌ Error: {e}")


if __name__ == "__main__":
    main()
