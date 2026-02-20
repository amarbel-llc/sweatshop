{
  description = "Shell-agnostic git worktree session manager wrapping zmx";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/23d72dabcb3b12469f57b37170fcbc1789bd7457";
    utils.url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
    go.url = "github:friedenberg/eng?dir=devenvs/go";
    shell.url = "github:friedenberg/eng?dir=devenvs/shell";
    batman.url = "github:amarbel-llc/batman";
  };

  outputs =
    {
      self,
      nixpkgs,
      utils,
      go,
      shell,
      batman,
    }:
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [
            go.overlays.default
          ];
        };

        version = "0.1.0";

        sweatshop = pkgs.buildGoApplication {
          pname = "sweatshop";
          inherit version;
          src = ./.;
          modules = ./gomod2nix.toml;
          subPackages = [ "cmd/sweatshop" ];
        };

        spinclass = pkgs.runCommand "spinclass" { } ''
          mkdir -p $out/bin
          ln -s ${sweatshop}/bin/sweatshop $out/bin/spinclass
        '';

        shellCompletions = pkgs.runCommand "sweatshop-completions-files" { } ''
          install -Dm644 ${./completions/sweatshop.bash-completion} \
            $out/share/bash-completion/completions/sweatshop
          install -Dm644 ${./completions/sweatshop.fish} \
            $out/share/fish/vendor_completions.d/sweatshop.fish
          install -Dm644 ${./completions/spinclass.bash-completion} \
            $out/share/bash-completion/completions/spinclass
          install -Dm644 ${./completions/spinclass.fish} \
            $out/share/fish/vendor_completions.d/spinclass.fish
        '';
      in
      {
        packages = {
          default = pkgs.symlinkJoin {
            name = "sweatshop";
            paths = [
              sweatshop
              shellCompletions
            ];
          };

          spinclass = pkgs.symlinkJoin {
            name = "spinclass";
            paths = [
              spinclass
              shellCompletions
            ];
          };
        };

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.jq
            pkgs.just
            batman.packages.${system}.bats
            batman.packages.${system}.bats-libs
          ];

          inputsFrom = [
            go.devShells.${system}.default
            shell.devShells.${system}.default
          ];

          shellHook = ''
            echo "sweatshop - dev environment"
          '';
        };

        apps = {
          default = {
            type = "app";
            program = "${sweatshop}/bin/sweatshop";
          };

          spinclass = {
            type = "app";
            program = "${spinclass}/bin/spinclass";
          };
        };
      }
    );
}
