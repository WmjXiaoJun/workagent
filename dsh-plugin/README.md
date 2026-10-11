# WorkAgent — DeepSeek Harness plugin

Exposes a WorkAgent delivery package to the Harness as **MCP tools**. Once installed you can just say
"run the points reconciliation" in a conversation, and the agent calls it and gets back a
**reproducible conclusion**.

[中文](README.zh-CN.md) · **English**

## What it does

The plugin itself **contains no logic** — it is five lines of configuration that connect the
`workagent mcp` stdio server to the Harness. The capability lives in the delivery package.

| Tool | Purpose |
|---|---|
| `workagent_cases` | List the package's scenarios, grouped by **delivery form** |
| `workagent_run` | Run one scenario and return its conclusion plus the evidence bundle id |
| `workagent_verify` | Reproducibility check: all scenarios must PASS for the package to be deliverable |

## Why MCP rather than a Cordis plugin

Because **an MCP server can be tested offline**, while a Cordis host plugin can only be verified inside
a live Harness. Going through MCP also gives you Claude Desktop, Cursor and any other MCP client for
free — one implementation, many consumers.

## Prerequisites

```bash
# 1) the workagent binary (a single static file, no dependencies)
workagent --help

# 2) sanity-check the MCP server
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | workagent mcp .
```

## Install

### From npm (recommended)

`plugin_manager.installBundle` already resolves **npm registries** (with an npmmirror
fallback), so publishing to npm *is* the DSH ecosystem distribution path:

```text
plugin_manager  action: install_bundle
                target: dsh-plugin-workagent
```

### From a local directory

Point `target` at the **absolute path of this directory** (handy while developing or editing the
config):

```text
plugin_manager  action: install_bundle
                target: <absolute path to this directory>
```

### From GitHub

`installBundle` checks a repository with `git ls-remote` before handing it to pnpm:

```text
plugin_manager  action: install_bundle
                target: github:WmjXiaoJun/workagent#path:dsh-plugin
```

> The exact spec syntax depends on your `plugin_manager` version; the first two are the paths we
> have actually verified.

#### What has to happen, mechanically

A profile records its bundles in an explicit list inside its own `package.json`:

```json
{ "dsh": { "profile": { "bundles": [ "@deepseek-ai/dsh-base", "...", "dsh-plugin-workagent" ] } } }
```

`install_bundle` does **two** things: installs the package, and adds its name to that list.
The `dsh plugin` CLI only forwards its arguments to pnpm, so it covers the first half — the second
half is what makes the bundle actually load, and it is why the supported path is `plugin_manager`.

## Configuration

Edit `cordis.patch.yml`:

| Field | Meaning |
|---|---|
| `command` | the `workagent` executable; use an absolute path if it is not on `PATH` |
| `args` | `["mcp", "."]` — the second argument is the package root; point it at your own package |
| `cwd` | optional working directory |

## Publishing to npm

The package is already configured for publication (`private` removed, license / author /
repository / keywords / files in place).

```bash
cd dsh-plugin
npm login                       # your own npm account
npm publish --access public     # --access is unnecessary for unscoped packages
```

Self-checks that need no login:

```bash
npm pack --dry-run              # see exactly which files would be packed
npm publish --dry-run           # run the whole publication flow without publishing
```

### ⚠️ One prerequisite you must know

**This package contains configuration only, not the `workagent` executable.**
DSH **blocks package install scripts by default** (pnpm 11's `pendingBuilds` mechanism), so it cannot
download a binary during installation.

After installing the plugin, `workagent` must be on `PATH`, or the MCP server will not start.
`failOnStartupError: true` makes it **fail loudly** rather than silently doing nothing — that is
deliberate.

```bash
# fetch the static binary for your platform from GitHub Releases
# https://github.com/WmjXiaoJun/workagent/releases/latest
workagent --help
```

## ⚠️ Verification status

- ✅ **The MCP server side is verified**: handshake, `tools/list`, `tools/call` and unknown-tool
  errors were all exercised.
- ⚠️ **The Cordis loading side is not.** There is no `plugin_manager` tool available here, so it
  could not be installed locally — therefore "it works inside the Harness once installed" is
  **not something we can claim**. Please confirm it yourself after installing.

## Uninstall

`plugin_manager`"s `remove_bundle`.
