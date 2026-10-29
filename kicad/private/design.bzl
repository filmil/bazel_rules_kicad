# SPDX-License-Identifier: Apache-2.0
"""The input rules: a schematic and a board, staged as KiCad projects.

Neither rule runs KiCad. Each puts its files, by symlink, into one
directory, `<name>_project/`, with the project file and the library
tables that KiCad reads from a project's directory. Every rule that runs
KiCad on the design works on that directory, so KiCad sees a whole project
and finds libraries only through the tables (spec H4).
"""

load(":library.bzl", "KicadLibraryInfo", "merge_libraries", "write_lib_tables")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

KicadSchematicInfo = provider(
    doc = "A schematic, staged as a KiCad project.",
    fields = {
        "root": "File: the root sheet, in the project directory.",
        "sheets": "list of File: the other sheets, in the project directory.",
        "project": "File: the project file, <root name>.kicad_pro.",
        "project_dir": "string: the project directory, relative to the execution root.",
        "library": "KicadLibraryInfo: the libraries the tables list.",
        "files": "depset of File: every file KiCad reads for this schematic, libraries included.",
    },
)

KicadPcbInfo = provider(
    doc = "A board, staged as a KiCad project with its schematic.",
    fields = {
        "board": "File: the board, in the project directory. Its name is the schematic's root name " +
                 "(<root>.kicad_pcb), or with a netlist the board's own.",
        "schematic": "KicadSchematicInfo or None: the schematic the board follows, staged in the board's " +
                     "project directory, next to the board.",
        "netlist": "File or None: the netlist the board follows, when there is no schematic; " +
                   "staged as <board name>.net.",
        "design_rules": "File or None: the design rules, staged with the same name as `board` " +
                        "(<root>.kicad_dru, or with a netlist <board name>.kicad_dru).",
        "project": "File: the project file.",
        "project_dir": "string: the project directory, relative to the execution root.",
        "library": "KicadLibraryInfo: the libraries the tables list.",
        "files": "depset of File: every file KiCad reads for this board, libraries included.",
    },
)

def _stage(ctx, dir, rel, target_file):
    out = ctx.actions.declare_file(dir + "/" + rel)
    ctx.actions.symlink(output = out, target_file = target_file)
    return out

def relative_path(short_path, base_dir):
    """Returns short_path below base_dir, or None if it is not below it.

    Both are short paths: `File.short_path`, and the directory part of
    another file's `short_path` (not `File.dirname`, which includes the
    output root of a generated file). Short paths are the same for source
    and generated files. An empty base_dir is the root package.

    Args:
      short_path: the path of a file.
      base_dir: the directory it should be in or below.

    Returns:
      The path below base_dir, or None.
    """
    if base_dir == "":
        return None if short_path.startswith("../") else short_path
    if short_path.startswith(base_dir + "/"):
        return short_path[len(base_dir) + 1:]
    return None

def _short_dir(f):
    return f.short_path.rpartition("/")[0]

def _relative(f, root, what, label):
    rel = relative_path(f.short_path, _short_dir(root))
    if rel == None:
        fail("%s: %s %s is not in or below %s, the directory of the root sheet" % (label, what, f.short_path, _short_dir(root) or "the root package"))
    return rel

def _project_file(ctx, dir, base, given):
    """Stages the project file, or writes an empty one.

    KiCad reads a directory's library tables only when it holds a project.
    """
    if given:
        return _stage(ctx, dir, base + ".kicad_pro", given)
    out = ctx.actions.declare_file(dir + "/" + base + ".kicad_pro")
    ctx.actions.write(out, "{}\n")
    return out

def _tables(ctx, dir, library, project_dir):
    sym = ctx.actions.declare_file(dir + "/sym-lib-table")
    fp = ctx.actions.declare_file(dir + "/fp-lib-table")
    write_lib_tables(ctx, library, project_dir, sym, fp)
    return [sym, fp]

def _kicad_schematic_impl(ctx):
    dir = ctx.label.name + "_project"
    root_src = ctx.file.root
    if not root_src.basename.endswith(".kicad_sch"):
        fail("%s: root %s is not a .kicad_sch file" % (ctx.label, root_src.short_path))
    base = root_src.basename[:-len(".kicad_sch")]
    root = _stage(ctx, dir, root_src.basename, root_src)
    project_dir = root.dirname
    sheets = []
    for f in ctx.files.srcs:
        if f == root_src:
            fail("%s: srcs lists the root sheet %s" % (ctx.label, f.short_path))
        sheets.append(_stage(ctx, dir, _relative(f, root_src, "sheet", ctx.label), f))
    project = _project_file(ctx, dir, base, ctx.file.project)
    library = merge_libraries(ctx.label, [d[KicadLibraryInfo] for d in ctx.attr.deps])
    tables = _tables(ctx, dir, library, project_dir)

    # Fails the build when srcs and the sheets the root references differ;
    # the message names each sheet. Bazel runs validation actions with
    # every build of the target.
    stamp = ctx.actions.declare_file(ctx.label.name + ".sheetcheck")
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "sheetcheck",
        arguments = ["--stamp", stamp.path, root.path] + [s.path for s in sheets],
        inputs = [root] + sheets,
        outputs = [stamp],
        mnemonic = "KicadSheetCheck",
        progress_message = "Checking the sheets of %{label}",
    )

    staged = [root] + sheets + [project] + tables
    files = depset(staged, transitive = [library.files])
    return [
        KicadSchematicInfo(
            root = root,
            sheets = sheets,
            project = project,
            project_dir = project_dir,
            library = library,
            files = files,
        ),
        DefaultInfo(files = depset(staged)),
        OutputGroupInfo(_validation = depset([stamp])),
    ]

kicad_schematic = rule(
    implementation = _kicad_schematic_impl,
    doc = """A KiCad schematic: the root sheet, its other sheets, and its libraries.

```starlark
kicad_schematic(
    name = "sch",
    root = "main.kicad_sch",
    srcs = ["power.kicad_sch"],
    deps = ["@kicad_libs//:standard"],
)
```

It runs no KiCad. It stages the files as a KiCad project in
`<name>_project/`: the sheets at their paths relative to the root's
directory, `<root name>.kicad_pro`, and a `sym-lib-table` and an
`fp-lib-table` whose paths are relative to the project. Every rule that
runs KiCad on the schematic reads `KicadSchematicInfo`.

Building the target checks that `srcs` holds exactly the sheets the root
references, directly or through other sheets. Otherwise the build fails
and names the sheet.
""",
    attrs = {
        "root": attr.label(
            doc = "The root sheet.",
            allow_single_file = [".kicad_sch"],
            mandatory = True,
        ),
        "srcs": attr.label_list(
            doc = "Every other sheet, in or below the root's directory.",
            allow_files = [".kicad_sch"],
        ),
        "deps": attr.label_list(
            doc = "Symbol and footprint libraries (`kicad_library`).",
            providers = [KicadLibraryInfo],
        ),
        "project": attr.label(
            doc = "The project file (.kicad_pro). An empty one is used if not given.",
            allow_single_file = [".kicad_pro"],
        ),
    },
    toolchains = RUN_TOOL_TOOLCHAINS,
)

def _kicad_pcb_impl(ctx):
    sch = ctx.attr.schematic[KicadSchematicInfo] if ctx.attr.schematic else None
    if (sch == None) == (ctx.file.netlist == None):
        fail("%s: give exactly one of schematic and netlist" % ctx.label)
    dir = ctx.label.name + "_project"

    # KiCad finds a board's schematic by name: <base>.kicad_pcb next to
    # <base>.kicad_sch.
    if sch:
        base = sch.root.basename[:-len(".kicad_sch")]
    else:
        base = ctx.file.board.basename[:-len(".kicad_pcb")]
    board = _stage(ctx, dir, base + ".kicad_pcb", ctx.file.board)
    project_dir = board.dirname
    staged = [board]
    sch_root = None
    sch_sheets = []
    if sch:
        sch_root = _stage(ctx, dir, sch.root.basename, sch.root)
        for f in sch.sheets:
            sch_sheets.append(_stage(ctx, dir, f.path[len(sch.project_dir) + 1:], f))
        staged.extend([sch_root] + sch_sheets)
    netlist = None
    if ctx.file.netlist:
        netlist = _stage(ctx, dir, base + ".net", ctx.file.netlist)
        staged.append(netlist)
    rules = None
    if ctx.file.design_rules:
        rules = _stage(ctx, dir, base + ".kicad_dru", ctx.file.design_rules)
        staged.append(rules)
    given = ctx.file.project or (sch.project if sch else None)
    project = _project_file(ctx, dir, base, given)
    staged.append(project)
    infos = [d[KicadLibraryInfo] for d in ctx.attr.deps]
    if sch:
        infos.append(sch.library)
    library = merge_libraries(ctx.label, infos)
    tables = _tables(ctx, dir, library, project_dir)
    staged.extend(tables)

    # The schematic's own staged files come along, so the symlinks above
    # resolve wherever the action runs.
    files = depset(staged, transitive = [library.files] + ([sch.files] if sch else []))
    pcb_sch = None
    if sch:
        # The schematic as it is in this project directory, so that a rule
        # never mixes files of two directories.
        pcb_sch = KicadSchematicInfo(
            root = sch_root,
            sheets = sch_sheets,
            project = project,
            project_dir = project_dir,
            library = library,
            # The schematic's files only, not the board's or its rules.
            files = depset(
                [sch_root] + sch_sheets + [project] + tables,
                transitive = [library.files, sch.files],
            ),
        )
    return [
        KicadPcbInfo(
            board = board,
            schematic = pcb_sch,
            netlist = netlist,
            design_rules = rules,
            project = project,
            project_dir = project_dir,
            library = library,
            files = files,
        ),
        DefaultInfo(files = depset(staged)),
    ]

kicad_pcb = rule(
    implementation = _kicad_pcb_impl,
    doc = """A KiCad board, with the schematic or netlist it follows.

```starlark
kicad_pcb(
    name = "pcb",
    board = "main.kicad_pcb",
    schematic = ":sch",
    design_rules = "main.kicad_dru",
)
```

It runs no KiCad. It stages a KiCad project in `<name>_project/`: the
board as `<root name>.kicad_pcb` next to the schematic's sheets (KiCad
finds a board's schematic by that name), the design rules as
`<root name>.kicad_dru`, the project file (`project`, else the
schematic's), and library tables with the libraries of the schematic and
of `deps`. With `netlist` instead of `schematic`, the names come from the
board's: `<board name>.kicad_pcb`, `<board name>.net` and
`<board name>.kicad_dru`. Every rule that runs KiCad on the board reads
`KicadPcbInfo`.
""",
    attrs = {
        "board": attr.label(
            doc = "The board.",
            allow_single_file = [".kicad_pcb"],
            mandatory = True,
        ),
        "schematic": attr.label(
            doc = "The schematic (`kicad_schematic`). Give this or `netlist`.",
            providers = [KicadSchematicInfo],
        ),
        "netlist": attr.label(
            doc = "A KiCad netlist, for a circuit with no schematic (such as one written in code). Give this or `schematic`.",
            allow_single_file = [".net"],
        ),
        "design_rules": attr.label(
            doc = "Custom design rules (.kicad_dru).",
            allow_single_file = [".kicad_dru"],
        ),
        "deps": attr.label_list(
            doc = "More libraries (`kicad_library`), for footprints the schematic does not bring.",
            providers = [KicadLibraryInfo],
        ),
        "project": attr.label(
            doc = "The project file (.kicad_pro). Default: the schematic's, else an empty one.",
            allow_single_file = [".kicad_pro"],
        ),
    },
)

def _sheet_check_test_impl(ctx):
    helpers = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = helpers.sheetcheck.files_to_run.executable,
        is_executable = True,
    )
    files = [ctx.file.root] + ctx.files.srcs
    return [
        DefaultInfo(
            executable = exe,
            runfiles = ctx.runfiles(files = files).merge(helpers.sheetcheck.default_runfiles),
        ),
        RunEnvironmentInfo(environment = {
            "SHEETCHECK_EXPECT": ctx.attr.expect,
            "SHEETCHECK_FILES": "\n".join([f.short_path for f in files]),
        }),
    ]

sheet_check_test = rule(
    implementation = _sheet_check_test_impl,
    test = True,
    doc = """The sheet check's own known-bad tests (spec T3): passes only if the
check finds the problem `expect` names. It runs the checker of
kicad_schematic's validation on the same files.""",
    attrs = {
        "root": attr.label(allow_single_file = [".kicad_sch"], mandatory = True),
        "srcs": attr.label_list(allow_files = [".kicad_sch"]),
        "expect": attr.string(
            doc = "KIND:SHEET, KIND being missing, unused or circular; SHEET relative to the root's directory.",
            mandatory = True,
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)
