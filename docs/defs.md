<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of rules_kicad.

Load every rule from this file:

```starlark
load("@rules_kicad//kicad:defs.bzl", "determinism_test", "swap_test", "tool_toolchain")
```

The rules arrive in milestones M1 and M2 (see the issues of
https://github.com/filmil/bazel_rules_kicad). This file is the only public
entry point; everything under `//kicad/private` may change without notice.

<a id="kicad_toolchain"></a>

## kicad_toolchain

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_toolchain")

kicad_toolchain(<a href="#kicad_toolchain-name">name</a>, <a href="#kicad_toolchain-data">data</a>, <a href="#kicad_toolchain-fonts">fonts</a>, <a href="#kicad_toolchain-fonts_conf">fonts_conf</a>, <a href="#kicad_toolchain-kicad_cli">kicad_cli</a>, <a href="#kicad_toolchain-version">version</a>)
</pre>

A KiCad toolchain, for the toolchain type `@rules_kicad//kicad:toolchain_type`.

The `kicad` module extension makes one from the prebuilt KiCad. A user who
builds or fetches KiCad another way declares their own:

```starlark
load("@rules_kicad//kicad:defs.bzl", "kicad_toolchain")

kicad_toolchain(
    name = "my_kicad_impl",
    data = ["@my_kicad//:all_files"],
    fonts = ["@my_fonts//:ttf"],
    fonts_conf = "fonts.conf",
    kicad_cli = "@my_kicad//:bin/kicad-cli",
    version = "9.0.9",
)

toolchain(
    name = "my_kicad",
    exec_compatible_with = ["@platforms//os:linux", "@platforms//cpu:x86_64"],
    target_settings = ["@rules_kicad//kicad/toolchains:stub_off"],
    toolchain = ":my_kicad_impl",
    toolchain_type = "@rules_kicad//kicad:toolchain_type",
)
```

Every action runs kicad-cli through `run_tool`, which gives it a private
home: `KICAD_CONFIG_HOME`, `KICAD_DOCUMENTS_HOME`, `KICAD_CACHE_HOME` and
the XDG directories all point inside the action (spec H3), and
`FONTCONFIG_FILE` names `fonts_conf`.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_toolchain-data"></a>data |  Every file kicad-cli reads: its libraries and data.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="kicad_toolchain-fonts"></a>fonts |  The font files that `fonts_conf` names.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="kicad_toolchain-fonts_conf"></a>fonts_conf |  A fontconfig file that names only `fonts` (spec H3: no host fonts).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="kicad_toolchain-kicad_cli"></a>kicad_cli |  The kicad-cli executable.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="kicad_toolchain-version"></a>version |  The KiCad version kicad-cli reports, such as "9.0.9".   | String | required |  |


<a id="tool_toolchain"></a>

## tool_toolchain

<pre>
load("@rules_kicad//kicad:defs.bzl", "tool_toolchain")

tool_toolchain(<a href="#tool_toolchain-name">name</a>, <a href="#tool_toolchain-tools">tools</a>)
</pre>

A toolchain that offers tools by name, for `run_tool`.

Example, a toolchain that offers a user's own build of kicad-cli for the
swap test or a custom rule. For the KiCad toolchain type, `kicad_toolchain`
also brings KiCad's data, fonts and environment:

```starlark
tool_toolchain(
    name = "my_kicad_impl",
    tools = {"//third_party/kicad:kicad_cli": "kicad_cli"},
)

toolchain(
    name = "my_kicad",
    # So that swap_test can put the stub in its place.
    target_settings = ["@rules_kicad//kicad/toolchains:stub_off"],
    toolchain = ":my_kicad_impl",
    toolchain_type = "//my:toolchain_type",
)
```

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="tool_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="tool_toolchain-tools"></a>tools |  Executable targets, each mapped to the name rules use for it.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: Label -> String</a> | required |  |


<a id="KicadToolchainInfo"></a>

## KicadToolchainInfo

<pre>
load("@rules_kicad//kicad:defs.bzl", "KicadToolchainInfo")

KicadToolchainInfo(<a href="#KicadToolchainInfo-version">version</a>, <a href="#KicadToolchainInfo-major">major</a>)
</pre>

What a KiCad toolchain offers beyond the `kicad_cli` tool.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="KicadToolchainInfo-version"></a>version |  The KiCad version kicad-cli reports, such as "9.0.9".    |
| <a id="KicadToolchainInfo-major"></a>major |  The major version as a string, such as "9". KiCad's library variables are named after it (KICAD9_SYMBOL_DIR).    |


<a id="determinism_test"></a>

## determinism_test

<pre>
load("@rules_kicad//kicad:defs.bzl", "determinism_test")

determinism_test(<a href="#determinism_test-name">name</a>, <a href="#determinism_test-target">target</a>, <a href="#determinism_test-kwargs">**kwargs</a>)
</pre>

Checks that two builds of `target` give byte-identical outputs.

The target is built twice in one build, under two configurations that
differ only in a setting no rule reads, so every action runs twice.
The test compares the SHA-256 of every output file, directories walked,
and fails on any difference (spec D1).

Two runs in the same second can hide a time stamp with a one-second
resolution, so pair this with a header scan where a format has one. CI
also builds every determinism_test target in two fresh output bases and
compares the results.

Example:

```starlark
determinism_test(
    name = "gerbers_determinism_test",
    target = ":gerbers",
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="determinism_test-name"></a>name |  the test's name.   |  none |
| <a id="determinism_test-target"></a>target |  the target whose outputs must not change between builds.   |  none |
| <a id="determinism_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


<a id="swap_test"></a>

## swap_test

<pre>
load("@rules_kicad//kicad:defs.bzl", "swap_test")

swap_test(<a href="#swap_test-name">name</a>, <a href="#swap_test-targets">targets</a>, <a href="#swap_test-kwargs">**kwargs</a>)
</pre>

Builds targets with every tool swapped for a stub, and checks them.

The targets are built with `--@rules_kicad//kicad/toolchains:use_stub`,
so each tool toolchain that honors the flag resolves to the stub tool.
The stub writes a record into every output, files and directories
alike. The test fails if any output file of the targets lacks the
record: some action made it without a toolchain tool.

Example:

```starlark
swap_test(
    name = "swap_test",
    targets = [":gerbers", ":bom"],
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="swap_test-name"></a>name |  the test's name.   |  none |
| <a id="swap_test-targets"></a>targets |  targets whose outputs must all come from the stub.   |  none |
| <a id="swap_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


