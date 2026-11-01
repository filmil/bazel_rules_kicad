# SPDX-License-Identifier: Apache-2.0
"""The Freerouting toolchain: Freerouting's jar and the Java runtime it runs on."""

load("@rules_java//java/common:java_common.bzl", "java_common")

FreeroutingToolchainInfo = provider(
    doc = "What a Freerouting toolchain offers beyond its `java` tool.",
    fields = {
        "jar": "File: Freerouting's jar, which `java -jar` runs.",
        "version": "string: Freerouting's version, such as \"2.5.0\".",
    },
)

def _freerouting_toolchain_impl(ctx):
    rt = ctx.attr.java_runtime[java_common.JavaRuntimeInfo]
    java = [f for f in rt.files.to_list() if f.path == rt.java_executable_exec_path]
    if len(java) != 1:
        fail("java_runtime: %s has no java executable at %s" % (ctx.attr.java_runtime.label, rt.java_executable_exec_path))
    return [platform_common.ToolchainInfo(
        tools = {"java": java[0]},
        tool_inputs = depset([ctx.file.jar], transitive = [rt.files]),
        env = {
            "LC_ALL": "C.UTF-8",
            "LANG": "C.UTF-8",
            "TZ": "UTC0",
        },
        freerouting = FreeroutingToolchainInfo(jar = ctx.file.jar, version = ctx.attr.version),
    )]

freerouting_toolchain = rule(
    implementation = _freerouting_toolchain_impl,
    doc = """A Freerouting toolchain, for `@hdlf_rules_kicad//kicad:freerouting_toolchain_type`.

hdlf_rules_kicad registers one: Freerouting's jar, pinned by sha256, on
rules_java's JDK 25. A user who wants another declares their own:

```starlark
load("@hdlf_rules_kicad//kicad:defs.bzl", "freerouting_toolchain")

freerouting_toolchain(
    name = "my_freerouting_impl",
    jar = "@my_freerouting//file",
    java_runtime = "@remotejdk25_linux//:jdk",
    version = "2.5.0",
)

toolchain(
    name = "my_freerouting",
    exec_compatible_with = ["@platforms//os:linux", "@platforms//cpu:x86_64"],
    target_settings = ["@hdlf_rules_kicad//kicad/toolchains:stub_off"],
    toolchain = ":my_freerouting_impl",
    toolchain_type = "@hdlf_rules_kicad//kicad:freerouting_toolchain_type",
)
```
""",
    attrs = {
        "jar": attr.label(
            doc = "Freerouting's jar.",
            mandatory = True,
            allow_single_file = [".jar"],
            cfg = "exec",
        ),
        "java_runtime": attr.label(
            doc = "The Java runtime (a `java_runtime`) that runs the jar.",
            mandatory = True,
            providers = [java_common.JavaRuntimeInfo],
            cfg = "exec",
        ),
        "version": attr.string(
            doc = "Freerouting's version, such as \"2.5.0\".",
            mandatory = True,
        ),
    },
)

def _freerouting_stub_toolchain_impl(ctx):
    stub = ctx.attr.stub[DefaultInfo].files_to_run
    return [platform_common.ToolchainInfo(
        tools = {"java": stub},
        freerouting = FreeroutingToolchainInfo(jar = stub.executable, version = ctx.attr.version),
    )]

freerouting_stub_toolchain = rule(
    implementation = _freerouting_stub_toolchain_impl,
    doc = "swap_test's Freerouting toolchain: the stub stands in for java.",
    attrs = {
        "stub": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "version": attr.string(mandatory = True),
    },
)
