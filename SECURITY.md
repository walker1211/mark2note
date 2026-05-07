# Security Policy

## Supported versions

Security fixes are provided for the latest released version of `mark2note` and the current `main` branch.

## Reporting a vulnerability

Please do not disclose security issues publicly before maintainers have had a chance to investigate.

If you find a vulnerability in `mark2note`, open a private security advisory on GitHub if available, or contact the maintainers through the repository owner profile. Include:

* A short description of the issue
* Steps to reproduce or a proof of concept
* Affected versions or commit SHA
* Any relevant logs with secrets removed

## Handling sensitive data

`mark2note` may work with local Markdown documents, generated notes, rendered HTML, PNG files, and optional animation or Live Photo artifacts. Keep private source documents, generated private notes, local configuration, logs, and credentials out of git.

Do not commit API keys, tokens, passwords, `.env` files, `configs/config.yaml`, private documents, or generated local artifacts.
