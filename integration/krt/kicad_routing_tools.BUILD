# SPDX-License-Identifier: Apache-2.0
# The release zip's plugins/ directory (MODULE.bazel strips the prefix).
package(default_visibility = ["//visibility:public"])

exports_files(glob(["**"]))

# Everything route.py reads: the Python sources, the router module for
# Linux x86_64, and rust_router/Cargo.toml, against whose version
# startup_checks.py checks the module.
filegroup(
    name = "files",
    srcs = glob(
        ["**/*.py"],
        exclude = [
            "movie3d/**",
            "kicad_routing_plugin/**",
        ],
    ) + [
        "VERSION",
        "requirements.txt",
        "rust_router/Cargo.toml",
        "rust_router/grid_router-linux-x86_64.so",
    ],
)
