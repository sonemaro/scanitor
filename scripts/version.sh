#!/bin/bash

# Get the last git tag
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null)

# If no tags exist, use v0.1.0
if [ -z "$GIT_TAG" ]; then
    GIT_TAG="v0.1.0"
fi

# Get number of commits since tag
COMMITS_SINCE_TAG=$(git rev-list ${GIT_TAG}..HEAD --count 2>/dev/null || echo "0")

# Get current commit hash
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Get git branch
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Check if working directory is clean
if [[ -n $(git status -s 2>/dev/null) ]]; then
    DIRTY="-dirty"
else
    DIRTY=""
fi

# Build version string
if [ "$COMMITS_SINCE_TAG" -eq "0" ]; then
    VERSION="${GIT_TAG}${DIRTY}"
else
    VERSION="${GIT_TAG}-${COMMITS_SINCE_TAG}-g${COMMIT_HASH}${DIRTY}"
fi

# Build number (using date and time if not in CI)
if [ -n "$CI" ]; then
    BUILD_NUM="${GITHUB_RUN_NUMBER:-${CIRCLE_BUILD_NUM:-0}}"
else
    BUILD_NUM=$(date +%Y%m%d%H%M%S)
fi

# Output in format suitable for Makefile
cat << EOF
VERSION=${VERSION}
BUILD_NUM=${BUILD_NUM}
COMMIT_HASH=${COMMIT_HASH}
BRANCH=${BRANCH}
BUILD_DATE=$(date -u '+%Y-%m-%d %H:%M:%S')
EOF
