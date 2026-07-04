# Git operations
# Call as: just git::version

# Tag a release: bump the semver tag, run guards, and push (triggers the
# tag-gated release CI). Usage: just git::version major|minor|hotfix [-y]
# Pass -y (or set YES=1) to skip the confirmation. Set SKIP_TESTS=1 to skip tests.
[group('git')]
[group('deploy')]
[no-cd]
version bump confirm="":
    #!/usr/bin/env bash
    set -euo pipefail

    # Validate argument
    if [[ "{{ bump }}" != "major" && "{{ bump }}" != "minor" && "{{ bump }}" != "hotfix" ]]; then
        echo "Error: argument must be 'major', 'minor', or 'hotfix'"
        echo "Usage: just git::version major|minor|hotfix"
        exit 1
    fi

    # Guard: working tree must be clean (don't tag a dirty state).
    if ! git diff-index --quiet HEAD -- 2>/dev/null; then
        echo "Error: working tree has uncommitted changes. Commit or stash first."
        exit 1
    fi

    # Guard: local branch must be in sync with origin, so the tag points at a
    # commit that exists on the remote (the release build checks out the tag).
    BRANCH=$(git rev-parse --abbrev-ref HEAD)
    git fetch --quiet origin "$BRANCH" 2>/dev/null || true
    if git rev-parse --verify --quiet "origin/$BRANCH" >/dev/null; then
        AHEAD=$(git rev-list --count "origin/$BRANCH..HEAD")
        if [[ "$AHEAD" -gt 0 ]]; then
            echo "Error: $AHEAD local commit(s) not pushed to origin/$BRANCH. Run 'git push' first."
            exit 1
        fi
    fi

    # Guard: tests must pass before we cut a release.
    if [[ "${SKIP_TESTS:-0}" != "1" ]]; then
        echo "Running tests..."
        ( cd "{{ justfile_directory() }}/src/huly" && go test ./... )
    else
        echo "SKIP_TESTS=1 — skipping test gate."
    fi

    # Get latest version tag
    LATEST=$(git tag --list 'v*' --sort=-v:refname | head -1)
    if [[ -z "$LATEST" ]]; then
        LATEST="v0.0.0"
        echo "No existing tags found, starting from v0.0.0"
    fi
    echo "Current version: $LATEST"

    # Parse semver
    VERSION="${LATEST#v}"
    IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

    # Increment
    case "{{ bump }}" in
        major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
        minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
        hotfix) PATCH=$((PATCH + 1)) ;;
    esac

    NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"
    echo "New version: $NEW_TAG ({{ bump }} bump from $LATEST)"

    # Confirm before creating a release-triggering tag, unless -y / YES=1.
    if [[ "{{ confirm }}" != "-y" && "{{ confirm }}" != "-Y" && "{{ confirm }}" != "yes" && "${YES:-0}" != "1" ]]; then
        read -r -p "Tag and push $NEW_TAG? This triggers the release build. [y/N] " reply
        if [[ "$reply" != "y" && "$reply" != "Y" ]]; then
            echo "Aborted."
            exit 1
        fi
    fi

    # Create and push the annotated tag → fires the tag-gated release CI.
    git tag -a "$NEW_TAG" -m "Release $NEW_TAG"
    git push origin "$NEW_TAG"
    echo "Tag $NEW_TAG created and pushed."

    REMOTE=$(git remote get-url origin 2>/dev/null || echo "")
    if [[ "$REMOTE" == *github.com* ]]; then
        SLUG=$(echo "$REMOTE" | sed -E 's#(git@github.com:|https://github.com/)##; s#\.git$##')
        echo "Release build: https://github.com/${SLUG}/actions"
    fi
