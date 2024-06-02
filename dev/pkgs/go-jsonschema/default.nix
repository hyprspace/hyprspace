{
  lib,
  buildGoModule,
  fetchFromGitHub,
}:

buildGoModule rec {
  pname = "go-jsonschema";
  version = "0.16.0";

  src = fetchFromGitHub {
    owner = "omissis";
    repo = "go-jsonschema";
    rev = "v${version}";
    hash = "sha256-+CapTmg4RObK6mzjAS/EFbX4s2AtQvlFXmT119aUkZA=";
  };

  vendorHash = "sha256-gk+aKGqcHEjuYxc2o+83HA2AxU+jT7URt0N/q+uyUtA=";

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
