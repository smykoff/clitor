{ lib
, stdenv
, fetchurl
}:

let
  inherit (stdenv.hostPlatform) system;

  sources = builtins.fromJSON (builtins.readFile ./sources.json);
  platformSrc = sources.${system}
    or (throw "clitor: unsupported system ${system}");
in
stdenv.mkDerivation {
  pname = "clitor";
  version = sources.version;

  src = fetchurl { inherit (platformSrc) url hash; };

  dontBuild = true;
  dontStrip = true; # the binary is already built with -s -w

  unpackPhase = ''
    runHook preUnpack
    mkdir src
    tar -xzf "$src" -C src
    runHook postUnpack
  '';

  installPhase = ''
    runHook preInstall
    install -Dm755 src/clitor $out/bin/clitor
    runHook postInstall
  '';

  meta = {
    description = "Small CLI that switches git identity in the current repository";
    homepage = "https://github.com/smykoff/clitor";
    platforms = builtins.attrNames (removeAttrs sources [ "version" ]);
    sourceProvenance = [ lib.sourceTypes.binaryNativeCode ];
    mainProgram = "clitor";
  };
}
