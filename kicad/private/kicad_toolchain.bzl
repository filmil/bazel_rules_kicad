# SPDX-License-Identifier: Apache-2.0
"""The KiCad toolchain: kicad-cli, its data, and its environment."""

KicadToolchainInfo = provider(
    doc = "What a KiCad toolchain offers beyond the `kicad_cli` tool.",
    fields = {
        "version": "The KiCad version kicad-cli reports, such as \"9.0.9\".",
        "major": "The major version as a string, such as \"9\". KiCad's " +
                 "library variables are named after it (KICAD9_SYMBOL_DIR).",
    },
)

def _kicad_toolchain_impl(ctx):
    cli = ctx.attr.kicad_cli[DefaultInfo].files_to_run
    if cli == None or cli.executable == None:
        fail("kicad_cli: %s is not executable" % ctx.attr.kicad_cli.label)
    major = ctx.attr.version.split(".")[0]
    return [platform_common.ToolchainInfo(
        tools = {"kicad_cli": cli},
        tool_inputs = depset(ctx.files.data + ctx.files.fonts + [ctx.file.fonts_conf]),
        env = {
            # UTF-8 file names and text whatever the host's locale is. glibc
            # has C.UTF-8 built in, so no locale files are read.
            "LC_ALL": "C.UTF-8",
            "LANG": "C.UTF-8",
            # UTC, whatever the host's /etc/localtime says. KiCad writes
            # local time into its outputs: without this, a DRC report made
            # in CET just after midnight carries the next day's date. UTC0
            # is a POSIX TZ string, so glibc reads no zone file for it.
            "TZ": "UTC0",
        },
        path_env = {
            # Only the fonts of this toolchain; no host font directory.
            "FONTCONFIG_FILE": ctx.file.fonts_conf,
        },
        home_env = [
            "KICAD_CONFIG_HOME",
            "KICAD_DOCUMENTS_HOME",
            "KICAD_CACHE_HOME",
        ],
        kicad = KicadToolchainInfo(version = ctx.attr.version, major = major),
    )]

kicad_toolchain = rule(
    implementation = _kicad_toolchain_impl,
    doc = """A KiCad toolchain, for the toolchain type `@hdlf_rules_kicad//kicad:toolchain_type`.

The `kicad` module extension makes one from the prebuilt KiCad. A user who
builds or fetches KiCad another way declares their own:

```starlark
load("@hdlf_rules_kicad//kicad:defs.bzl", "kicad_toolchain")

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
    target_settings = ["@hdlf_rules_kicad//kicad/toolchains:stub_off"],
    toolchain = ":my_kicad_impl",
    toolchain_type = "@hdlf_rules_kicad//kicad:toolchain_type",
)
```

Every action runs kicad-cli through `run_tool`, which gives it a private
home: `KICAD_CONFIG_HOME`, `KICAD_DOCUMENTS_HOME`, `KICAD_CACHE_HOME` and
the XDG directories all point inside the action (spec H3), and
`FONTCONFIG_FILE` names `fonts_conf`.
""",
    attrs = {
        "kicad_cli": attr.label(
            doc = "The kicad-cli executable.",
            mandatory = True,
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
        "data": attr.label_list(
            doc = "Every file kicad-cli reads: its libraries and data.",
            allow_files = True,
            cfg = "exec",
        ),
        "fonts": attr.label_list(
            doc = "The font files that `fonts_conf` names.",
            allow_files = True,
            cfg = "exec",
        ),
        "fonts_conf": attr.label(
            doc = "A fontconfig file that names only `fonts` (spec H3: no host fonts).",
            mandatory = True,
            allow_single_file = True,
            cfg = "exec",
        ),
        "version": attr.string(
            doc = "The KiCad version kicad-cli reports, such as \"9.0.9\".",
            mandatory = True,
        ),
    },
)

def _kicad_stub_toolchain_impl(ctx):
    return [platform_common.ToolchainInfo(
        tools = {"kicad_cli": ctx.attr.stub[DefaultInfo].files_to_run},
        kicad = KicadToolchainInfo(
            version = ctx.attr.version,
            major = ctx.attr.version.split(".")[0],
        ),
    )]

kicad_stub_toolchain = rule(
    implementation = _kicad_stub_toolchain_impl,
    doc = """swap_test's KiCad toolchain: the stub stands in for kicad-cli.

It offers the same `KicadToolchainInfo` as the real toolchain, so a rule
that reads it analyzes the same way under the stub.
""",
    attrs = {
        "stub": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "version": attr.string(mandatory = True),
    },
)
