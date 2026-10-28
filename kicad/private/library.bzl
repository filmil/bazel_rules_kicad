# SPDX-License-Identifier: Apache-2.0
"""KiCad libraries, and library tables with paths relative to the project."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE")

KicadLibraryInfo = provider(
    doc = "Symbol and footprint libraries, by the nickname KiCad uses for each.",
    fields = {
        "symbols": "dict: nickname to the `.kicad_sym` File.",
        "footprints": "dict: nickname to the path of its `.pretty` directory, " +
                      "relative to the execution root.",
        "files": "depset of every file of the libraries.",
    },
)

def _nickname(path, suffix):
    base = path.rsplit("/", 1)[-1]
    if not base.endswith(suffix) or base == suffix:
        fail("%s: expected a name ending in %s, with a nickname before it" % (path, suffix))
    return base[:-len(suffix)]

def _merge(into, nick, value, what, label):
    if nick in into and into[nick] != value:
        fail("%s: two %s libraries are named %r: %s and %s" % (label, what, nick, into[nick], value))
    into[nick] = value

def merge_libraries(label, infos):
    """Returns one `KicadLibraryInfo` with every library of `infos`.

    A nickname may appear twice only for the same library (a diamond).

    Args:
      label: the label to name in an error.
      infos: a list of `KicadLibraryInfo`.

    Returns:
      A `KicadLibraryInfo`.
    """
    symbols = {}
    footprints = {}
    for info in infos:
        for nick, f in info.symbols.items():
            _merge(symbols, nick, f, "symbol", label)
        for nick, d in info.footprints.items():
            _merge(footprints, nick, d, "footprint", label)
    return KicadLibraryInfo(
        symbols = symbols,
        footprints = footprints,
        files = depset(transitive = [i.files for i in infos]),
    )

def _kicad_library_impl(ctx):
    deps = merge_libraries(ctx.label, [d[KicadLibraryInfo] for d in ctx.attr.deps])
    symbols = dict(deps.symbols)
    footprints = dict(deps.footprints)
    transitive = [deps.files]
    for f in ctx.files.symbols:
        _merge(symbols, _nickname(f.path, ".kicad_sym"), f, "symbol", ctx.label)
    mods = [f for f in ctx.files.footprints if f.extension == "kicad_mod"]
    if ctx.files.footprints and not mods:
        fail("%s: footprints has no .kicad_mod file" % ctx.label)
    for f in mods:
        _merge(footprints, _nickname(f.dirname, ".pretty"), f.dirname, "footprint", ctx.label)
    files = depset(ctx.files.symbols + mods, transitive = transitive)
    return [
        KicadLibraryInfo(symbols = symbols, footprints = footprints, files = files),
        DefaultInfo(files = files),
    ]

kicad_library = rule(
    implementation = _kicad_library_impl,
    doc = """Symbol and footprint libraries for KiCad (spec H4).

A symbol library is one `.kicad_sym` file; its nickname is the file name
without the extension. A footprint library is a `<nickname>.pretty`
directory; list its `.kicad_mod` files. Files of other kinds in
`footprints` (a README) are ignored.

```starlark
kicad_library(
    name = "my_parts",
    symbols = ["My_Parts.kicad_sym"],
    footprints = glob(["My_Parts.pretty/*.kicad_mod"]),
    deps = ["@kicad_libs//:standard"],
)
```

`@kicad_libs//:standard` holds the KiCad 9 symbol and footprint libraries,
pinned by sha256. Rules that run KiCad on a design generate its
`sym-lib-table` and `fp-lib-table` from these, with paths relative to the
project (see `kicad_lib_tables`), so KiCad never reads a library from the
host.
""",
    attrs = {
        "symbols": attr.label_list(
            doc = "`.kicad_sym` files.",
            allow_files = [".kicad_sym"],
        ),
        "footprints": attr.label_list(
            doc = "The files of `.pretty` directories.",
            allow_files = True,
        ),
        "deps": attr.label_list(
            doc = "Other libraries to include. A nickname may appear only once.",
            providers = [KicadLibraryInfo],
        ),
    },
)

def relative_to(path, project_dir):
    """Returns `${KIPRJMOD}/...` for a path, both relative to the execution root.

    Args:
      path: the library's path, relative to the execution root.
      project_dir: the directory KiCad sees as the project, relative to the
        execution root.

    Returns:
      A URI that KiCad resolves inside the execution root wherever it is.
    """
    if path.startswith("/") or project_dir.startswith("/"):
        fail("absolute path: %s, %s" % (path, project_dir))
    parts = [s for s in project_dir.split("/") if s not in ("", ".")]
    if ".." in parts:
        fail("project_dir %s leaves the execution root" % project_dir)

    # A path that starts with ../ (an external repository with
    # --experimental_sibling_repository_layout) still resolves: the ../
    # steps first reach the execution root.
    up = len(parts)
    return "${KIPRJMOD}/" + "../" * up + path

def _quote(s):
    return '"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"'

def _table(kind, entries, project_dir):
    lines = ["(%s" % kind, "  (version 7)"]
    for nick in sorted(entries.keys()):
        lines.append("  (lib (name %s)(type \"KiCad\")(uri %s)(options \"\")(descr \"\"))" % (
            _quote(nick),
            _quote(relative_to(entries[nick], project_dir)),
        ))
    lines.append(")")
    return "\n".join(lines) + "\n"

def write_lib_tables(ctx, info, project_dir, sym_table, fp_table):
    """Writes the library tables of a project.

    Every URI is `${KIPRJMOD}` followed by a path relative to the project,
    so the tables name no host path and work in any execution root or
    sandbox.

    Args:
      ctx: the rule context.
      info: the `KicadLibraryInfo` of the libraries.
      project_dir: the directory where the project's files will be, as a
        path relative to the execution root.
      sym_table: the output `File` for `sym-lib-table`.
      fp_table: the output `File` for `fp-lib-table`.
    """
    ctx.actions.write(sym_table, _table(
        "sym_lib_table",
        {n: f.path for n, f in info.symbols.items()},
        project_dir,
    ))
    ctx.actions.write(fp_table, _table("fp_lib_table", info.footprints, project_dir))

def _kicad_lib_tables_impl(ctx):
    sym = ctx.actions.declare_file(ctx.label.name + "/sym-lib-table")
    fp = ctx.actions.declare_file(ctx.label.name + "/fp-lib-table")
    info = ctx.attr.library[KicadLibraryInfo]
    write_lib_tables(ctx, info, sym.dirname, sym, fp)

    # The libraries come along, so whatever uses the tables has the files
    # they name.
    return [
        DefaultInfo(
            files = depset([sym, fp]),
            runfiles = ctx.runfiles(transitive_files = info.files),
        ),
        info,
    ]

kicad_lib_tables = rule(
    implementation = _kicad_lib_tables_impl,
    doc = """Writes `<name>/sym-lib-table` and `<name>/fp-lib-table` for a library.

The project directory is `<name>/` next to the tables, as when a rule puts
a design's files there. The rules that run KiCad write their own tables
the same way.
""",
    attrs = {
        "library": attr.label(
            doc = "The libraries the tables list.",
            mandatory = True,
            providers = [KicadLibraryInfo],
        ),
    },
)

def _lib_table_test_impl(ctx):
    helpers = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers
    files = depset(transitive = [t[DefaultInfo].files for t in ctx.attr.tables]).to_list()
    tables = [f for f in files if f.basename in ("sym-lib-table", "fp-lib-table")]
    if not tables:
        fail("%s: no sym-lib-table or fp-lib-table among the outputs of %s" % (ctx.label, ctx.attr.tables))
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = helpers.tablecheck.files_to_run.executable,
        is_executable = True,
    )
    runfiles = ctx.runfiles(files = tables).merge(helpers.tablecheck.default_runfiles)
    return [
        DefaultInfo(executable = exe, runfiles = runfiles),
        RunEnvironmentInfo(environment = {
            "TABLECHECK_EXPECT": ctx.attr.expect,
            "TABLECHECK_FILES": "\n".join([f.short_path for f in tables]),
        }),
    ]

raw_lib_table_test = rule(
    implementation = _lib_table_test_impl,
    test = True,
    doc = "The rule behind lib_table_test. Use the macro.",
    attrs = {
        "tables": attr.label_list(
            doc = "Targets whose sym-lib-table and fp-lib-table outputs are checked.",
            allow_files = True,
            mandatory = True,
        ),
        "expect": attr.string(
            doc = "\"clean\", or \"host\" for this rule's own known-bad test.",
            default = "clean",
            values = ["clean", "host"],
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)

def lib_table_test(name, tables, expect = "clean", **kwargs):
    """Fails if a library table names a path on the host (spec H4).

    Every library URI in the `sym-lib-table` and `fp-lib-table` outputs of
    `tables` must start with `${KIPRJMOD}/` and name no other variable. An
    absolute path, a home-relative path or a variable such as
    `${KICAD9_SYMBOL_DIR}` fails the test.

    ```starlark
    lib_table_test(
        name = "tables_test",
        tables = [":tables"],
    )
    ```

    Args:
      name: the test's name.
      tables: targets whose table outputs are checked.
      expect: "clean": every table must pass. "host": some table must fail;
        for a known-bad fixture that proves the check works (spec T3).
      **kwargs: common test attributes, such as `size` and `tags`.
    """
    raw_lib_table_test(name = name, tables = tables, expect = expect, **kwargs)
