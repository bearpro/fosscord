{
  description = "Dev shell for Tauri v2 + SvelteKit + Go backend (+ LiveKit via docker-compose)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };

        # Runtime/libs Tauri (wry/webkit) needs on Linux
        tauriLibs = with pkgs; [
          glib
          gtk3
          webkitgtk_4_1
          libsoup_3
          libappindicator-gtk3
          libayatana-appindicator
          librsvg
          cairo
          pango
          gdk-pixbuf
          atk
          harfbuzz
        ];

        # Build tooling commonly needed by Rust crates (openssl-sys, etc.)
        buildTools = with pkgs; [
          pkg-config
          openssl
          clang
          llvmPackages.bintools
        ];

        devTools = with pkgs; [
          git
          go
          nodejs_22
          pnpm
          rustup
        ];

        libPath = pkgs.lib.makeLibraryPath (tauriLibs ++ [ pkgs.openssl ]);

      in {
        devShells.default = pkgs.mkShell {
          packages = devTools ++ buildTools ++ tauriLibs;

          # Helpful defaults for native builds
          env = {
            # Helps some crates find system libs on Nix
            LD_LIBRARY_PATH = libPath;

            # If you use Rust toolchain via rustup, keep it in project dir:
            RUSTUP_HOME = "\${PWD}/.rustup";
            CARGO_HOME = "\${PWD}/.cargo";

            # Some crates call `cc`; point it at clang to reduce surprises
            CC = "clang";
            CXX = "clang++";
          };

          shellHook = ''
            echo "Dev shell: Tauri v2 + SvelteKit + Go"
            echo "Node: $(node -v 2>/dev/null || true) | pnpm: $(pnpm -v 2>/dev/null || true) | Go: $(go version 2>/dev/null || true)"
            echo "Tip: run 'rustup toolchain install stable' once (inside this shell)."
          '';
        };
      }
    );
}
