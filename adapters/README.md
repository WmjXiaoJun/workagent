# adapters/

**Generated, not a source of truth.** Produced by `workagent adapt`:

| File | Target ecosystem | What it is |
|---|---|---|
| `plugin.json` | Agent Plugins 1.0 (led by Vercel; supported by OpenAI/AWS/GitHub/Microsoft) | a portable plugin manifest |
| `agent-card.json` | A2A (Agent2Agent) | a capability and identity self-description, for discovery |

The source of truth is always `agent.yaml` at the package root. Editing the adapter layer is wasted
work: the next generation overwrites it.
