# Homebrew Formula for Kolang
# Install with: brew install faralidev/tap/kolang
# Or: brew tap faralidev/tap && brew install kolang

class Kolang < Formula
  desc "Persian programming language interpreter"
  homepage "https://github.com/faralidev/kolang"
  url "https://github.com/faralidev/kolang/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "REPLACE_WITH_ACTUAL_SHA256"
  license "MIT"
  head "https://github.com/faralidev/kolang.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/kolang"
  end

  test do
    assert_equal "سلام\n", shell_output("#{bin}/kolang -c '«سلام» بنویس'")
  end
end
