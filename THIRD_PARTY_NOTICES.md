# Third-party notices

repoctx's distributed native binary contains the following Go modules. Versions
are pinned by `go.mod` and authenticated for retrieval by `go.sum`.

| Module | Version | License | Copyright notice |
| --- | --- | --- | --- |
| `github.com/mattn/go-pointer` | `v0.0.1` | MIT | Copyright 2019 Yasuhiro Matsumoto |
| `github.com/tree-sitter-grammars/tree-sitter-markdown` | `v0.4.1` | MIT | Copyright 2021 Matthias Deiml |
| `github.com/tree-sitter/go-tree-sitter` | `v0.25.0` | MIT | Copyright 2024 Amaan Qureshi |
| `github.com/tree-sitter/tree-sitter-css` | `v0.25.0` | MIT | Copyright 2018 Max Brunsfeld |
| `github.com/tree-sitter/tree-sitter-html` | `v0.23.2` | MIT | Copyright 2014 Max Brunsfeld |
| `github.com/tree-sitter/tree-sitter-javascript` | `v0.25.0` | MIT | Copyright 2014 Max Brunsfeld |
| `github.com/tree-sitter/tree-sitter-python` | `v0.25.0` | MIT | Copyright 2016 Max Brunsfeld |
| `github.com/tree-sitter/tree-sitter-typescript` | `v0.23.2` | MIT | Copyright 2017 Max Brunsfeld |
| `gopkg.in/yaml.v3` | `v3.0.1` | MIT and Apache-2.0 | Copyright 2006-2011 Kirill Simonov; Copyright 2011-2019 Canonical Ltd |

The Python wheel bundles the same native binary. Python build and test tools are
not bundled runtime dependencies. Inspect a particular binary with `go version
-m PATH` and verify cached Go modules with `go mod verify`.

## MIT License

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

## Apache License 2.0 notice

The non-libyaml-derived files in `gopkg.in/yaml.v3` are licensed under the
Apache License, Version 2.0. You may obtain a copy at
<https://www.apache.org/licenses/LICENSE-2.0>. Unless required by applicable law
or agreed to in writing, software distributed under that license is distributed
on an "AS IS" basis, without warranties or conditions of any kind.
