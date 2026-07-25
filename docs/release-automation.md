# Release automation

Release Please owns the repository's future semantic versions, changelog
entries, `v*` tags, and GitHub Releases. It does not deploy production.
Cloud Build continues to test, build, and deploy every successful `main` commit
as an image tagged with that commit's full SHA.

## Release flow

1. A normal pull request is squash-merged to `main`.
2. Release Please reads the resulting Conventional Commit on `main` and opens
   or updates one release pull request.
3. CI runs on that release pull request. Merging it does not deploy a special
   release build; it is another `main` commit and follows the existing
   commit-SHA Cloud Build path.
4. The next Release Please run creates the version tag and GitHub Release.
5. The asset workflow starts on `release.published`, checks out that tag, runs
   the Go tests, builds `gateway-linux-amd64`, creates `checksums.txt`, and
   uploads both files to the already-existing GitHub Release.

The asset workflow must not run on a tag push and must not create a release.
A tag-triggered release creator races Release Please because the tag and release
are separate GitHub resources. The `published` release event instead supplies
the tagged release commit as `GITHUB_SHA` and its tag as `GITHUB_REF`.
[GitHub documents the event payload and recommends
`types: [published]`](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#release).
Release Please also documents that it creates the tag and GitHub Release after
its release pull request is merged, and exposes `release_created`, `tag_name`,
`sha`, and `upload_url` outputs.
[Release Please lifecycle](https://github.com/googleapis/release-please/tree/v17.6.0#whats-a-release-pr)
[Action outputs](https://github.com/googleapis/release-please-action/blob/45996ed1f6d02564a971a2fa1b5860e934307cf7/README.md#outputs)

For recovery, a manual asset run may accept an existing `v*` tag, but it must
first verify that the matching GitHub Release exists. `gh release upload`
uploads to an existing release; `--clobber` makes a retry replace same-named
assets, although a failed replacement can remove the prior asset.
[GitHub CLI reference](https://cli.github.com/manual/gh_release_upload)

The live repository reported `immutable releases: disabled` on 2026-07-25.
Keep that setting disabled for this flow: GitHub does not allow assets to be
changed after an immutable release is published. If immutable releases become
a requirement, redesign this handoff around a Release Please-created draft,
upload the tested assets, and publish the draft only after the upload succeeds.
[GitHub immutable release behavior](https://cli.github.com/manual/gh_release_create#immutable-releases)

## Bootstrap and configuration

The repository has two real historical tags: `v1.0.0` and `v2.0.0`. The
[Release Please configuration](../release-please-config.json) fixes the first
automated changelog's exclusive lower bound at the real `v2.0.0` commit. The
[version manifest](../.release-please-manifest.json) is the current-version
source of truth and the root package configuration preserves the established
`vMAJOR.MINOR.PATCH` tag shape. Do not relabel earlier commits or create a
synthetic version.

Release Please says the bootstrap SHA is exclusive and is ignored after a
generated release pull request has been merged.
[Manifest bootstrap documentation](https://github.com/googleapis/release-please/blob/v17.6.0/docs/manifest-releaser.md#bootstrapping)
[Configuration schema](https://github.com/googleapis/release-please/blob/v17.6.0/schemas/config.json)
[Manifest schema](https://github.com/googleapis/release-please/blob/v17.6.0/schemas/manifest.json)

The Go release strategy updates `CHANGELOG.md`. The manifest remains the
semantic-version source of truth. Existing changelog prose predates Release
Please and remains intact. A short banner marks its dated `[Unreleased]`
sections as historical; Release Please inserts future generated versions above
them without assigning invented versions to old prose.
[Supported Go strategy](https://github.com/googleapis/release-please/tree/v17.6.0#strategy-language-types-supported)

`cmd/api/main.go` independently logs the stale hardcoded value `1.0.0`.
Do not bind that value to Release Please in this change: production deploys
unreleased `main` commits, so a release version would misidentify those
revisions. Track a separate change to inject the immutable build commit SHA
into the binary (for example with Go linker flags) and expose that deployment
provenance.

## Merge and commit contract

Follow the repository's
[pull request title and squash-merge contract](../CONTRIBUTING.md). Release
Please parses commits on the target branch, and its maintainers recommend
squash merging so one reviewed pull request becomes one controlled changelog
entry.
[Conventional Commit mapping and squash recommendation](https://github.com/googleapis/release-please/tree/v17.6.0#how-should-i-write-my-commits)
[Releasable units](https://github.com/googleapis/release-please/tree/v17.6.0#step-1-ensure-releasable-units-are-merged)

## GitHub App operator contract

Use a dedicated GitHub App installation token, not a human personal access
token and not the workflow's default `GITHUB_TOKEN`. GitHub currently suppresses
most workflow runs caused by `GITHUB_TOKEN`; automated pull-request events are
created in an approval-required state. GitHub explicitly says an App
installation token makes those pull-request workflows run automatically and
allows other token-created events to trigger workflows. This is required both
for unattended required CI on the release pull request and for the
`release.published` asset handoff.
[GitHub workflow recursion rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow#triggering-a-workflow-from-a-workflow)

One-time setup by a repository or organization owner:

1. Create a dedicated GitHub App for release automation.
2. Grant only repository `Contents: read and write`, `Pull requests: read and
   write`, and `Issues: read and write`. Contents covers branches, tags, and
   releases; pull-request write opens and updates the release PR; issues write
   is used for Release Please's lifecycle labels.
3. Install the App on only
   `Data-Buddies-Solutions/amd_middleware` (GitHub currently redirects the
   repository's older remote name).
4. Add the App client ID as the repository Actions variable
   `RELEASE_PLEASE_APP_CLIENT_ID`.
5. Add the App private key PEM as the repository Actions secret
   `RELEASE_PLEASE_APP_PRIVATE_KEY`.
6. Ensure the repository's Actions policy allows the pinned Google and GitHub
   actions below.
7. Configure `main` protection or a repository ruleset to require pull requests
   and the existing CI build-and-test job. No branch protection or ruleset was
   active when inspected on 2026-07-25.
8. Make squash merge the repository convention. Set the squash commit title to
   the pull request title, and disable merge commits and rebase merges if the
   convention should be enforced in GitHub rather than by review.
9. Keep immutable releases disabled. If that policy changes, redesign the asset
   handoff before enabling it.

No App credential belongs in the repository. `actions/create-github-app-token`
should omit `owner` and `repositories`; the official action then scopes the
installation token to the current repository. Request the three permissions
explicitly with `permission-contents`, `permission-pull-requests`, and
`permission-issues` so the token does not inherit broader App permissions.
The workflow-level `GITHUB_TOKEN` can have `permissions: {}` in the Release
Please job because the App token owns all GitHub writes. The downstream asset
workflow needs only `contents: write` for checkout and release-asset upload.
[App token scope and permission inputs](https://github.com/actions/create-github-app-token/blob/bcd2ba49218906704ab6c1aa796996da409d3eb1/README.md#inputs)
[Release Please's documented permissions](https://github.com/googleapis/release-please-action/blob/45996ed1f6d02564a971a2fa1b5860e934307cf7/README.md#workflow-permissions)
[Create-release token permission](https://docs.github.com/en/rest/releases/releases#create-a-release)
[Create-PR token permission](https://docs.github.com/en/rest/pulls/pulls#create-a-pull-request)

The [Release Please workflow](../.github/workflows/release-please.yml),
[asset workflow](../.github/workflows/release.yml), and
[CI workflow](../.github/workflows/ci.yml) are the action-pin sources of truth;
each `uses:` line carries an immutable revision and human-readable version.
GitHub can enforce full-length commit pins at the repository or organization
level.
[GitHub Actions pinning policy](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository#requiring-workflows-to-pin-actions-to-a-full-length-commit-sha)
