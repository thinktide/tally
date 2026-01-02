# This is a template formula for homebrew-tally tap
# Copy this to your homebrew-tally repository and update:
# 1. Replace thinktide with your GitHub username
# 2. Update VERSION and SHA256 values after running goreleaser
#
# Users will install with:
#   brew tap thinktide/tally
#   brew install tally

class Tally < Formula
  desc "CLI time tracking utility"
  homepage "https://github.com/thinktide/tally"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/thinktide/tally/releases/download/v#{version}/tally_#{version}_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_ARM64_SHA256"
    else
      url "https://github.com/thinktide/tally/releases/download/v#{version}/tally_#{version}_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_AMD64_SHA256"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/thinktide/tally/releases/download/v#{version}/tally_#{version}_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_LINUX_ARM64_SHA256"
    else
      url "https://github.com/thinktide/tally/releases/download/v#{version}/tally_#{version}_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_LINUX_AMD64_SHA256"
    end
  end

  def install
    bin.install "tally"
  end

  test do
    system "#{bin}/tally", "version"
  end
end
