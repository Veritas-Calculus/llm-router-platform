# LLM Router Platform - Python Examples

This directory contains Python examples for using the LLM Router Platform.

## Authentication Flow

The LLM Router Platform uses **two types of authentication**:

1. **JWT Token** - Used for dashboard/management APIs (login, create API keys, manage providers)
2. **API Key** - Used for LLM API calls (chat completions, models list)

The examples demonstrate the complete flow:
1. Login with email/password to get a JWT token
2. Use the JWT token to create an API Key
3. Use the API Key for chat completion requests

## Prerequisites

1. Make sure the LLM Router Platform is running:
   ```bash
   docker compose up -d
   ```

2. Install the required dependencies:
   ```bash
   pip install -r requirements.txt
   ```

3. Configure at least one provider in the dashboard (http://localhost:5173)

## Examples

### chat_with_apikey.py (Recommended)

The simplest way - just use your API key directly, no login required!

```bash
export LLM_ROUTER_API_KEY=llm_xxxxxxxxxxxxxxxx
python chat_with_apikey.py
```

### simple_chat.py

A complete example showing the full authentication flow (login → get API key → chat).
Useful for understanding the system or for automation scripts.

```bash
python simple_chat.py
```

### streaming_chat.py

An example demonstrating streaming responses.

```bash
python streaming_chat.py
```

### openai_compatible.py

Shows how to use the OpenAI Python SDK with LLM Router as a drop-in replacement.

```bash
python openai_compatible.py
```

### multi_turn_chat.py

Demonstrates multi-turn conversations with conversation history.

```bash
python multi_turn_chat.py
```

## Configuration

You can set the following environment variables:

- `LLM_ROUTER_URL`: The base URL of your LLM Router instance (default: `http://localhost:8080`)
- `LLM_ROUTER_EMAIL`: Your login email (default: `admin@example.com`)
- `LLM_ROUTER_PASSWORD`: Your login password (default: `admin123`)

Example:
```bash
export LLM_ROUTER_URL=http://localhost:8080
export LLM_ROUTER_EMAIL=admin@example.com
export LLM_ROUTER_PASSWORD=admin123
python simple_chat.py
```

## API Key Usage

Once you have an API key, you can use it directly without logging in each time:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/api/v1",
    api_key="your-api-key-here",  # API Key, not JWT token!
)

response = client.chat.completions.create(
    model="llama2",
    messages=[{"role": "user", "content": "Hello!"}],
)
print(response.choices[0].message.content)
```

Or with requests:

```python
import requests

response = requests.post(
    "http://localhost:8080/api/v1/chat/completions",
    json={"model": "llama2", "messages": [{"role": "user", "content": "Hello!"}]},
    headers={"X-API-Key": "your-api-key-here"},
)
print(response.json()["choices"][0]["message"]["content"])
```
