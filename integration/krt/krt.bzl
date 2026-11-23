# SPDX-License-Identifier: Apache-2.0
"""kicad_route_krt: a board routed by KiCadRoutingTools' route.py (the
router trial of https://github.com/filmil/bazel_rules_kicad/issues/104).

A trial, kept in the integration module: users of hdlf_rules_kicad get
none of it. It loads hdlf_rules_kicad's private run_tool.bzl, as a rule
inside the module would; if the trial succeeds, the rule moves into the
module and becomes public.
"""

load("@hdlf_rules_kicad//kicad:defs.bzl", "KicadPcbInfo")

# buildifier: disable=bzl-visibility
# The trial runs its tool through the module's run_tool, as every action
# here must (CLAUDE.md), though it lives outside //kicad: if the trial
# succeeds, the rule moves into //kicad/private and this goes.
load("@hdlf_rules_kicad//kicad/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("@hdlf_rules_kicad//kicad:toolchain_type")
KRT_TOOLCHAIN_TYPE = Label("//krt:toolchain_type")

KrtInfo = provider(
    doc = "What a KiCadRoutingTools toolchain offers beyond its route tool.",
    fields = {
        "root": "string: the path of the release's plugins/ directory, which holds py_router/.",
        "files": "depset: the release's files route.py reads.",
    },
)

def _krt_toolchain_impl(ctx):
    route_py = ctx.file.route_py
    if route_py.basename != "route.py" or not route_py.dirname.endswith("/py_router"):
        fail("route_py: %s is not <root>/py_router/route.py" % route_py.path)
    files = depset([route_py] + ctx.files.files)
    return [platform_common.ToolchainInfo(
        tools = {"route": ctx.attr.route[DefaultInfo].files_to_run},
        tool_inputs = files,
        krt = KrtInfo(root = route_py.dirname[:-len("/py_router")], files = files),
    )]

krt_toolchain = rule(
    implementation = _krt_toolchain_impl,
    doc = "A KiCadRoutingTools toolchain: the release's files, and krt_route, which runs route.py from them.",
    attrs = {
        "files": attr.label(mandatory = True, allow_files = True, cfg = "exec"),
        "route": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "route_py": attr.label(mandatory = True, allow_single_file = True, cfg = "exec"),
    },
)

def _kicad_route_krt_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    kicad = ctx.toolchains[_KICAD]
    krt = ctx.toolchains[KRT_TOOLCHAIN_TYPE].krt
    if "kicad_python" not in kicad.tools:
        fail("%s: the KiCad toolchain has no kicad_python; route.py's exact fill needs it" % ctx.label)
    cli, kpy = kicad.tools["kicad_cli"], kicad.tools["kicad_python"]
    out = ctx.actions.declare_file(ctx.label.name + ".kicad_pcb")
    env = dict(getattr(kicad, "env", {}) or {})
    env.update({
        # The same order of sets and dicts on every run, no .pyc written
        # into the release's files, and one thread each for numpy's and
        # scipy's OpenBLAS: the same result whatever the machine's cores.
        "PYTHONHASHSEED": "0",
        "PYTHONDONTWRITEBYTECODE": "1",
        "OPENBLAS_NUM_THREADS": "1",
        "OMP_NUM_THREADS": "1",
    })
    path_env = dict(getattr(kicad, "path_env", {}) or {})
    path_env["KICAD_CLI"] = cli.executable
    path_env["KICAD_PYTHON"] = kpy.executable
    run_tool(
        ctx,
        KRT_TOOLCHAIN_TYPE,
        "route",
        arguments = ["--krt_root", krt.root, "--", pcb.board.path, out.path] + ctx.attr.route_args,
        inputs = depset(transitive = [pcb.files, krt.files, kicad.tool_inputs]),
        outputs = [out],
        tools = [cli, kpy],
        env = env,
        path_env = path_env,
        home_env = list(getattr(kicad, "home_env", []) or []),
        block_network = True,
        mnemonic = "KrtRoute",
        progress_message = "Routing %{label} with KiCadRoutingTools",
    )
    return [DefaultInfo(files = depset([out]))]

kicad_route_krt = rule(
    implementation = _kicad_route_krt_impl,
    doc = """`<name>.kicad_pcb`: the board of `pcb`, routed by KiCadRoutingTools'
route.py. route.py reads the board's design rules from the staged
`<board>.kicad_dru` beside it (kicad_pcb stages them so), through the
pinned kicad-cli, and runs its exact fills in the pinned KiCad's Python.""",
    attrs = {
        "pcb": attr.label(mandatory = True, providers = [KicadPcbInfo]),
        "route_args": attr.string_list(doc = "More route.py options."),
    },
    toolchains = [_KICAD, KRT_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
