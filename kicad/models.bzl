# SPDX-License-Identifier: Apache-2.0
"""KiCad's 3D model libraries, fetched over BitTorrent (issue #43).

```starlark
models = use_extension("@rules_kicad//kicad:models.bzl", "models")
use_repo(models, "kicad_models")
```

This file has no generated page: Stardoc cannot load rules_bittorrent's
package (https://github.com/filmil/bazel_rules_bt). See the `models`
extension's doc below and README.md.
"""

load("//kicad/private:models.bzl", _models = "models")

models = _models
