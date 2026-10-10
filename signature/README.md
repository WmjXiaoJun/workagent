# signature/

- `agent.sig` 由 `python tools/workagent.py seal` 生成，记录整个包的文件级摘要。
- **默认只做摘要（digest-only），不冒充密码学签名。** 如果你有 GPG 或 Ed25519 私钥，
  用外部工具对 `agent.sig` 再签一次，并把公钥放进本目录。
- 字段 `scheme` 会如实标明本次采用的方式（`digest-only` 或 `ed25519`）。
- **诚实原则**：没有私钥就不要声称"已签名"。摘要只能证明"内容没变"，不能证明"谁签的"。
