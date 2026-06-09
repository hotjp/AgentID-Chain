#!/usr/bin/env python3
"""register-agent OpenAI Function Calling 示例."""

import json
import openai
import requests


# 1. 注册工具
tools = [
    {
        "type": "function",
        "function": {
            "name": "register_agent",
            "description": "注册新 Agent 到 AgentID-Chain",
            "parameters": {
                "type": "object",
                "properties": {
                    "owner": {
                        "type": "string",
                        "description": "Agent 所有者"
                    },
                    "level": {
                        "type": "string",
                        "enum": ["test", "prod", "internal"],
                        "description": "Agent 等级"
                    },
                    "metadata": {
                        "type": "object",
                        "description": "自定义元数据"
                    }
                },
                "required": ["owner", "level"]
            }
        }
    }
]


def call_mcp(tool_name: str, args: dict, aap_jwt: str) -> dict:
    """调用 MCP 工具."""
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": tool_name,
            "arguments": args
        }
    }
    resp = requests.post(
        "http://localhost:8080/mcp/v1/rpc",
        json=payload,
        headers={"Authorization": f"Bearer {aap_jwt}"}
    )
    return resp.json()


def main():
    aap_jwt = "eyJhbGciOi..."

    # 2. LLM 对话
    response = openai.ChatCompletion.create(
        model="gpt-4",
        messages=[
            {"role": "system", "content": "You are an agent registration assistant."},
            {"role": "user", "content": "为 alice 注册一个 test 等级的 agent"}
        ],
        tools=tools
    )

    # 3. 检查是否需要调用工具
    message = response.choices[0].message
    if message.tool_calls:
        for tool_call in message.tool_calls:
            function_name = tool_call.function.name
            arguments = json.loads(tool_call.function.arguments)

            print(f"LLM 决定调用: {function_name}({arguments})")

            # 4. 执行工具调用
            result = call_mcp(function_name, arguments, aap_jwt)
            print(f"MCP 返回: {json.dumps(result, indent=2)}")

            # 5. 把结果反馈给 LLM
            second_response = openai.ChatCompletion.create(
                model="gpt-4",
                messages=[
                    {"role": "system", "content": "You are an agent registration assistant."},
                    {"role": "user", "content": "为 alice 注册一个 test 等级的 agent"},
                    message,
                    {
                        "role": "tool",
                        "tool_call_id": tool_call.id,
                        "content": json.dumps(result)
                    }
                ]
            )
            print(f"最终回复: {second_response.choices[0].message.content}")


if __name__ == "__main__":
    main()
