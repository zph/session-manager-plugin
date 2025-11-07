# Homebrew Casks

This directory contains Homebrew casks for installing the Session Manager Plugin and SSM CLI.

## Installation

### Install directly from this repository

```bash
# Session Manager Plugin
brew install zph/session-manager-plugin/session-manager-plugin

# SSM CLI
brew install zph/session-manager-plugin/ssmcli
```

Or tap the repository first:

```bash
# Tap this repository
brew tap zph/session-manager-plugin https://github.com/zph/session-manager-plugin

# Then install
brew install session-manager-plugin
brew install ssmcli
```

## How it works

The casks in this directory are automatically generated and committed by GoReleaser during the release process. When you create a new release with a git tag, GoReleaser will:

1. Build the binaries for all platforms
2. Create archives and packages
3. Generate Homebrew casks with the correct checksums and download URLs
4. Commit the casks to this directory

Users can then install directly from this GitHub repository using the commands above.

## macOS Quarantine Handling

The casks include a post-install hook that automatically removes macOS quarantine attributes from the binaries, allowing them to run without additional security prompts even though they are not signed or notarized.
