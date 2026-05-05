# antithesis
This is a scratch dir containing binaries for antithesis PoC.
<!-- This is likely not the ideal location, it's just for the hackaton. -->

* Live in a test template, e.g. /opt/antithesis/test/v1/<test_template>/<prefix>_<command>.
* Have a filename starting with a recognized <prefix>. The <prefix> determines when and how the command runs. <command> can be any valid Unix filename, with any or no extension.
* Be marked executable by the container’s default user.
* Be a compiled binary or have a shebang, e.g. #!/usr/bin/env bash, #!/usr/bin/env python3 if it’s a script.


## Generate instrumentation example:

```sh
go tool antithesis-go-instrumentor tool/antithesis/crud tool/antithesis/crud_generated_instrumentation
```