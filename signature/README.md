# signature/

- `agent.sig` is produced by `workagent seal` and records the whole package's per-file digests.
- **By default it is digest-only and does not pretend to be a cryptographic signature.** If you hold a
  GPG or Ed25519 private key, sign `agent.sig` again with an external tool and put the public key in
  this directory.
- The `scheme` field states truthfully which method was used (`digest-only` or `ed25519`).
- **Principle of honesty**: without a private key, do not claim "signed". A digest proves only that the
  content is unchanged; it cannot prove who produced it.
