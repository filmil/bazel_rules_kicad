# SPDX-License-Identifier: Apache-2.0
"""kicad_netlist: a schematic's netlist, in KiCad's own format."""

load(":design.bzl", "KicadSchematicInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_netlist_impl(ctx):
    sch = ctx.attr.schematic[KicadSchematicInfo]

    # <name>.net, or <name> itself when it already ends in .net.
    base = ctx.label.name[:-len(".net")] if ctx.label.name.endswith(".net") else ctx.label.name
    raw = ctx.actions.declare_file(base + ".raw.net")
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = ["sch", "export", "netlist", "--format", "kicadsexpr", "--output", raw.path, sch.root.path],
        inputs = sch.files,
        outputs = [raw],
        mnemonic = "KicadNetlist",
        progress_message = "Writing the netlist of %{label}",
    )
    out = ctx.actions.declare_file(base + ".net")
    normalize(ctx, raw, out)
    return [DefaultInfo(files = depset([out]))]

kicad_netlist = rule(
    implementation = _kicad_netlist_impl,
    doc = """A schematic's netlist, in KiCad's s-expression format (`.net`).

```starlark
kicad_netlist(
    name = "netlist",
    schematic = ":sch",
)
```

The output is `<name>.net` (or `<name>` when the name already ends in
`.net`). It lists the parts (reference, value, footprint, symbol) and the
nets with the pins on each. `kicad_pcb` takes it as `netlist`, and rules_netlist
compares netlists. KiCad writes the export's date and the absolute paths
of the schematic and of its libraries into the header. The date is set
to `--@rules_kicad//kicad:source_date_epoch` (default 1970-01-01) and the
paths are cut to file names (spec D2, no time stamps in outputs), so the
same schematic gives the same bytes in any build directory.
""",
    attrs = {
        "schematic": attr.label(
            doc = "The schematic (`kicad_schematic`).",
            mandatory = True,
            providers = [KicadSchematicInfo],
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
