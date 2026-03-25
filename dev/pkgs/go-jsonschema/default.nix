{
  lib,
  buildGoModule,
  fetchFromGitHub,
}:

buildGoModule rec {
  pname = "go-jsonschema";
  version = "0.22.0";

  src = fetchFromGitHub {
    owner = "omissis";
    repo = "go-jsonschema";
    rev = "v${version}";
    hash = "sha256-ffrP4L5cfK75Tw/xfcdXAwGUP8WLL+81ltBDb/P5Gwo=";
  };

  env.GOWORK = "off";

  vendorHash = "sha256-mCOJ8GROrbNXH7CSLLMZj/4wTa65hscTt8RzIxzgG+A=";

  ldflags = [
    "-s"
    "-w"
    "-X=main.version=${version}"
    "-X=main.gitCommit=${src.rev}"
    "-X=main.buildTime=1970-01-01T00:00:00Z"
  ];

  subPackages = [ "." ];
  meta = with lib; {
    description = "A tool to generate Go data types from JSON Schema definitions";
    homepage = "https://github.com/omissis/go-jsonschema";
    license = licenses.mit;
    mainProgram = "go-jsonschema";
  };
}
