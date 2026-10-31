# SPDX-License-Identifier: Apache-2.0
"""3D model libraries staged as KiCad looks them up, for kicad_step and kicad_render."""

# The names KiCad's footprints use for the 3D model directory, by version.
MODEL_DIR_VARS = [
    "KICAD6_3DMODEL_DIR",
    "KICAD7_3DMODEL_DIR",
    "KICAD8_3DMODEL_DIR",
    "KICAD9_3DMODEL_DIR",
    "KISYS3DMOD",
]

def _package_relative(f):
    """The path of `f` inside its package: "<lib>.3dshapes/x.step".

    A library of @kicad_models has its files at the top of its repository;
    a library of one's own may sit in any package.
    """
    parts = f.short_path.split("/")
    if parts[0] == "..":
        parts = parts[2:]
    rel = "/".join(parts)
    pkg = f.owner.package
    if pkg and rel.startswith(pkg + "/"):
        rel = rel[len(pkg) + 1:]
    return rel

def stage_models(ctx):
    """Links the model libraries into one directory, as KiCad expects them.

    Each library repository holds <lib>.3dshapes/ and LICENSE.md; the
    directory gets every <lib>.3dshapes/ side by side.

    Args:
      ctx: the rule's context; its `models` attribute lists the libraries.

    Returns:
      The directory's path, relative to the execution root, and the links.
    """
    root = ctx.label.name + "_models"
    links = []
    rels = []
    seen = {}
    for f in ctx.files.models:
        rel = _package_relative(f)
        if rel == "LICENSE.md":
            continue
        segs = rel.split("/")
        if len(segs) < 2 or not segs[0].endswith(".3dshapes"):
            fail("%s: %s is not in a <name>.3dshapes directory; give targets of @kicad_models, or files under <name>.3dshapes/ of a library of your own" % (ctx.label, f.short_path))
        if rel in seen:
            fail("%s: %s comes from two models targets" % (ctx.label, rel))
        seen[rel] = True
        link = ctx.actions.declare_file(root + "/" + rel)
        ctx.actions.symlink(output = link, target_file = f)
        links.append(link)
        rels.append(rel)
    if not links:
        fail("%s: models has no model file" % ctx.label)

    # The directory that holds the links: a link's path less its <rel>.
    return links[0].path[:-(len(rels[0]) + 1)], links
