#!/usr/bin/env python3
"""
Direct API Key Usage Example for LLM Router Platform

This is the simplest way to use LLM Router - just use your API key directly!
No login required.

Get your API key from the dashboard: http://localhost:5173/settings
"""

import os
import requests

# Configuration
BASE_URL = os.getenv("LLM_ROUTER_URL", "http://localhost:8080")
API_KEY = os.getenv("LLM_ROUTER_API_KEY", "your-api-key-here")


def chat(messages: list, model: str = "qwq-32b") -> dict:
    """Send a chat completion request."""
    response = requests.post(
        f"{BASE_URL}/api/v1/chat/completions",
        json={
            "model": model,
            "messages": messages,
        },
        headers={
            "Content-Type": "application/json",
            "X-API-Key": API_KEY,
        },
    )
    response.raise_for_status()
    return response.json()


def main():
    if API_KEY == "your-api-key-here":
        print("Please set your API key!")
        print()
        print("Option 1: Set environment variable")
        print("  export LLM_ROUTER_API_KEY=llm_xxxxxxxxxxxxxxxx")
        print()
        print("Option 2: Edit this file and replace 'your-api-key-here'")
        print()
        print("Get your API key from: http://localhost:5173/settings")
        return

    print("Sending chat request...")
    
    response = chat([
        {"role": "user", "content": "Hello! Say hi in one sentence."}
    ])
    
    print(response["choices"][0]["message"]["content"])


if __name__ == "__main__":
    main()
