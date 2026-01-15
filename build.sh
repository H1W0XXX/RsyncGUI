#!/bin/bash
set -e

echo "=== 1. Building Frontend (Web) ==="
cd web
if [ ! -d "node_modules" ]; then
    echo "Installing npm dependencies..."
    npm install
fi
echo "Running npm build..."
npm run build
cd ..

echo "=== 2. Preparing Embed Assets ==="
TARGET_DIR="internal/uiembed/web"
rm -rf "$TARGET_DIR"
mkdir -p "$TARGET_DIR"

echo "Copying web/dist to $TARGET_DIR/dist ..."
cp -r web/dist "$TARGET_DIR/"

echo "=== 3. Building Binaries ==="
mkdir -p dist

# Common Flags
# -s -w: Strip debug info (smaller binary)
# -trimpath: Remove file system paths from panic traces
LDFLAGS="-s -w"
TRIM_FLAGS="-trimpath=$PWD"

echo "--> Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
    -ldflags "$LDFLAGS" \
    -gcflags="all=$TRIM_FLAGS" \
    -asmflags="all=$TRIM_FLAGS" \
    -o dist/rsyncgui-linux-amd64 \
    ./cmd/rsyncgui

echo "--> Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
    -ldflags "$LDFLAGS" \
    -gcflags="all=$TRIM_FLAGS" \
    -asmflags="all=$TRIM_FLAGS" \
    -o dist/rsyncgui-windows-amd64.exe \
    ./cmd/rsyncgui

echo "=== Build Complete ==="
echo "Artifacts are in 'dist/' directory:"
ls -lh dist/
