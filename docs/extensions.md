<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Module extensions of hdlf_rules_kicad.

```starlark
kicad = use_extension("@hdlf_rules_kicad//kicad:extensions.bzl", "kicad")
kicad.toolchain(version = "9.0.9")
```

<a id="kicad"></a>

## kicad

<pre>
kicad = use_extension("@hdlf_rules_kicad//kicad:extensions.bzl", "kicad")
kicad.toolchain(<a href="#kicad.toolchain-version">version</a>)
</pre>

The prebuilt KiCad toolchain.

hdlf_rules_kicad registers it, so a user needs nothing to get KiCad
9.0.9. The extension also makes `@kicad_libs`, KiCad's symbol and
footprint libraries of the same version: `@kicad_libs//:standard`,
`:symbols` and `:footprints`, each a `kicad_library`. Bring it in with
`use_repo(kicad, "kicad_libs")`.

The extension also makes `@freerouting`, Freerouting's jar, which the
registered Freerouting toolchain of `kicad_autoroute` runs; it is
fetched only when a build routes a board.

KiCad's 3D model libraries come from a second extension, `models` in
`@hdlf_rules_kicad//kicad:models.bzl`.

The root module can pick another
version that hdlf_rules_kicad has a prebuilt for:

```starlark
kicad = use_extension("@hdlf_rules_kicad//kicad:extensions.bzl", "kicad")
kicad.toolchain(version = "9.0.9")
```


**TAG CLASSES**

<a id="kicad.toolchain"></a>

### toolchain

Picks the KiCad version. Only the root module's tag counts.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="kicad.toolchain-version"></a>version |  A KiCad version that hdlf_rules_kicad has a prebuilt for.   | String | optional |  `"9.0.9"`  |


