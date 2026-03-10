#!/bin/bash
# Release script for Find Large Files
# Usage: ./scripts/release.sh <version>
# Example: ./scripts/release.sh 0.3.0

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if version is provided
if [ -z "$1" ]; then
    echo -e "${RED}Error: Version number required${NC}"
    echo "Usage: ./scripts/release.sh <version>"
    echo "Example: ./scripts/release.sh 0.3.0"
    exit 1
fi

VERSION=$1
TAG="v${VERSION}"
CURRENT_BRANCH=$(git branch --show-current)

echo -e "${BLUE}=== Find Large Files Release Script ===${NC}"
echo -e "${YELLOW}Version: ${VERSION}${NC}"
echo -e "${YELLOW}Branch: ${CURRENT_BRANCH}${NC}"
echo ""

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo -e "${RED}Error: Tag ${TAG} already exists${NC}"
    exit 1
fi

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo -e "${RED}Error: You have uncommitted changes${NC}"
    echo "Please commit or stash your changes first"
    exit 1
fi

# Update version in main.go
echo -e "${BLUE}Updating version in main.go...${NC}"
sed -i "s/version = \".*\"/version = \"${VERSION}\"/" main.go
git add main.go
git commit -m "Bump version to ${VERSION}"

# Build all platforms
echo -e "${BLUE}Building all platforms...${NC}"
rm -rf build/*
./scripts/build.sh all

if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}Build completed successfully!${NC}"
echo ""

# Create git tag
echo -e "${BLUE}Creating git tag ${TAG}...${NC}"
git tag -a "$TAG" -m "Release version ${VERSION}"

echo ""
echo -e "${GREEN}=== Release preparation completed ===${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Push the changes and tag:"
echo -e "   ${BLUE}git push origin ${CURRENT_BRANCH}${NC}"
echo -e "   ${BLUE}git push origin ${TAG}${NC}"
echo -e "   ${BLUE}git push origin ${CURRENT_BRANCH} --follow-tags${NC}"
echo ""
echo "2. Wait for GitHub Actions workflow '.github/workflows/release.yml' to run."
echo "   It is triggered by pushing tag ${TAG}."
echo ""
echo "3. Release assets expected from CI:"
ls -lh build/
echo ""
echo -e "${YELLOW}4. Optional manual GitHub CLI release command:${NC}"
echo -e "   ${BLUE}gh release create ${TAG} build/* --title \"${TAG}\" --notes \"Release ${VERSION}\"${NC}"
