# a2a-setup 示例

## 配 mTLS 给指定 peer

```python
result = call_tool("a2a_setup", {
    "agent_id": "0190a3b4-7c8d-7def-9abc-def012345678",
    "peer_url": "https://peer.svc.cluster.local"
})
print(f"Credential: {result['credential_path']}")
```
