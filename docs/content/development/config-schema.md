# Configuration Schema

The schema for the configuration file is defined in `nixos/settings.nix`. This schema is used for
- generating Go code so Hyprspace can parse the configuration
- typed configuration in the NixOS module
- generating the [config documentation](../options.html)

The conversion to Go code happens by first converting the module options to a JSON schema using [clan.lol's NixOS to JSON schema converter](https://docs.clan.lol/blog/2024/05/25/jsonschema-converter/). The JSON schema is then used to generate Go code using [go-jsonschema](https://github.com/omissis/go-jsonschema).
