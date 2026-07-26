#!/usr/bin/env bash

set -euo pipefail

version="${1:?version is required}"
checksum_file="${2:?checksum file is required}"
tap_dir="${RUNNER_TEMP}/homebrew-tap"

sha_for() {
  local asset="${1}"
  local sha

  sha="$(awk -v asset="${asset}" '$2 == asset || $2 == "*" asset { print $1; exit }' "${checksum_file}")"

  if [[ -z "${sha}" ]]; then
    echo "Missing checksum for ${asset}" >&2
    exit 1
  fi

  printf '%s' "${sha}"
}

darwin_amd64="ytrack_${version}_darwin_amd64.tar.gz"
darwin_arm64="ytrack_${version}_darwin_arm64.tar.gz"
linux_amd64="ytrack_${version}_linux_amd64.tar.gz"
linux_arm64="ytrack_${version}_linux_arm64.tar.gz"

darwin_amd64_sha="$(sha_for "${darwin_amd64}")"
darwin_arm64_sha="$(sha_for "${darwin_arm64}")"
linux_amd64_sha="$(sha_for "${linux_amd64}")"
linux_arm64_sha="$(sha_for "${linux_arm64}")"

git clone "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/rtcoder/homebrew-tap.git" "${tap_dir}"
mkdir -p "${tap_dir}/Formula"

cat > "${tap_dir}/Formula/ytrack.rb" <<EOF
class Ytrack < Formula
  desc "YouTrack CLI with global and per-project configuration"
  homepage "https://github.com/rtcoder/ytrack"
  license "MIT"
  version "${version}"
  head "https://github.com/rtcoder/ytrack.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/rtcoder/ytrack/releases/download/v${version}/${darwin_arm64}"
      sha256 "${darwin_arm64_sha}"
    else
      url "https://github.com/rtcoder/ytrack/releases/download/v${version}/${darwin_amd64}"
      sha256 "${darwin_amd64_sha}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/rtcoder/ytrack/releases/download/v${version}/${linux_arm64}"
      sha256 "${linux_arm64_sha}"
    else
      url "https://github.com/rtcoder/ytrack/releases/download/v${version}/${linux_amd64}"
      sha256 "${linux_amd64_sha}"
    end
  end

  def install
    bin.install "ytrack"
  end

  test do
    assert_match "Manage YouTrack issues", shell_output("#{bin}/ytrack --help")
  end
end
EOF

cd "${tap_dir}"
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git add Formula/ytrack.rb

if git diff --cached --quiet; then
  exit 0
fi

git commit -m "Update ytrack to ${version}"
git push origin main
