{
  lib,
  runCommand,
  generateSchemasProgram,
}:

runCommand "hyprspace-vendoring" { } ''
  mkdir -p $out/schema
  cd $out
  ${lib.getExe generateSchemasProgram}
''
