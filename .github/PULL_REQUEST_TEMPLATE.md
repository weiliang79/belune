<!--
Thanks for contributing to Belune.

Every commit needs a DCO sign-off (`git commit -s`). See CONTRIBUTING.md.
-->

## What and why

<!-- What does this change, and what problem does it solve? Link the issue: Fixes #123 -->

## How it was tested

<!--
Say what you actually ran. For anything touching deploys, databases, backups, or
the proxy, please test against a real dev stack — mocks do not catch Docker
behaviour. Note the image tags / versions you verified against.
-->

## Checklist

- [ ] Every commit is signed off (`git commit -s`)
- [ ] `task lint` and `task test` pass
- [ ] Docs updated if behaviour changed
- [ ] Database changes are a new forward-only migration plus `task generate:sqlc`
      (no edits to generated files)
