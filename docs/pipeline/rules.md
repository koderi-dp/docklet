# Project rules

## before dispatch

- Stop before anything that needs a real registry account, a code-signing certificate, a force push: these are the user's decisions.

## in every implementer brief

- Commit as the repository's git user with no Co-Authored-By or other attribution line.
- Never add DESIGN.md, docker-desktop-alternative-spec.md, design-references/ or docs/ROADMAP.md to git, and never ship mocked data.
- Keep files small (about 500 lines, split by responsibility) and write only unit tests that check real behaviour.
- When testing against a live engine, create only throwaway `docklet-test-*` resources: leave the `catalo` containers, the `Ubuntu-24.04` WSL distribution and the Podman machine untouched, and never run `wsl --shutdown`.

## in every reviewer brief

- UI changes must be aligned and readable in both the dark and the light theme.

## before the PR

- A UI change is checked in the running app, not only by tests.

## after merge

- Read the real conclusion of the CI run; never tag a commit whose message contains `[skip ci]`.
