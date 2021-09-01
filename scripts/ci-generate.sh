#!/usr/bin/env bash
#
# A script to be run with a clean working tree.
# Runs the code generator and then ensures the working tree is still clean.

set -e

if [[ $(git diff --stat) != '' ]]; then
  echo 'FAILURE: Dirty working tree BEFORE code generation!'
  exit 1
fi

go generate ./...

if [[ $(git diff --stat) != '' ]]; then
  echo 'FAILURE: Dirty working tree after code generation.'
  exit 1
fi
