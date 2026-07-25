# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in galloc, please report it privately
through [GitHub Security Advisories](https://github.com/gogpu/galloc/security/advisories/new).

**Do not open a public issue for security vulnerabilities.**

## Response

We commit to:

- Acknowledging your report within **48 hours**
- Providing an initial assessment within **5 business days**
- Releasing a fix as soon as practical, coordinated with you

## Scope

galloc is a pure-Go offset allocator with no external dependencies, no unsafe
code, and no network or file system access. Security concerns are primarily
limited to:

- Denial of service via crafted inputs (e.g., exhausting node pool)
- Panic-inducing inputs (e.g., double-free detection)
- Logic errors in allocation/coalescing that could cause overlapping regions

## Supported Versions

Only the latest release receives security updates.
