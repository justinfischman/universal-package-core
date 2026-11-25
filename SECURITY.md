# Security policy

## Supported versions

Only the latest release receives security fixes. Check it with `upc version` and the [GitHub releases](https://github.com/justinfischman/universal-package-core/releases) page.

## Reporting a vulnerability

Do not open a public issue for a vulnerability. Use GitHub private vulnerability reporting: open the repository's **Security** tab and choose **Report a vulnerability**. If that option is not available, open an issue that asks for a private contact without describing the problem, and the maintainer will reply.

Include the `upc version` output, your OS, the command you ran, and the smallest steps that reproduce the problem. Do not include secrets, tokens or private repository contents; redact them.

You can expect an acknowledgement as soon as the maintainer is able to respond. This is a single-maintainer project, so no response-time guarantee is made.

## Scope

`upc` runs the native package managers on your behalf and reads project and config files under your control. Reports about command or path injection through project files, config handling, or the release artifacts are in scope. Vulnerabilities in the native tools themselves (uv, poetry, pip, npm, yarn) belong to those projects.

Release artifacts ship with a `checksums.txt` (sha256) you can verify.
