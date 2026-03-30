#!/bin/bash
cd "$(dirname "$0")" || exit
echo "Building..."
go build -o mark2note ./cmd/mark2note/
echo "Done. Binary: ./mark2note"
