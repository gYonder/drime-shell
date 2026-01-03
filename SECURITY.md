# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in Drime Shell, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, please report security issues by emailing the maintainers directly or using GitHub's private vulnerability reporting feature:

1. Go to the [Security tab](../../security) of this repository
2. Click "Report a vulnerability"
3. Provide details about the vulnerability

### What to include

- A description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Any suggested fixes (optional)

### What to expect

- Acknowledgment of your report within 48 hours
- Regular updates on the progress
- Credit in the security advisory (unless you prefer to remain anonymous)

## Security Considerations

Drime Shell handles sensitive data including:

- **API tokens**: Stored in `~/.drime-shell/config.yaml` with `0600` permissions
- **Vault encryption keys**: Derived from user passwords, held only in memory during session
- **File transfers**: All API communication uses HTTPS

### Best Practices

- Never share your API token or config file
- Use strong passwords for vault encryption
- Review permissions on your config file (`chmod 600 ~/.drime-shell/config.yaml`)
- Log out when using shared systems (`drime logout`)
