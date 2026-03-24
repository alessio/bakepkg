#!/bin/bash

set -x

VER=$1
ARCH=$2
BIN=$3

SELF_DIR=$(SELF=$(dirname "$0") && bash -c "cd \"$SELF\" && pwd")
TARGET_DIR="$(dirname "${BIN}")"

cat >${TARGET_DIR}/bakepkg.json <<EOF
{
  "id": "dev.essio.al.bakepkg",
  "name": "bakepkg",
  "version": "${VER}",
  "output": "${TARGET_DIR}/bakepkg-$VER-$ARCH.pkg",
  "symlink_binaries": true,
  "files": {
    "bin": ["${BIN}"],
    "share": ["$SELF_DIR/README.md", "$SELF_DIR/LICENSE", "$SELF_DIR/examples"]
  }
}
EOF

${BIN} -config "${TARGET_DIR}/bakepkg.json" -verbose -debug
echo $?
