# from https://git.clan.lol/clan/clan-core/raw/commit/0b34c340fc502235af25d103b2b83948b4ad925e/lib/jsonschema/default.nix

{
  lib ? import <nixpkgs/lib>,
  excludedTypes ? [
    "functionTo"
    "package"
  ],
  includeDefaults ? true,
  header ? {
    "$schema" = "http://json-schema.org/draft-07/schema#";
  },
  specialArgs ? { },
}:
let
  # remove _module attribute from options
  clean = opts: builtins.removeAttrs opts [ "_module" ];

  # throw error if option type is not supported
  notSupported =
    option:
    lib.trace option throw ''
      option type '${option.type.name}' ('${option.type.description}') not supported by jsonschema converter
      location: ${lib.concatStringsSep "." option.loc}
    '';

  # Exclude the option if its type is in the excludedTypes list
  # or if the option has a defaultText attribute
  isExcludedOption = option: (lib.elem (option.type.name or null) excludedTypes);

  filterExcluded = lib.filter (opt: !isExcludedOption opt);

  excludedOptionNames = [ "_freeformOptions" ];
  filterExcludedAttrs = lib.filterAttrs (
    name: opt: !isExcludedOption opt && !builtins.elem name excludedOptionNames
  );

  # Filter out options where the visible attribute is set to false
  filterInvisibleOpts = lib.filterAttrs (_name: opt: opt.visible or true);

  # Constant: Used for the 'any' type
  allBasicTypes = [
    "boolean"
    "integer"
    "number"
    "string"
    "array"
    "object"
    "null"
  ];
in
rec {
  # parses a nixos module to a jsonschema
  parseModule =
    module:
    let
      evaled = lib.evalModules {
        modules = [ module ];
        inherit specialArgs;
      };
    in
    parseOptions evaled.options { };

  # get default value from option

  # Returns '{ default = Value; }'
  # - '{}' if no default is present.
  # - Value is "<thunk>" (string literal) if the option has a defaultText attribute. This means we cannot evaluate default safely
  getDefaultFrom =
    opt:
    if !includeDefaults then
      { }
    else if opt ? defaultText then
      {
        # dont add default to jsonschema. It seems to alter the type
        # default = "<thunk>";
      }
    else
      lib.optionalAttrs (opt ? default) {
        default = opt.default;
      };

  parseSubOptions =
    {
      option,
      prefix ? [ ],
    }:
    let
      subOptions = option.type.getSubOptions option.loc;
    in
    parseOptions subOptions {
      addHeader = false;
      path = option.loc ++ prefix;
    };

  makeModuleInfo =
    {
      path,
      defaultText ? null,
    }:
    {
      "$exportedModuleInfo" =
        {
          inherit path;
        }
        // lib.optionalAttrs (defaultText != null) {
          inherit defaultText;
        };
    };

  # parses a set of evaluated nixos options to a jsonschema
  parseOptions =
    options:
    {
      # The top-level header object should specify at least the schema version
      # Can be customized if needed
      # By default the header is not added to the schema
      addHeader ? true,
      path ? [ ],
    }:
    let
      options' = filterInvisibleOpts (filterExcludedAttrs (clean options));
      # parse options to jsonschema properties
      properties = lib.mapAttrs (_name: option: (parseOption' (path ++ [ _name ]) option)) options';
      # TODO: figure out how to handle if prop.anyOf is used
      isRequired = prop: !(prop ? default || prop.type or null == "object");
      requiredProps = lib.filterAttrs (_: prop: isRequired prop) properties;
      required = lib.optionalAttrs (requiredProps != { }) { required = lib.attrNames requiredProps; };
      header' = if addHeader then header else { };

      # freeformType is a special type
      freeformDefs = options._module.freeformType.definitions or [ ];
      checkFreeformDefs =
        defs:
        if (builtins.length defs) != 1 then
          throw "parseOptions: freeformType definitions not supported"
        else
          defs;
      # It seems that freeformType has [ null ]
      freeformProperties =
        if freeformDefs != [ ] && builtins.head freeformDefs != null then
          # freeformType has only one definition
          parseOption {
            # options._module.freeformType.definitions
            type = builtins.head (checkFreeformDefs freeformDefs);
            _type = "option";
            loc = path;
          }
        else
          { };

      # Metadata about the module that is made available to the schema via '$propagatedModuleInfo'
      exportedModuleInfo = lib.optionalAttrs true (makeModuleInfo {
        inherit path;
      });
    in
    # return jsonschema
    header'
    // exportedModuleInfo
    // required
    // {
      type = "object";
      inherit properties;
      additionalProperties = false;
    }
    // freeformProperties;

  # parses and evaluated nixos option to a jsonschema property definition
  parseOption = parseOption' [ ];
  parseOption' =
    currentPath: option:
    let
      default = getDefaultFrom option;
      example = lib.optionalAttrs (option ? example) {
        examples =
          if (builtins.typeOf option.example) == "list" then option.example else [ option.example ];
      };
      description = lib.optionalAttrs (option ? description) {
        description = option.description.text or option.description;
      };
      exposedModuleInfo = makeModuleInfo {
        path = option.loc;
        defaultText = option.defaultText or null;
      };
    in
    # either type
    # TODO: if all nested options are excluded, the parent should be excluded too
    if
      option.type.name or null == "either" || option.type.name or null == "coercedTo"
    # return jsonschema property definition for either
    then
      let
        optionsList' = [
          {
            type = option.type.nestedTypes.left or option.type.nestedTypes.coercedType;
            _type = "option";
            loc = option.loc;
          }
          {
            type = option.type.nestedTypes.right or option.type.nestedTypes.finalType;
            _type = "option";
            loc = option.loc;
          }
        ];
        optionsList = filterExcluded optionsList';
      in
      exposedModuleInfo // default // example // description // { oneOf = map parseOption optionsList; }
    # handle nested options (not a submodule)
    # foo.bar = mkOption { type = str; };
    else if !option ? _type then
      (parseOptions option {
        addHeader = false;
        path = currentPath;
      })
    # throw if not an option
    else if option._type != "option" && option._type != "option-type" then
      throw "parseOption: not an option"
    # parse nullOr
    else if
      option.type.name == "nullOr"
    # return jsonschema property definition for nullOr
    then
      let
        nestedOption = {
          type = option.type.nestedTypes.elemType;
          _type = "option";
          loc = option.loc;
        };
      in
      default
      // exposedModuleInfo
      // example
      // description
      // {
        oneOf = [
          { type = "null"; }
        ] ++ (lib.optional (!isExcludedOption nestedOption) (parseOption nestedOption));
      }
    # parse bool
    else if
      option.type.name == "bool"
    # return jsonschema property definition for bool
    then
      exposedModuleInfo // default // example // description // { type = "boolean"; }
    # parse float
    else if
      option.type.name == "float"
    # return jsonschema property definition for float
    then
      exposedModuleInfo // default // example // description // { type = "number"; }
    # parse int
    else if
      (option.type.name == "int" || option.type.name == "positiveInt")
    # return jsonschema property definition for int
    then
      exposedModuleInfo // default // example // description // { type = "integer"; }
    # TODO: Add support for intMatching in jsonschema
    # parse port type aka. "unsignedInt16"
    else if
      option.type.name == "unsignedInt16"
      || option.type.name == "unsignedInt"
      || option.type.name == "pkcs11"
      || option.type.name == "intBetween"
    then
      exposedModuleInfo // default // example // description // { type = "integer"; }
    # parse string
    # TODO: parse more precise string types
    else if
      option.type.name == "str"
      || option.type.name == "singleLineStr"
      || option.type.name == "passwdEntry str"
      || option.type.name == "passwdEntry path"
    # return jsonschema property definition for string
    then
      exposedModuleInfo // default // example // description // { type = "string"; }
    # TODO: Add support for stringMatching in jsonschema
    # parse stringMatching
    else if lib.strings.hasPrefix "strMatching" option.type.name then
      exposedModuleInfo // default // example // description // { type = "string"; }
    # TODO: Add support for separatedString in jsonschema
    else if lib.strings.hasPrefix "separatedString" option.type.name then
      exposedModuleInfo // default // example // description // { type = "string"; }
    # parse string
    else if
      option.type.name == "path"
    # return jsonschema property definition for path
    then
      exposedModuleInfo // default // example // description // { type = "string"; }
    # parse anything
    else if
      option.type.name == "anything"
    # return jsonschema property definition for anything
    then
      exposedModuleInfo // default // example // description // { type = allBasicTypes; }
    # parse unspecified
    else if
      option.type.name == "unspecified"
    # return jsonschema property definition for unspecified
    then
      exposedModuleInfo // default // example // description // { type = allBasicTypes; }
    # parse raw
    else if
      option.type.name == "raw"
    # return jsonschema property definition for raw
    then
      exposedModuleInfo // default // example // description // { type = allBasicTypes; }
    # parse enum
    else if
      option.type.name == "enum"
    # return jsonschema property definition for enum
    then
      exposedModuleInfo // default // example // description // { enum = option.type.functor.payload; }
    # parse listOf submodule
    else if
      option.type.name == "listOf" && option.type.nestedTypes.elemType.name == "submodule"
    # return jsonschema property definition for listOf submodule
    then
      default
      // exposedModuleInfo
      // example
      // description
      // {
        type = "array";
        items = parseSubOptions { inherit option; };
      }
    # parse list
    else if
      (option.type.name == "listOf")
    # return jsonschema property definition for list
    then
      let
        nestedOption = {
          type = option.type.nestedTypes.elemType;
          _type = "option";
          loc = option.loc;
        };
      in
      default
      // exposedModuleInfo
      // example
      // description
      // {
        type = "array";
      }
      // (lib.optionalAttrs (!isExcludedOption nestedOption) { items = parseOption nestedOption; })
    # parse list of unspecified
    else if
      (option.type.name == "listOf") && (option.type.nestedTypes.elemType.name == "unspecified")
    # return jsonschema property definition for list
    then
      exposedModuleInfo // default // example // description // { type = "array"; }
    # parse attrsOf submodule
    else if
      option.type.name == "attrsOf" && option.type.nestedTypes.elemType.name == "submodule"
    # return jsonschema property definition for attrsOf submodule
    then
      default
      // exposedModuleInfo
      // example
      // description
      // {
        type = "object";
        additionalProperties = parseSubOptions {
          inherit option;
          prefix = [ "<name>" ];
        };
      }
    # parse attrs
    else if
      option.type.name == "attrs"
    # return jsonschema property definition for attrs
    then
      default
      // exposedModuleInfo
      // example
      // description
      // {
        type = "object";
        additionalProperties = true;
      }
    # parse attrsOf
    # TODO: if nested option is excluded, the parent should be excluded too
    else if
      option.type.name == "attrsOf" || option.type.name == "lazyAttrsOf"
    # return jsonschema property definition for attrs
    then
      let
        nestedOption = {
          type = option.type.nestedTypes.elemType;
          _type = "option";
          loc = option.loc;
        };
      in
      default
      // exposedModuleInfo
      // example
      // description
      // {
        type = "object";
        additionalProperties =
          if !isExcludedOption nestedOption then
            parseOption {
              type = option.type.nestedTypes.elemType;
              _type = "option";
              loc = option.loc;
            }
          else
            false;
      }
    # parse submodule
    else if
      option.type.name == "submodule"
    # return jsonschema property definition for submodule
    # then (lib.attrNames (option.type.getSubOptions option.loc).opt)
    then
      exposedModuleInfo // example // description // parseSubOptions { inherit option; }
    # throw error if option type is not supported
    else
      notSupported option;
}
