# Releasing

`.github/workflows/release.yml` publishes a release with GoReleaser
(`.goreleaser.yaml`), dispatched by hand against a release tag. The run vets
and tests before it publishes.

A release carries two archives, `chromectl_<version>_linux_x64.tar.gz` and
`…_linux_arm64.tar.gz`, each holding `chromectl` alone; the binary embeds the
extension. Beside them: one SPDX SBOM per archive, a checksums file signed
keyless with cosign (`.sig` and `.pem`), and SLSA build provenance. The archive
names let `mise use -g github:hk9890/chromectl` pick the right asset.

cosign is pinned to v2.6.5 in `release.yml`: v3 defaults to
`--new-bundle-format`, which rejects the `--output-signature` and
`--output-certificate` flags in `.goreleaser.yaml`.

## Cut a release

1. Pick `vX.Y.Z` and confirm it is free: `git tag --list "v*"`.
2. Land a PR ([CHANGE-WORKFLOW.md](CHANGE-WORKFLOW.md)) that adds the
   `## vX.Y.Z` section to `CHANGELOG.md` and sets `"version"` in
   `extension/manifest.json` to `X.Y.Z`.
3. On `main` at `origin/main`, with a clean tree, run the gate:

   ```bash
   git switch main && git pull --ff-only && git status --porcelain
   mise run quality && mise run drive
   ```

4. Check the build on the live Chrome ([RUNNING.md](RUNNING.md)): run
   `mise run install`, the user reloads the chromectl card, then
   `./bin/chromectl list` answers. The Go tests cannot see the extension.
5. Tag and push:

   ```bash
   git tag -a vX.Y.Z -m "chromectl vX.Y.Z"
   git push origin vX.Y.Z
   ```

6. Dispatch the workflow and watch it. The run takes a few seconds to
   register; rerun the lookup if it prints nothing. A tag with no
   `CHANGELOG.md` section fails before anything is published.

   ```bash
   gh workflow run release.yml --ref vX.Y.Z
   gh run watch "$(gh run list --workflow=release.yml --branch vX.Y.Z --limit 1 --json databaseId -q '.[0].databaseId')" --exit-status
   ```

7. Verify: two archives, two `.sbom.json`, `…_checksums.txt` with its `.sig`
   and `.pem`, and a body that is the tag's `CHANGELOG.md` section. An empty
   body is repaired with `gh release edit vX.Y.Z --notes-file`, no re-dispatch.

   ```bash
   gh release view vX.Y.Z --json assets,body -q '.body, (.assets[].name)'
   gh release download vX.Y.Z -p '*_linux_x64.tar.gz' -D "$(mktemp -d)"
   gh attestation verify <that dir>/chromectl_X.Y.Z_linux_x64.tar.gz -R hk9890/chromectl
   ```

**The dispatched ref must be the tagged commit.** The workflow file is read from
that ref, and GoReleaser refuses a checkout that is not the tag. A
`release.yml` fix a release needs lands by PR first; then move the tag onto the
new `main` with `git tag -f -a` and `git push -f origin vX.Y.Z`, and dispatch
again. A re-dispatch replaces the assets (`release.replace_existing_artifacts`).

## What goes in the CHANGELOG section

The section is the release text on GitHub. Write it for the person who runs
`chromectl` and has never opened the source:

- Features, changed behaviour and fixes; never a commit list.
- State the symptom, not the cause. No package, function or test names.
- No entry for a change with no visible effect: a refactor, a test, a doc fix.
- A change that breaks an existing setup leads with **Action required** and
  names the fix, for example a reload of the extension card.
