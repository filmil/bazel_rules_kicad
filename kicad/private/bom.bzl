# SPDX-License-Identifier: Apache-2.0
"""kicad_bom: a schematic's bill of materials, as CSV."""

load(":design.bzl", "KicadSchematicInfo")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_bom_impl(ctx):
    if len(ctx.attr.fields) != len(ctx.attr.labels):
        fail("%s: %d fields but %d labels; give one label per field" % (ctx.label, len(ctx.attr.fields), len(ctx.attr.labels)))
    if not ctx.attr.fields:
        fail("%s: fields is empty" % ctx.label)
    for f in ctx.attr.fields + ctx.attr.labels + ctx.attr.group_by:
        if "," in f:
            fail("%s: %r has a comma, which kicad-cli takes as a list separator" % (ctx.label, f))
    sch = ctx.attr.schematic[KicadSchematicInfo]
    base = ctx.label.name[:-len(".csv")] if ctx.label.name.endswith(".csv") else ctx.label.name
    out = ctx.actions.declare_file(base + ".csv")

    # Every option is given, so a change in kicad-cli's defaults cannot
    # change the columns that rules_fab reads.
    args = [
        "sch",
        "export",
        "bom",
        "--fields",
        ",".join(ctx.attr.fields),
        "--labels",
        ",".join(ctx.attr.labels),
        "--group-by",
        ",".join(ctx.attr.group_by),
        "--sort-field",
        ctx.attr.sort_field,
        # A flag with no value: kicad-cli 9.0.9 crashes on "--sort-asc true"
        # (bad any_cast, exit 255).
        "--sort-asc",
        "--field-delimiter",
        ",",
        "--string-delimiter",
        "\"",
        "--ref-delimiter",
        ",",
        "--ref-range-delimiter",
        "",
    ]
    if ctx.attr.exclude_dnp:
        args.append("--exclude-dnp")
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = args + ["--output", out.path, sch.root.path],
        inputs = sch.files,
        outputs = [out],
        mnemonic = "KicadBom",
        progress_message = "Writing the bill of materials of %{label}",
    )
    return [DefaultInfo(files = depset([out]))]

kicad_bom = rule(
    implementation = _kicad_bom_impl,
    doc = """A schematic's bill of materials, as CSV.

```starlark
kicad_bom(
    name = "bom",
    schematic = ":sch",
)
```

The output is `<name>.csv` (or `<name>` when the name already ends in
`.csv`). The first row holds the labels; each other row is a group of
parts with the same values of `group_by`, sorted by `sort_field`. Fields
are quoted with `"`, separated with `,`, and the references of a group are
listed one by one (`R1,R2,R3`), never as a range. Every option is passed
to kicad-cli, so the columns do not change with its defaults.

`fields` takes symbol fields (`Reference`, `Value`, `Footprint`, or any
field of the parts) and kicad-cli's own: `${QUANTITY}`, `${DNP}`,
`${ITEM_NUMBER}`. Parts marked "exclude from BOM" are left out, and so,
with `exclude_dnp`, are parts marked "do not populate". The output has no
time stamp.
""",
    attrs = {
        "schematic": attr.label(
            doc = "The schematic (`kicad_schematic`).",
            mandatory = True,
            providers = [KicadSchematicInfo],
        ),
        "fields": attr.string_list(
            doc = "The columns, in order: symbol fields or kicad-cli's ${...} fields.",
            default = ["Reference", "Value", "Footprint", "${QUANTITY}", "${DNP}"],
        ),
        "labels": attr.string_list(
            doc = "The header of each column, in the order of `fields`.",
            default = ["Refs", "Value", "Footprint", "Qty", "DNP"],
        ),
        "group_by": attr.string_list(
            doc = "Parts with the same values of these fields share a row. Empty: one row per part.",
            default = ["Value", "Footprint"],
        ),
        "sort_field": attr.string(
            doc = "The field the rows are sorted by, ascending.",
            default = "Reference",
        ),
        "exclude_dnp": attr.bool(
            doc = "Leave out parts marked \"do not populate\".",
            default = False,
        ),
    },
    toolchains = [_KICAD] + RUN_TOOL_TOOLCHAINS,
)
