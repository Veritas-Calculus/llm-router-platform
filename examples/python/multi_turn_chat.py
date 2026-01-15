#!/usr/bin/env python3
"""
Multi-turn Conversation Example for LLM Router Platform

This example demonstrates how to have a multi-turn conversation
with the LLM Router Platform, maintaining conversation history.

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
        json={"name": "Multi-turn Chat Example Key"},
        headers=headers,
    )
    response.raise_for_status()
    return response.json()["key"]


class ChatSession:
    """A simple chat session that maintains conversation history."""
    
    def __init__(self, api_key: str, model: str = "llama2", system_prompt: str = None):
        self.api_key = api_key
        self.model = model
        self.messages = []
        
        if system_prompt:
            self.messages.append({"role": "system", "content": system_prompt})
    
    def chat(self, user_message: str) -> str:
        """
        Send a message and get a response, maintaining conversation history.
        
        Args:
            user_message: The user's message
        
        Returns:
            The assistant's response
        """
        # Add user message to history
        self.messages.append({"role": "user", "content": user_message})
        
        # Make API request
        response = requests.post(
            f"{BASE_URL}/api/v1/chat/completions",
            json={
                "model": self.model,
                "messages": self.messages,
                "stream": False,
            },
            headers={
                "Content-Type": "application/json",
                "X-API-Key": self.api_key,
            },
        )
        response.raise_for_status()
        
        # Extract assistant response
        data = response.json()
        assistant_message = data["choices"][0]["message"]["content"]
        
        # Add assistant response to history
        self.messages.append({"role": "assistant", "content": assistant_message})
        
        return assistant_message
    
    def clear_history(self):
        """Clear conversation history, keeping only system prompt if present."""
        system_messages = [m for m in self.messages if m["role"] == "system"]
        self.messages = system_messages


def main():
    print("LLM Router Platform - Multi-turn Conversation Example")
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
    
    # Step 3: Create a chat session
    print("\n[3] Creating chat session...")
    session = ChatSession(
        api_key=api_key,
        model="qwq-32b",  # Use a model available in your LM Studio / Ollama
        system_prompt="You are a helpful math tutor. Be concise and educational.",
    )
    print("    Session created!")
    
    # Step 4: Have a multi-turn conversation
    print("\n[4] Starting multi-turn conversation:")
    print("-" * 50)
    
    try:
        # Turn 1
        print("\nUser: What is the Pythagorean theorem?")
        response = session.chat("What is the Pythagorean theorem?")
        print(f"\nAssistant: {response}")
        
        # Turn 2 - Follow-up question (the model should remember context)
        print("\n" + "-" * 30)
        print("\nUser: Can you give me an example with numbers?")
        response = session.chat("Can you give me an example with numbers?")
        print(f"\nAssistant: {response}")
        
        # Turn 3 - Another follow-up
        print("\n" + "-" * 30)
        print("\nUser: What if one side is 5 and another is 12?")
        response = session.chat("What if one side is 5 and another is 12?")
        print(f"\nAssistant: {response}")
        
        print("\n" + "-" * 50)
        print(f"\nConversation complete! Total messages: {len(session.messages)}")
        
    except requests.exceptions.RequestException as e:
        print(f"\n    Request failed: {e}")
        print("\n    Make sure you have at least one provider configured and active.")


if __name__ == "__main__":
    main()
