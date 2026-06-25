# Git operations
# Call as: just git::version

# Bump version tag and push (usage: just git::version major|minor|hotfix)
[group('git')]
[no-cd]
version bump:
    #!/usr/bin/env bash
    set -euo pipefail

    # Validate argument
    if [[ "{{ bump }}" != "major" && "{{ bump }}" != "minor" && "{{ bump }}" != "hotfix" ]]; then
        echo "Error: argument must be 'major', 'minor', or 'hotfix'"
        echo "Usage: just git::version major|minor|hotfix"
        exit 1
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
    echo "New version: $NEW_TAG"

    # Create and push tag
    git tag -a "$NEW_TAG" -m "Release $NEW_TAG"
    git push origin "$NEW_TAG"
    echo "Tag $NEW_TAG created and pushed"
