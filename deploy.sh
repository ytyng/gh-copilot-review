#!/usr/bin/env zsh

cd "$(dirname "$0")"

echo "Building gh-copilot-review..."
go build -o gh-copilot-review ./

echo "Installing gh-copilot-review extension..."
gh extension install .

echo "Testing gh-copilot-review extension..."
gh copilot-review --help

gh copilot-review status 1


