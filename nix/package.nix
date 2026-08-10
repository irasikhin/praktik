{ lib
, buildGoModule
, makeWrapper
, git
, fzf
}:

let
  pname = "praktik";
  version = "0.1.2";
in
buildGoModule {
  inherit pname version;

  src = lib.cleanSource ./..;

  vendorHash = "sha256-OOnhkv4fbTmK+HbUS0Fgzmn3GaKMBFM+wA/59+cPLqo=";

  subPackages = [ "cmd/praktik" ];

  ldflags = [ "-s" "-w" "-X" "main.version=v${version}" ];

  doCheck = true;

  nativeBuildInputs = [ makeWrapper ];

  # praktik shells out to git (clone-on-demand source, planned) and fzf (the
  # interactive picker, planned); put them on PATH.
  postInstall = ''
    wrapProgram $out/bin/praktik \
      --prefix PATH : ${lib.makeBinPath [ git fzf ]}

    mkdir -p $out/share/bash-completion/completions
    $out/bin/praktik completion bash > $out/share/bash-completion/completions/praktik

    mkdir -p $out/share/zsh/site-functions
    $out/bin/praktik completion zsh > $out/share/zsh/site-functions/_praktik

    mkdir -p $out/share/fish/vendor_completions.d
    $out/bin/praktik completion fish > $out/share/fish/vendor_completions.d/praktik.fish
  '';

  meta = {
    description = "CLI for the praktik skill library: list, show, validate, vendor, sync";
    mainProgram = "praktik";
    license = lib.licenses.mit;
    platforms = lib.platforms.unix;
  };
}
