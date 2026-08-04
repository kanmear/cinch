# templates/

Portable workflow templates. Two things never cross into here:

**A stack.** No language, framework, or command literal. Reference `{{commands.check}}`,
`{{paths.tests.integration}}`, `{{taxonomy.layers}}` instead — see `AGENTS.md` and
`scaffold/manifest.example.yml` for the variable contract.

**A harness.** No harness names, no slash-command syntax. Another workflow is referenced by its
rendered path, `.agent/workflows/<name>.md`, never by name or invocation syntax — a harness's
entry point is consumer-side glue (D045).

`templates_test.go` (`go test` / `make test`) catches the shapes of both — command-shaped inline
tokens, runner-config paths, and stack literals hardcoded from the scaffold contract. It does not
catch prose describing how a runner surfaces something; that's a judgment call while porting.
