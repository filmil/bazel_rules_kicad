# SPDX-License-Identifier: Apache-2.0
"""kicad_sch_pdf: a schematic as one PDF, a page per sheet."""

load(":design.bzl", "KicadSchematicInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_sch_pdf_impl(ctx):
    sch = ctx.attr.schematic[KicadSchematicInfo]
    raw = ctx.actions.declare_file(ctx.label.name + ".raw.pdf")
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = ["sch", "export", "pdf", "--output", raw.path, sch.root.path],
        inputs = sch.files,
        outputs = [raw],
        mnemonic = "KicadSchPdf",
        progress_message = "Plotting %{label} to PDF",
    )
    out = ctx.actions.declare_file(ctx.label.name + ".pdf")
    normalize(ctx, raw, out)
    return [DefaultInfo(files = depset([out]))]

kicad_sch_pdf = rule(
    implementation = _kicad_sch_pdf_impl,
    doc = """A schematic as one PDF, a page per sheet, in KiCad's colors.

```starlark
kicad_sch_pdf(
    name = "sch_pdf",
    schematic = ":sch",
)
```

kicad-cli plots the schematic's staged project. The PDF's creation date is
removed (spec D2), so the same schematic gives the same bytes.
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
