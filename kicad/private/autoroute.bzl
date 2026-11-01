# SPDX-License-Identifier: Apache-2.0
"""kicad_autoroute: a board's copper, routed by Freerouting."""

load(":design.bzl", "KicadPcbInfo")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")
_FREEROUTING = Label("//kicad:freerouting_toolchain_type")

def _kicad_autoroute_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    base = ctx.label.name[:-len(".kicad_pcb")] if ctx.label.name.endswith(".kicad_pcb") else ctx.label.name
    dsn = ctx.actions.declare_file(base + ".dsn")
    ses = ctx.actions.declare_file(base + ".ses")
    out = ctx.actions.declare_file(base + ".kicad_pcb")
    report = ctx.actions.declare_file(base + ".routing.md")

    # 1. The board as Specctra DSN, with the vendor's clearance.
    if ctx.attr.max_passes < 0:
        fail("%s: max_passes must be 0 (Freerouting's default) or more, not %d" % (ctx.label, ctx.attr.max_passes))

    # -I: the scripts see no PYTHON* variable, user site or current directory.
    args = ["-I", ctx.file._dsn.path, "--board", pcb.board.path, "--out", dsn.path]
    inputs = [ctx.file._dsn]
    if pcb.design_rules:
        args += ["--rules", pcb.design_rules.path]
    run_tool(
        ctx,
        _KICAD,
        "kicad_python",
        arguments = args,
        inputs = depset(inputs, transitive = [pcb.files]),
        outputs = [dsn],
        mnemonic = "KicadDsn",
        progress_message = "Exporting %{label} for Freerouting",
    )

    # 2. Freerouting, headless, on one thread, with no telemetry, no
    # analytics, no log file, console messages from warnings up, and no
    # network. Its settings go to the
    # XDG directories, which run_tool points inside the action.
    fr = ctx.toolchains[_FREEROUTING].freerouting
    fargs = [
        "-Djava.awt.headless=true",
        "-jar",
        fr.jar.path,
        "--gui.enabled=false",
        "--profile.allow_telemetry=false",
        "--usage_and_diagnostic_data.disable_analytics=true",
        "--logging.file.enabled=false",
        "--logging.console.level=WARN",
        "-de",
        dsn.path,
        "-do",
        ses.path,
        "-mt",
        "1",
    ]
    if ctx.attr.max_passes:
        fargs += ["-mp", str(ctx.attr.max_passes)]
    run_tool(
        ctx,
        _FREEROUTING,
        "java",
        arguments = fargs,
        inputs = [dsn],
        outputs = [ses],
        mnemonic = "Freerouting",
        progress_message = "Routing %{label} with Freerouting",
        block_network = True,
    )

    # 3. The routes, back into the board.
    sargs = ["-I", ctx.file._ses.path, "--board", pcb.board.path, "--ses", ses.path, "--out", out.path, "--report", report.path]
    if ctx.attr.allow_unrouted:
        sargs.append("--allow_unrouted")
    run_tool(
        ctx,
        _KICAD,
        "kicad_python",
        arguments = sargs,
        inputs = depset([ctx.file._ses, ses], transitive = [pcb.files]),
        outputs = [out, report],
        mnemonic = "KicadSes",
        progress_message = "Importing the routes of %{label}",
    )
    return [
        DefaultInfo(files = depset([out])),
        OutputGroupInfo(dsn = depset([dsn]), ses = depset([ses]), report = depset([report])),
    ]

kicad_autoroute = rule(
    implementation = _kicad_autoroute_impl,
    doc = """A board's copper, routed by Freerouting.

```starlark
kicad_pcb(
    name = "placed",
    board = "placed.kicad_pcb",     # footprints placed, nets assigned, no copper
    design_rules = ":jlcpcb_rules",  # hdlf_rules_fab's fab_rules, or a .kicad_dru
    netlist = ":netlist",
)

kicad_autoroute(
    name = "routed",
    pcb = ":placed",
)

kicad_pcb(
    name = "pcb",
    board = ":routed",              # routed.kicad_pcb
    design_rules = ":jlcpcb_rules",
    netlist = ":netlist",
)

kicad_drc_test(
    name = "drc_test",
    pcb = ":pcb",
)
```

The output is `<name>.kicad_pcb`: the board of `pcb` with Freerouting's
tracks and vias. Three actions make it:

1. KiCad's own Python exports the board as Specctra DSN (`<name>.dsn`).
   When `pcb` has design rules, every net class's clearance becomes at
   least the largest `clearance` or `edge_clearance` minimum in them, and
   its track width at least the largest `track_width` minimum, because
   Freerouting keeps one clearance. Hole clearances stay the DRC's to
   check.
2. Freerouting routes it (`<name>.ses`), headless and on one thread, with
   its telemetry, analytics and log file off, in an action without
   network where the strategy allows.
3. KiCad's Python imports the session into the board. New tracks and vias
   get UUIDs from a seeded generator, so the same inputs give the same
   board. A connection left unrouted fails the build, unless
   `allow_unrouted`. It also writes the report below.

Freerouting's result is not checked against the design rules: give the
routed board to `kicad_drc_test`, and to `kicad_render` for its images.

The output group `report` holds `<name>.routing.md`, the routing for a
person to read: per net, the track length on each copper layer, the
tracks and the vias, and how many connections are left unrouted. The
output groups `dsn` and `ses` hold the DSN and Freerouting's session.
""",
    attrs = {
        "pcb": attr.label(
            doc = "The board to route (`kicad_pcb`): its footprints placed and their pads on nets. " +
                  "Copper it has already stays, and Freerouting routes around it.",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
        "max_passes": attr.int(
            doc = "Freerouting's most routing passes; 0 keeps Freerouting's own default.",
            default = 0,
        ),
        "allow_unrouted": attr.bool(
            doc = "Write the board even when connections are left unrouted.",
            default = False,
        ),
        "_dsn": attr.label(
            default = "//kicad/private/autoroute:dsn.py",
            allow_single_file = True,
        ),
        "_ses": attr.label(
            default = "//kicad/private/autoroute:ses.py",
            allow_single_file = True,
        ),
    },
    toolchains = [_KICAD, _FREEROUTING] + RUN_TOOL_TOOLCHAINS,
)

def _autoroute_selftest_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    run_tool(
        ctx,
        _KICAD,
        "kicad_python",
        arguments = ["-I", ctx.file._dsn.path, "--selftest"],
        inputs = [ctx.file._dsn],
        stdout = out,
        mnemonic = "KicadDsnSelftest",
        progress_message = "Checking the design rules reader of kicad_autoroute",
    )
    return [DefaultInfo(files = depset([out]))]

autoroute_selftest = rule(
    implementation = _autoroute_selftest_impl,
    doc = "Runs dsn.py's self-test of its design rules reader; it builds only if the test passes.",
    attrs = {
        "_dsn": attr.label(
            default = "//kicad/private/autoroute:dsn.py",
            allow_single_file = True,
        ),
    },
    toolchains = [_KICAD] + RUN_TOOL_TOOLCHAINS,
)
