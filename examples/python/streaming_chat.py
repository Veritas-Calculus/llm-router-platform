#!/usr/bin/env python3
"""
Streaming Chat Example for LLM Router Platform

This example demonstrates how to make a streaming chat completion request
to the LLM Router Platform, receiving tokens as they are generated.

Authentication Flow:
1. Login with email/password to get JWT token
2. Use JWT token to get or create an API Key
3. Use API Key for chat completion requests
"""

import os
import json
import requests

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
    """
    Get existing API key or create a new one.
    
    Note: Once created, API keys cannot be retrieved in full (only prefix is stored).
    This function checks for a cached key file first, then creates a new one if needed.
    """
    # Cache file to store the API key for reuse
    cache_file = os.path.join(os.path.dirname(__file__), ".api_key_cache")
    
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {jwt_token}",
    }
    
    # Try to load cached API key
    if os.path.exists(cache_file):
        with open(cache_file, "r") as f:
            cached_key = f.read().strip()
            if cached_key:
                # Verify the key is still valid by making a test request
                try:
                    test_response = requests.post(
                        f"{BASE_URL}/api/v1/chat/completions",
                        json={"model": "test", "messages": [{"role": "user", "content": "test"}]},
                        headers={"Content-Type": "application/json", "X-API-Key": cached_key},
                        timeout=5,
                    )
                    # If we get 401/403, key is invalid; any other response means key is valid
                    if test_response.status_code not in [401, 403]:
                        return cached_key
                except requests.exceptions.RequestException:
                    pass  # Key might still be valid, network issue
    
    # Create a new API key
    response = requests.post(
        f"{BASE_URL}/api/v1/api-keys",
        json={"name": "Streaming Example Key"},
        headers=headers,
    )
    response.raise_for_status()
    new_key = response.json()["key"]
    
    # Cache the key for future use
    with open(cache_file, "w") as f:
        f.write(new_key)
    
    return new_key


def stream_chat_completion(api_key: str, messages: list, model: str = "gpt-4"):
    """
    Send a streaming chat completion request to LLM Router.
    
    Args:
        api_key: User API key (not JWT token!)
        messages: List of message dicts with 'role' and 'content'
        model: Model name (e.g., 'gpt-4', 'claude-3-opus', 'llama2')
    
    Yields:
        Content chunks as they are received
    """
    response = requests.post(
        f"{BASE_URL}/api/v1/chat/completions",
        json={
            "model": model,
            "messages": messages,
            "stream": True,
        },
        headers={
            "Content-Type": "application/json",
            "X-API-Key": api_key,
        },
        stream=True,
        timeout=120,  # Increase timeout for long responses
    )
    response.raise_for_status()
    
    try:
        for line in response.iter_lines():
            if line:
                line = line.decode("utf-8")
                if line.startswith("data: "):
                    data = line[6:]  # Remove "data: " prefix
                    if data == "[DONE]":
                        return  # Normal end of stream
                    try:
                        chunk = json.loads(data)
                        if "choices" in chunk and len(chunk["choices"]) > 0:
                            delta = chunk["choices"][0].get("delta", {})
                            content = delta.get("content", "")
                            if content:
                                yield content
                    except json.JSONDecodeError:
                        continue
    except requests.exceptions.ChunkedEncodingError:
        # Stream ended, this is normal for some servers
        pass


def main():
    print("LLM Router Platform - Streaming Chat Example")
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
    
    # Step 3: Send a streaming chat completion request
    print("\n[3] Sending streaming chat request...")
    
    messages = [
        {"role": "system", "content": "You are a helpful assistant."},
        {"role": "user", "content": "Write a short poem about programming."},
    ]
    
    print("\n[4] Streaming response:")
    print("-" * 50)
    
    try:
        # Use a model available in your LM Studio / Ollama instance
        # Run: curl http://localhost:8080/api/v1/models -H "X-API-Key: YOUR_KEY" to see available models
        for chunk in stream_chat_completion(api_key, messages, model="qwq-32b"):
            print(chunk, end="", flush=True)
        
        print("\n" + "-" * 50)
        print("\nStreaming complete!")
        
    except requests.exceptions.RequestException as e:
        print(f"\n    Request failed: {e}")
        print("\n    Make sure you have at least one provider configured and active.")


if __name__ == "__main__":
    main()
