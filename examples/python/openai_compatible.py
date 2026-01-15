#!/usr/bin/env python3
"""
OpenAI SDK Compatible Example for LLM Router Platform

This example demonstrates how to use the official OpenAI Python SDK
with LLM Router as a drop-in replacement. This is useful if you have
existing code that uses the OpenAI SDK and want to switch to LLM Router.

Authentication Flow:
1. Login with email/password to get JWT token
2. Use JWT token to get or create an API Key
3. Use API Key for chat completion requests
"""

import os
import requests
from openai import OpenAI

# Configuration
BASE_URL = os.getenv("LLM_ROUTER_URL", "http://localhost:8080")
EMAIL = os.getenv("LLM_ROUTER_EMAIL", "admin@example.com")
PASSWORD = os.getenv("LLM_ROUTER_PASSWORD", "admin123")


def get_auth_token() -> str:
    """Login and get JWT authentication token."""
    response = requests.post(
        f"{BASE_URL}/api/v1/auth/login",
        json={"email": EMAIL, "password": PASSWORD},
        headers={"Content-Type": "application/json"},
    )
    response.raise_for_status()
    return response.json()["token"]


def get_or_create_api_key(jwt_token: str) -> str:
    """Get existing API key or create a new one."""
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {jwt_token}",
    }
    response = requests.post(
        f"{BASE_URL}/api/v1/api-keys",
        json={"name": "OpenAI SDK Example Key"},
        headers=headers,
    )
    response.raise_for_status()
    return response.json()["key"]


def main():
    print("LLM Router Platform - OpenAI SDK Compatible Example")
    print("=" * 50)
    
    # Step 1: Get JWT token
    print("\n[1] Logging in...")
    try:
        jwt_token = get_auth_token()
        print("    Login successful!")
    except requests.exceptions.RequestException as e:
        print(f"    Login failed: {e}")
        print("\n    Make sure the LLM Router Platform is running:")
        print("    docker compose up -d")
        return
    
    # Step 2: Get API key
    print("\n[2] Getting API key...")
    try:
        api_key = get_or_create_api_key(jwt_token)
        print(f"    API key obtained: {api_key[:20]}...")
    except requests.exceptions.RequestException as e:
        print(f"    Failed to get API key: {e}")
        return
    
    # Step 3: Create OpenAI client pointing to LLM Router
    print("\n[3] Creating OpenAI client with LLM Router backend...")
    
    client = OpenAI(
        base_url=f"{BASE_URL}/api/v1",
        api_key=api_key,  # Use the API key (not JWT token!)
    )
    
    print("    Client created!")
    
    # Step 4: Make a chat completion request using OpenAI SDK
    print("\n[4] Sending chat request via OpenAI SDK...")
    
    try:
        # Non-streaming request
        # Use a model available in your LM Studio / Ollama instance
        response = client.chat.completions.create(
            model="qwq-32b",
            messages=[
                {"role": "system", "content": "You are a helpful assistant."},
                {"role": "user", "content": "What is 2 + 2? Answer briefly."},
            ],
        )
        
        print("\n[5] Response received:")
        print("-" * 50)
        print(response.choices[0].message.content)
        print("-" * 50)
        
        if response.usage:
            print(f"\nToken usage:")
            print(f"  - Prompt tokens: {response.usage.prompt_tokens}")
            print(f"  - Completion tokens: {response.usage.completion_tokens}")
            print(f"  - Total tokens: {response.usage.total_tokens}")
        
    except Exception as e:
        print(f"    Request failed: {e}")
        print("\n    Make sure you have at least one provider configured and active.")
        return
    
    # Step 6: Demonstrate streaming with OpenAI SDK
    print("\n[6] Sending streaming request via OpenAI SDK...")
    print("-" * 50)
    
    try:
        stream = client.chat.completions.create(
            model="qwq-32b",
            messages=[
                {"role": "user", "content": "Count from 1 to 5."},
            ],
            stream=True,
        )
        
        for chunk in stream:
            if chunk.choices[0].delta.content:
                print(chunk.choices[0].delta.content, end="", flush=True)
        
        print("\n" + "-" * 50)
        print("\nStreaming complete!")
        
    except Exception as e:
        print(f"\n    Streaming request failed: {e}")


if __name__ == "__main__":
    main()
