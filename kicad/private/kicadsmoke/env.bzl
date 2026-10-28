# SPDX-License-Identifier: Apache-2.0
"""A rule that writes the fixed environment of the KiCad toolchain."""

_KICAD = Label("//kicad:toolchain_type")

def _kicad_env_impl(ctx):
    env = ctx.toolchains[_KICAD].env
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    ctx.actions.write(out, "".join(["%s=%s\n" % (k, env[k]) for k in sorted(env.keys())]))
    return [DefaultInfo(files = depset([out]))]

kicad_env = rule(
    implementation = _kicad_env_impl,
    doc = "Writes NAME=VALUE for each variable the resolved KiCad toolchain sets.",
    toolchains = [_KICAD],
)
