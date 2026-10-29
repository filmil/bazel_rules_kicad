<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of rules_kicad.

Load every rule from this file:

```starlark
load("@rules_kicad//kicad:defs.bzl", "determinism_test", "swap_test", "tool_toolchain")
```

The rules arrive in milestones M1 and M2 (see the issues of
https://github.com/filmil/bazel_rules_kicad). This file is the only public
entry point; everything under `//kicad/private` may change without notice.

<a id="kicad_bom"></a>

## kicad_bom

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_bom")

kicad_bom(<a href="#kicad_bom-name">name</a>, <a href="#kicad_bom-exclude_dnp">exclude_dnp</a>, <a href="#kicad_bom-fields">fields</a>, <a href="#kicad_bom-group_by">group_by</a>, <a href="#kicad_bom-labels">labels</a>, <a href="#kicad_bom-schematic">schematic</a>, <a href="#kicad_bom-sort_field">sort_field</a>)
</pre>

A schematic's bill of materials, as CSV.

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
to kicad-cli that shapes the output (no project preset applies), so the
columns do not change with its defaults.

The default grouping has `${DNP}`, so a do-not-populate part gets a row
of its own: grouped by value and footprint only, it joins the row of its
twins, which then counts it and says DNP for all of them.

`fields` takes symbol fields (`Reference`, `Value`, `Footprint`, or any
field of the parts) and kicad-cli's own: `${QUANTITY}`, `${DNP}`,
`${ITEM_NUMBER}`. Parts marked "exclude from BOM" are left out, and so,
with `exclude_dnp`, are parts marked "do not populate". The output has no
time stamp.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_bom-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_bom-exclude_dnp"></a>exclude_dnp |  Leave out parts marked "do not populate".   | Boolean | optional |  `False`  |
| <a id="kicad_bom-fields"></a>fields |  The columns, in order: symbol fields or kicad-cli's ${...} fields.   | List of strings | optional |  `["Reference", "Value", "Footprint", "${QUANTITY}", "${DNP}"]`  |
| <a id="kicad_bom-group_by"></a>group_by |  Parts with the same values of these fields share a row; each must be in `fields`. Empty: one row per part.   | List of strings | optional |  `["Value", "Footprint", "${DNP}"]`  |
| <a id="kicad_bom-labels"></a>labels |  The header of each column, in the order of `fields`.   | List of strings | optional |  `["Refs", "Value", "Footprint", "Qty", "DNP"]`  |
| <a id="kicad_bom-schematic"></a>schematic |  The schematic (`kicad_schematic`).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="kicad_bom-sort_field"></a>sort_field |  The field the rows are sorted by, ascending.   | String | optional |  `"Reference"`  |


<a id="kicad_drc_report"></a>

## kicad_drc_report

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_drc_report")

kicad_drc_report(<a href="#kicad_drc_report-name">name</a>, <a href="#kicad_drc_report-pcb">pcb</a>)
</pre>

KiCad's DRC report of a board, in JSON.

kicad-cli runs on the board's staged project (`kicad_pcb`), so it finds
footprints only through the project's library tables, and reads the
design rules (`design_rules`) where KiCad expects them, next to the board.
When the board follows a schematic, the report also has the schematic
parity check: footprints and nets that differ from the schematic. It lists
every violation of severity error or warning, and every unconnected item;
which checks have which severity is the design's own setting. The report's
date is removed (spec D2), so the same board gives the same bytes.

KiCad drops a design rules file it cannot read, without a word: its
built-in checks still run, but none of the file's rules do, and DRC still
passes (kicad#25848). So with `design_rules`, the rule also
runs DRC on a canary board, with a copy of the rule file that has a canary
rule appended; that report is the output group `canary`, and
`kicad_drc_test` fails unless it has the canary's violation.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_drc_report-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_drc_report-pcb"></a>pcb |  The board (`kicad_pcb`).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="kicad_erc_report"></a>

## kicad_erc_report

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_erc_report")

kicad_erc_report(<a href="#kicad_erc_report-name">name</a>, <a href="#kicad_erc_report-schematic">schematic</a>)
</pre>

KiCad's ERC report of a schematic, in JSON.

kicad-cli runs on the schematic's staged project, so it finds symbols and
footprints only through the project's library tables. The report lists
every violation of severity error or warning; which checks have which
severity is the design's own setting, in its project file. The report's
date is removed (spec D2), so the same schematic gives the same bytes.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_erc_report-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_erc_report-schematic"></a>schematic |  The schematic (`kicad_schematic`).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="kicad_lib_tables"></a>

## kicad_lib_tables

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_lib_tables")

kicad_lib_tables(<a href="#kicad_lib_tables-name">name</a>, <a href="#kicad_lib_tables-library">library</a>)
</pre>

Writes `<name>/sym-lib-table` and `<name>/fp-lib-table` for a library.

The project directory is `<name>/` next to the tables, as when a rule puts
a design's files there. The rules that run KiCad write their own tables
the same way.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_lib_tables-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_lib_tables-library"></a>library |  The libraries the tables list.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="kicad_library"></a>

## kicad_library

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_library")

kicad_library(<a href="#kicad_library-name">name</a>, <a href="#kicad_library-deps">deps</a>, <a href="#kicad_library-footprints">footprints</a>, <a href="#kicad_library-symbols">symbols</a>)
</pre>

Symbol and footprint libraries for KiCad (spec H4).

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

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_library-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_library-deps"></a>deps |  Other libraries to include. A nickname may appear only once.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="kicad_library-footprints"></a>footprints |  The files of `.pretty` directories.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="kicad_library-symbols"></a>symbols |  `.kicad_sym` files.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |


<a id="kicad_pcb"></a>

## kicad_pcb

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_pcb")

kicad_pcb(<a href="#kicad_pcb-name">name</a>, <a href="#kicad_pcb-deps">deps</a>, <a href="#kicad_pcb-board">board</a>, <a href="#kicad_pcb-design_rules">design_rules</a>, <a href="#kicad_pcb-netlist">netlist</a>, <a href="#kicad_pcb-project">project</a>, <a href="#kicad_pcb-schematic">schematic</a>)
</pre>

A KiCad board, with the schematic or netlist it follows.

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

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_pcb-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_pcb-deps"></a>deps |  More libraries (`kicad_library`), for footprints the schematic does not bring.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="kicad_pcb-board"></a>board |  The board.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="kicad_pcb-design_rules"></a>design_rules |  Custom design rules (.kicad_dru).   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |
| <a id="kicad_pcb-netlist"></a>netlist |  A KiCad netlist, for a circuit with no schematic (such as one written in code). Give this or `schematic`.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |
| <a id="kicad_pcb-project"></a>project |  The project file (.kicad_pro). Default: the schematic's, else an empty one.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |
| <a id="kicad_pcb-schematic"></a>schematic |  The schematic (`kicad_schematic`). Give this or `netlist`.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |


<a id="kicad_sch_pdf"></a>

## kicad_sch_pdf

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_sch_pdf")

kicad_sch_pdf(<a href="#kicad_sch_pdf-name">name</a>, <a href="#kicad_sch_pdf-schematic">schematic</a>)
</pre>

A schematic as one PDF, a page per sheet, in KiCad's colors.

```starlark
kicad_sch_pdf(
    name = "sch_pdf",
    schematic = ":sch",
)
```

kicad-cli plots the schematic's staged project. The PDF's creation date is
removed (spec D2), so the same schematic gives the same bytes.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_sch_pdf-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_sch_pdf-schematic"></a>schematic |  The schematic (`kicad_schematic`).   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="kicad_schematic"></a>

## kicad_schematic

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_schematic")

kicad_schematic(<a href="#kicad_schematic-name">name</a>, <a href="#kicad_schematic-deps">deps</a>, <a href="#kicad_schematic-srcs">srcs</a>, <a href="#kicad_schematic-project">project</a>, <a href="#kicad_schematic-root">root</a>)
</pre>

A KiCad schematic: the root sheet, its other sheets, and its libraries.

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

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad_schematic-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="kicad_schematic-deps"></a>deps |  Symbol and footprint libraries (`kicad_library`).   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="kicad_schematic-srcs"></a>srcs |  Every other sheet, in or below the root's directory.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="kicad_schematic-project"></a>project |  The project file (.kicad_pro). An empty one is used if not given.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |
| <a id="kicad_schematic-root"></a>root |  The root sheet.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


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


<a id="KicadLibraryInfo"></a>

## KicadLibraryInfo

<pre>
load("@rules_kicad//kicad:defs.bzl", "KicadLibraryInfo")

KicadLibraryInfo(<a href="#KicadLibraryInfo-symbols">symbols</a>, <a href="#KicadLibraryInfo-footprints">footprints</a>, <a href="#KicadLibraryInfo-files">files</a>)
</pre>

Symbol and footprint libraries, by the nickname KiCad uses for each.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="KicadLibraryInfo-symbols"></a>symbols |  dict: nickname to the `.kicad_sym` File.    |
| <a id="KicadLibraryInfo-footprints"></a>footprints |  dict: nickname to the path of its `.pretty` directory, relative to the execution root.    |
| <a id="KicadLibraryInfo-files"></a>files |  depset of every file of the libraries.    |


<a id="KicadPcbInfo"></a>

## KicadPcbInfo

<pre>
load("@rules_kicad//kicad:defs.bzl", "KicadPcbInfo")

KicadPcbInfo(<a href="#KicadPcbInfo-board">board</a>, <a href="#KicadPcbInfo-schematic">schematic</a>, <a href="#KicadPcbInfo-netlist">netlist</a>, <a href="#KicadPcbInfo-design_rules">design_rules</a>, <a href="#KicadPcbInfo-project">project</a>, <a href="#KicadPcbInfo-project_dir">project_dir</a>, <a href="#KicadPcbInfo-library">library</a>, <a href="#KicadPcbInfo-files">files</a>)
</pre>

A board, staged as a KiCad project with its schematic.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="KicadPcbInfo-board"></a>board |  File: the board, in the project directory. Its name is the schematic's root name (<root>.kicad_pcb), or with a netlist the board's own.    |
| <a id="KicadPcbInfo-schematic"></a>schematic |  KicadSchematicInfo or None: the schematic the board follows, staged in the board's project directory, next to the board.    |
| <a id="KicadPcbInfo-netlist"></a>netlist |  File or None: the netlist the board follows, when there is no schematic; staged as <board name>.net.    |
| <a id="KicadPcbInfo-design_rules"></a>design_rules |  File or None: the design rules, staged with the board's name (.kicad_dru).    |
| <a id="KicadPcbInfo-project"></a>project |  File: the project file.    |
| <a id="KicadPcbInfo-project_dir"></a>project_dir |  string: the project directory, relative to the execution root.    |
| <a id="KicadPcbInfo-library"></a>library |  KicadLibraryInfo: the libraries the tables list.    |
| <a id="KicadPcbInfo-files"></a>files |  depset of File: every file KiCad reads for this board, libraries included.    |


<a id="KicadSchematicInfo"></a>

## KicadSchematicInfo

<pre>
load("@rules_kicad//kicad:defs.bzl", "KicadSchematicInfo")

KicadSchematicInfo(<a href="#KicadSchematicInfo-root">root</a>, <a href="#KicadSchematicInfo-sheets">sheets</a>, <a href="#KicadSchematicInfo-project">project</a>, <a href="#KicadSchematicInfo-project_dir">project_dir</a>, <a href="#KicadSchematicInfo-library">library</a>, <a href="#KicadSchematicInfo-files">files</a>)
</pre>

A schematic, staged as a KiCad project.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="KicadSchematicInfo-root"></a>root |  File: the root sheet, in the project directory.    |
| <a id="KicadSchematicInfo-sheets"></a>sheets |  list of File: the other sheets, in the project directory.    |
| <a id="KicadSchematicInfo-project"></a>project |  File: the project file, <root name>.kicad_pro.    |
| <a id="KicadSchematicInfo-project_dir"></a>project_dir |  string: the project directory, relative to the execution root.    |
| <a id="KicadSchematicInfo-library"></a>library |  KicadLibraryInfo: the libraries the tables list.    |
| <a id="KicadSchematicInfo-files"></a>files |  depset of File: every file KiCad reads for this schematic, libraries included.    |


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


<a id="NormalizeToolchainInfo"></a>

## NormalizeToolchainInfo

<pre>
load("@rules_kicad//kicad:defs.bzl", "NormalizeToolchainInfo")

NormalizeToolchainInfo(<a href="#NormalizeToolchainInfo-epoch">epoch</a>)
</pre>

What the normalize toolchain offers beyond the `normalize` tool.

**FIELDS**

| Name  | Description |
| :------------- | :------------- |
| <a id="NormalizeToolchainInfo-epoch"></a>epoch |  int: the time every time stamp becomes, in seconds since 1970 UTC.    |


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


<a id="kicad_drc_test"></a>

## kicad_drc_test

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_drc_test")

kicad_drc_test(<a href="#kicad_drc_test-name">name</a>, <a href="#kicad_drc_test-pcb">pcb</a>, <a href="#kicad_drc_test-expect">expect</a>, <a href="#kicad_drc_test-kwargs">**kwargs</a>)
</pre>

Fails if KiCad's design rules check finds a violation.

The check covers the board's design rules, unconnected items and, when
the board follows a schematic, schematic parity. It makes
`<name>_report`, the DRC report (`kicad_drc_report`), and a test that
prints a summary of it and keeps it among the test's outputs.

```starlark
kicad_drc_test(
    name = "drc_test",
    pcb = ":pcb",
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="kicad_drc_test-name"></a>name |  the test's name.   |  none |
| <a id="kicad_drc_test-pcb"></a>pcb |  the board (`kicad_pcb`).   |  none |
| <a id="kicad_drc_test-expect"></a>expect |  empty for a board that must be clean. For a known-bad board (spec T3), a violation type, such as `clearance`: the test then passes only if the report has violations of that type and of no other. `design_rules_not_loaded` is the type of a rule file that KiCad did not read.   |  `""` |
| <a id="kicad_drc_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


<a id="kicad_erc_test"></a>

## kicad_erc_test

<pre>
load("@rules_kicad//kicad:defs.bzl", "kicad_erc_test")

kicad_erc_test(<a href="#kicad_erc_test-name">name</a>, <a href="#kicad_erc_test-schematic">schematic</a>, <a href="#kicad_erc_test-expect">expect</a>, <a href="#kicad_erc_test-kwargs">**kwargs</a>)
</pre>

Fails if KiCad's electrical rules check finds a violation.

It makes `<name>_report`, the ERC report (`kicad_erc_report`), and a
test that prints a summary of it and keeps it among the test's outputs
(`bazel-testlogs/<package>/<name>/test.outputs/`).

```starlark
kicad_erc_test(
    name = "erc_test",
    schematic = ":sch",
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="kicad_erc_test-name"></a>name |  the test's name.   |  none |
| <a id="kicad_erc_test-schematic"></a>schematic |  the schematic (`kicad_schematic`).   |  none |
| <a id="kicad_erc_test-expect"></a>expect |  empty for a schematic that must be clean. For a known-bad schematic (spec T3), a violation type, such as `pin_not_connected`: the test then passes only if the report has violations of that type and of no other.   |  `""` |
| <a id="kicad_erc_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


<a id="lib_table_test"></a>

## lib_table_test

<pre>
load("@rules_kicad//kicad:defs.bzl", "lib_table_test")

lib_table_test(<a href="#lib_table_test-name">name</a>, <a href="#lib_table_test-tables">tables</a>, <a href="#lib_table_test-expect">expect</a>, <a href="#lib_table_test-kwargs">**kwargs</a>)
</pre>

Fails if a library table names a path on the host (spec H4).

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


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="lib_table_test-name"></a>name |  the test's name.   |  none |
| <a id="lib_table_test-tables"></a>tables |  targets whose table outputs are checked.   |  none |
| <a id="lib_table_test-expect"></a>expect |  "clean": every table must pass. "host": some table must fail; for a known-bad fixture that proves the check works (spec T3).   |  `"clean"` |
| <a id="lib_table_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


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


