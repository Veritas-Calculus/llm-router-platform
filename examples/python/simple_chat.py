#!/usr/bin/env python3
"""
Simple Chat Example for LLM Router Platform

This example demonstrates how to make a basic chat completion request
to the LLM Router Platform using the requests library.

Authentication Flow:
1. Login with email/password to get JWT token
2. Use JWT token to get or create an API Key
3. Use API Key for chat completion requests
"""

import os
import requests

# Configuration
BASE_URL = os.getenv("LLM_ROUTER_URL", "http://localhost:8080")
EMAIL = os.getenv("LLM_ROUTER_EMAIL", "admin@example.com")
PASSWORD = os.getenv("LLM_ROUTER_PASSWORD", "admin123")


def get_auth_token() -> str:
    """Login and get JWT authentication token (for dashboard API)."""
    response = requests.post(
        f"{BASE_URL}/api/v1/auth/login",
        json={"email": EMAIL, "password": PASSWORD},
        headers={"Content-Type": "application/json"},
    )
    response.raise_for_status()
    return response.json()["token"]


def get_or_create_api_key(jwt_token: str) -> str:
    """
    Get existing API key or create a new one.
    
    API Keys are used for LLM API calls (chat completions),
    while JWT tokens are used for dashboard/management APIs.
    """
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {jwt_token}",
    }
    
    # First, try to list existing API keys
    response = requests.get(
        f"{BASE_URL}/api/v1/api-keys",
        headers=headers,
    )
    response.raise_for_status()
    keys = response.json().get("api_keys", [])
    
    # If there's an active key, we need to create a new one since we can't retrieve the raw key
    # Create a new API key for this example
    response = requests.post(
        f"{BASE_URL}/api/v1/api-keys",
        json={"name": "Python Example Key"},
        headers=headers,
    )
    response.raise_for_status()
    return response.json()["key"]


def chat_completion(api_key: str, messages: list, model: str = "gpt-4") -> dict:
    """
    Send a chat completion request to LLM Router.
    
    Args:
        api_key: User API key (not JWT token!)
        messages: List of message dicts with 'role' and 'content'
        model: Model name (e.g., 'gpt-4', 'claude-3-opus', 'llama2')
    
    Returns:
        The completion response as a dictionary
    """
    response = requests.post(
        f"{BASE_URL}/api/v1/chat/completions",
        json={
            "model": model,
            "messages": messages,
            "stream": False,
        },
        headers={
            "Content-Type": "application/json",
            "X-API-Key": api_key,  # Use X-API-Key header for LLM API calls
        },
    )
    response.raise_for_status()
    return response.json()


def main():
    print("LLM Router Platform - Simple Chat Example")
    print("=" * 50)
    
    # Step 1: Get JWT token (for dashboard API)
    print("\n[1] Logging in...")
    try:
        jwt_token = get_auth_token()
        print("    Login successful!")
    except requests.exceptions.RequestException as e:
        print(f"    Login failed: {e}")
        print("\n    Make sure the LLM Router Platform is running:")
        print("    docker compose up -d")
        return
    
    # Step 2: Get or create API Key (for LLM API calls)
    print("\n[2] Getting API key...")
    try:
        api_key = get_or_create_api_key(jwt_token)
        print(f"    API key obtained: {api_key[:20]}...")
    except requests.exceptions.RequestException as e:
        print(f"    Failed to get API key: {e}")
        return
    
    # Step 3: Send a chat completion request
    print("\n[3] Sending chat request...")
    
    messages = [
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Hello! Can you briefly introduce yourself?"},
    ]
    
    try:
        # Use a model available in your LM Studio / Ollama instance
        # Run: curl http://localhost:8080/api/v1/models -H "X-API-Key: YOUR_KEY" to see available models
        response = chat_completion(api_key, messages, model="qwq-32b")
        
        print("\n[4] Response received:")
        print("-" * 50)
        
        if "choices" in response and len(response["choices"]) > 0:
            content = response["choices"][0]["message"]["content"]
            print(content)
        else:
            print(response)
            
        print("-" * 50)
        
        # Print usage info if available
        if "usage" in response:
            usage = response["usage"]
            print(f"\nToken usage:")
            print(f"  - Prompt tokens: {usage.get('prompt_tokens', 'N/A')}")
            print(f"  - Completion tokens: {usage.get('completion_tokens', 'N/A')}")
            print(f"  - Total tokens: {usage.get('total_tokens', 'N/A')}")
            
    except requests.exceptions.RequestException as e:
        print(f"    Request failed: {e}")
        print("\n    Make sure you have at least one provider configured and active.")


if __name__ == "__main__":
    main()
