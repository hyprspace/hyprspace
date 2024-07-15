{ runCommand, generateSchemasProgram }:

runCommand "hyprspace-vendoring" { } ''
  mkdir -p $out/schema
  cd $out
  ${generateSchemasProgram}
''
