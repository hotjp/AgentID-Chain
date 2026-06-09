#!/usr/bin/env python3
"""register-agent Python 示例 (MCP 客户端)."""

import asyncio
import json
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client


async def main():
    aap_jwt = "eyJhbGciOi..."  # 从 AAP 鉴权获取

    async with streamablehttp_client(
        "http://localhost:8080/mcp/v1/rpc",
        headers={"Authorization": f"Bearer {aap_jwt}"}
    ) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()

            # 调用 register_agent
            result = await session.call_tool(
                "register_agent",
                {
                    "owner": "alice",
                    "level": "test",
                    "metadata": {
                        "team": "infra",
                        "env": "dev"
                    }
                }
            )

            for content in result.content:
                if content.type == "text":
                    agent = json.loads(content.text)
                    print(f"Registered: {agent['agent_id']}")
                    print(f"  Owner: {agent['owner']}")
                    print(f"  Level: {agent['level']}")
                    print(f"  Status: {agent['status']}")


if __name__ == "__main__":
    asyncio.run(main())
