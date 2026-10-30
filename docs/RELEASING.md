# Releasing

Dispatch `.github/workflows/release.yml` by hand against a release tag; it vets,
tests, and publishes with GoReleaser (`.goreleaser.yaml`).

- Keep cosign at v2 in `release.yml`; the comment there says why.
- Keep the archive `name_template`: `mise use -g github:hk9890/chromectl` picks
  the asset by it.
- Change `archives.files` together with the unpack command in `README.md` and
  the asset list in step 7.

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
5. Tag and push. The `v*` tag ruleset lets only a repository admin create,
   move or delete the tag.

   ```bash
   git tag -a vX.Y.Z -m "chromectl vX.Y.Z"
   git push origin vX.Y.Z
   ```

6. Dispatch the workflow and watch the run whose URL it prints; on a
   re-dispatch, a lookup by tag can return the previous run. A tag with no
   `CHANGELOG.md` section fails before anything is published.

   ```bash
   gh workflow run release.yml --ref vX.Y.Z
   gh run watch <run id, the last part of that URL> --exit-status
   ```

7. Verify: two archives, `_linux_x64` and `_linux_arm64`, each holding
   `chromectl` and `LICENSE`; two `.sbom.json`; `…_checksums.txt` with its
   `.sig` and `.pem`; and a body that is the tag's `CHANGELOG.md` section. An empty
   body is repaired with `gh release edit vX.Y.Z --notes-file`, no re-dispatch.

   ```bash
   gh release view vX.Y.Z --json assets,body -q '.body, (.assets[].name)'
   dir=$(mktemp -d)
   gh release download vX.Y.Z -p '*_linux_x64.tar.gz' -D "$dir"
   gh attestation verify "$dir"/chromectl_X.Y.Z_linux_x64.tar.gz -R hk9890/chromectl
   ```

## Fix a failed release

The dispatched ref must be the tagged commit: the workflow file is read from
that ref, and GoReleaser refuses a checkout that is not the tag.

1. Land the fix by PR.
2. Move the tag onto the new `main`:

   ```bash
   git switch main && git pull --ff-only
   git tag -f -a vX.Y.Z -m "chromectl vX.Y.Z"
   git push -f origin vX.Y.Z
   ```

3. Run steps 6 and 7 again. A re-dispatch replaces the assets
   (`release.replace_existing_artifacts`).

## What goes in the CHANGELOG section

The section is the release text on GitHub. Write it for the person who runs
`chromectl` and has never opened the source:

- Features, changed behaviour and fixes; never a commit list.
- State the symptom, not the cause. No package, function or test names.
- No entry for a change with no visible effect: a refactor, a test, a doc fix.
- A change that breaks an existing setup leads with **Action required** and
  names the fix, for example a reload of the extension card.
