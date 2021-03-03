let nixpkgs = builtins.fetchGit {
      url = "https://github.com/NixOS/nixpkgs.git";
      ref = "master";
      rev = "80308388cd77ee58823c9b6f24b46892cd359145";
    };
    pkgs = import nixpkgs {};
in  pkgs.mkShell {
  hardeningDisable = [ "all" ];
  buildInputs = [ pkgs.go_1_16 ];
  shellHook = ''
    if [[ -z "$THROTTLESOCKS_GOPATH" ]]; then
      export GOPATH="$(pwd)/.go"
      export GOBIN="$GOPATH/bin"
      mkdir -p "$GOBIN"
      export THROTTLESOCKS_GOPATH=$GOPATH
    fi
  '';
}
