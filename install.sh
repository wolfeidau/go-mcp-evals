#!/bin/sh
set -e

# install.sh - Install the latest release of mcp-evals
# Usage: curl -sSfL https://raw.githubusercontent.com/wolfeidau/go-mcp-evals/main/install.sh | sh

# Repository information
REPO="wolfeidau/go-mcp-evals"
BINARY_NAME="mcp-evals"

# Detect OS and architecture
detect_platform() {
  OS="$(uname -s)"
  ARCH="$(uname -m)"

  case "$OS" in
    Linux*)
      OS="Linux"
      ;;
    Darwin*)
      OS="Darwin"
      ;;
    MINGW*|MSYS*|CYGWIN*)
      OS="Windows"
      ;;
    *)
      echo "Unsupported operating system: $OS"
      exit 1
      ;;
  esac

  case "$ARCH" in
    x86_64|amd64)
      ARCH="x86_64"
      ;;
    aarch64|arm64)
      ARCH="arm64"
      ;;
    *)
      echo "Unsupported architecture: $ARCH"
      exit 1
      ;;
  esac

  echo "Detected platform: ${OS}_${ARCH}"
}

# Get the latest release version from GitHub
get_latest_release() {
  LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"

  if command -v curl >/dev/null 2>&1; then
    VERSION=$(curl -sSfL "$LATEST_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  elif command -v wget >/dev/null 2>&1; then
    VERSION=$(wget -qO- "$LATEST_URL" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
  else
    echo "Error: curl or wget is required"
    exit 1
  fi

  if [ -z "$VERSION" ]; then
    echo "Error: Unable to determine latest release"
    exit 1
  fi

  echo "Latest release: $VERSION"
}

# Verify checksum
verify_checksum() {
  # Strip 'v' prefix from VERSION for checksums filename
  VERSION_NO_V=$(echo "$VERSION" | sed 's/^v//')
  CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/go-mcp-evals_${VERSION_NO_V}_checksums.txt"

  echo "Downloading checksums..."

  # Download checksums file
  if command -v curl >/dev/null 2>&1; then
    curl -sSfL "$CHECKSUMS_URL" -o "${TMP_DIR}/checksums.txt"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "${TMP_DIR}/checksums.txt" "$CHECKSUMS_URL"
  fi

  # Extract expected checksum for our archive
  EXPECTED_CHECKSUM=$(grep "${ARCHIVE_NAME}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')

  if [ -z "$EXPECTED_CHECKSUM" ]; then
    echo "Warning: Could not find checksum for ${ARCHIVE_NAME}"
    return 1
  fi

  echo "Verifying checksum..."

  # Calculate actual checksum
  if command -v shasum >/dev/null 2>&1; then
    ACTUAL_CHECKSUM=$(shasum -a 256 "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $1}')
  elif command -v sha256sum >/dev/null 2>&1; then
    ACTUAL_CHECKSUM=$(sha256sum "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $1}')
  else
    echo "Warning: sha256sum or shasum not found, skipping checksum verification"
    return 1
  fi

  if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
    echo "Error: Checksum verification failed!"
    echo "Expected: $EXPECTED_CHECKSUM"
    echo "Got: $ACTUAL_CHECKSUM"
    exit 1
  fi

  echo "✓ Checksum verified"
}

# Download and install the binary
install_binary() {
  # Construct archive name based on goreleaser template
  # Note: Archive is named go-mcp-evals but contains mcp-evals binary
  if [ "$OS" = "Windows" ]; then
    ARCHIVE_NAME="go-mcp-evals_${OS}_${ARCH}.zip"
    ARCHIVE_EXT="zip"
  else
    ARCHIVE_NAME="go-mcp-evals_${OS}_${ARCH}.tar.gz"
    ARCHIVE_EXT="tar.gz"
  fi

  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"

  echo "Downloading from: $DOWNLOAD_URL"

  # Create temporary directory
  TMP_DIR=$(mktemp -d)
  trap 'rm -rf "$TMP_DIR"' EXIT

  # Download archive
  if command -v curl >/dev/null 2>&1; then
    curl -sSfL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "${TMP_DIR}/${ARCHIVE_NAME}" "$DOWNLOAD_URL"
  fi

  # Verify checksum
  verify_checksum

  # Extract archive
  cd "$TMP_DIR"
  if [ "$ARCHIVE_EXT" = "zip" ]; then
    unzip -q "$ARCHIVE_NAME"
  else
    tar -xzf "$ARCHIVE_NAME"
  fi

  # Determine install location
  if [ -n "$INSTALL_DIR" ]; then
    BIN_DIR="$INSTALL_DIR"
  elif [ -w "/usr/local/bin" ]; then
    BIN_DIR="/usr/local/bin"
  elif [ -w "$HOME/.local/bin" ]; then
    BIN_DIR="$HOME/.local/bin"
    mkdir -p "$BIN_DIR"
  else
    BIN_DIR="$HOME/bin"
    mkdir -p "$BIN_DIR"
  fi

  # Install binary
  if [ "$OS" = "Windows" ]; then
    BINARY="${BINARY_NAME}.exe"
  else
    BINARY="$BINARY_NAME"
  fi

  echo "Installing to: ${BIN_DIR}/${BINARY}"

  if [ -w "$BIN_DIR" ]; then
    mv "$BINARY" "${BIN_DIR}/"
    chmod +x "${BIN_DIR}/${BINARY}"
  else
    echo "Installing to $BIN_DIR requires elevated privileges"
    sudo mv "$BINARY" "${BIN_DIR}/"
    sudo chmod +x "${BIN_DIR}/${BINARY}"
  fi

  echo ""
  echo "✓ ${BINARY_NAME} ${VERSION} installed successfully to ${BIN_DIR}/${BINARY}"
  echo ""

  # Check if install directory is in PATH
  case ":$PATH:" in
    *":$BIN_DIR:"*)
      ;;
    *)
      echo "Note: Add $BIN_DIR to your PATH to use ${BINARY_NAME} from anywhere:"
      echo "  export PATH=\"\$PATH:$BIN_DIR\""
      ;;
  esac
}

# Main installation flow
main() {
  detect_platform
  get_latest_release
  install_binary
}

main
