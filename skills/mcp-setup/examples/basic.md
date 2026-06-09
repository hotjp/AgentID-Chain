# mcp-setup 示例

## Claude Desktop (stdio)

```python
result = call_tool("mcp_setup", {"client": "claude-desktop"})
print(f"Config written to: {result['config_path']}")
```

## Cursor (http)

```python
result = call_tool("mcp_setup", {
    "client": "cursor",
    "transport": "http",
    "http_url": "https://agentid.example.com/mcp"
})
```
