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
{ "dsh": { "profile": { "bundles": [ "@deepseek-ai/dsh-base", "dsh-plugin-workagent" ] } } }
```

Installing a bundle is **two** things: install the package, **and add its name to that list** — the
second is what makes it actually load. Both `plugin_manager.installBundle` and
`dsh plugin --profile <name> add <package>` do both.

Every path that forwards to `pnpm` and skips that list installs the files without ever loading them.

## Configuration

Edit `cordis.patch.yml`:

| Field | Meaning |
|---|---|
| `command` | the `workagent` executable; use an absolute path if it is not on `PATH` |
| `args` | `["mcp", "."]` — the second argument is the package root; point it at your own package |
| `cwd` | optional working directory |

## Publishing to npm

The package is already configured for publication (`private` removed, license / author /
repository / keywords / files in place). The name `dsh-plugin-workagent` is **not yet taken**.

### First time only: create the npm account

An npm account is an identity: it carries your name and your acceptance of npm's terms, and npm
verifies it by email. **Nobody else can do this step for you** — not least because it needs your
email and the verification link npm sends.

1. Sign up at <https://www.npmjs.com/signup>
2. **Click the verification link in the email** — publishing fails until the address is verified
3. Enable 2FA when prompted (npm requires it for publishing)

### Then publish (about two minutes)

```bash
cd dsh-plugin
npm login                       # your own npm account
npm publish --access public     # --access is unnecessary for unscoped packages
```

### Or publish from CI, so no token ever passes through a chat window

A token pasted into a conversation is an exposed token. Put it in **GitHub Secrets** instead —
it goes straight from your browser to GitHub and never through anyone else:

1. On npmjs.com: **Access Tokens → Generate New Token → Automation**
2. GitHub repo → **Settings → Secrets and variables → Actions → New repository secret**,
   name it `NPM_TOKEN`
3. **Actions → publish-dsh-plugin → Run workflow**, leaving `dry_run` at `true` first
4. Run it again with `dry_run=false`

The workflow asserts the bundle manifest is complete (patch, icon, both locales) **before** publishing,
and publishes with npm provenance.

### After it is published

Switch the profile from the local link to the published package:

```bash
dsh plugin --profile web remove dsh-plugin-workagent
dsh plugin --profile web add dsh-plugin-workagent
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

## Verification status

| Layer | Status |
|---|---|
| **MCP server** | ✅ handshake, `tools/list`, `tools/call` and unknown-tool errors were all exercised directly |
| **Cordis composition** | ✅ verified twice: in a throwaway profile, then **installed into a live profile** — both composed trees contain the row below |
| **Runtime connect** | ⚠️ installed; needs a **session restart** to take effect (installing from outside a running session does not hot-reload it) |

```yaml
# == dsh-plugin-workagent
- id: workagent-mcp
  name: '@deepseek-ai/dsh-mcp-client'
  config:
    serverName: workagent
    transport: stdio
    command: workagent
```

`dsh --profile <name> --dump-config` prints that tree without booting anything, which is how the
composition was checked.

### Two operational notes we hit while installing

**The working directory is usually not a package.** The plugin runs `workagent mcp .`, so the
package root is whatever directory the session happens to be in. The server therefore **starts even
when that directory has no `agent.yaml`**, answers `tools/list` normally, and returns an actionable
message per call:

```text
The working directory is not a WorkAgent package (no agent.yaml): <dir>.
Point the plugin's args at a package directory.
```

If it instead exited at startup, every session in an unrelated directory would fail — do not "fix" it
by making the server refuse to boot.

**Installing while the profile's session is running can hit a transient lock.** pnpm writes the
lockfile by rename, and the running harness holds the profile open:

```text
EPERM: operation not permitted, rename 'pnpm-lock.yaml.<n>' -> 'pnpm-lock.yaml'
```

The failure is clean — the profile is left untouched, and `plugin_manager` snapshots and restores
`package.json` and `pnpm-lock.yaml` on failure. **Retrying succeeded.**

### ⚠️ Order matters: put `workagent` on `PATH` first

The bundle sets `failOnStartupError: true`. If you install it into a profile while `workagent` is
**not** on `PATH`, the MCP client fails on **every** session start — you have turned a missing
prerequisite into a recurring fault.

So install the binary first, confirm `workagent --help` works, and only then install the bundle.

## Uninstall

`plugin_manager`"s `remove_bundle`.
