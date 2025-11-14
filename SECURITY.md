# Security Policy

## Supported Versions

We release patches for security vulnerabilities for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.5.x   | :white_check_mark: |
| < 1.5   | :x:                |

## Reporting a Vulnerability

We take the security of GitLab Reviewer Roulette seriously. If you have discovered a security vulnerability, please report it privately.

### How to Report

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please use [GitHub's Security Advisories](https://github.com/aimd54/gitlab-reviewer-roulette/security/advisories/new) to privately report vulnerabilities.

Alternatively, you can report via email to: **1679489+aimd54@users.noreply.github.com**

Include the following information:

- Type of vulnerability
- Full paths of source file(s) related to the manifestation of the issue
- Location of the affected source code (tag/branch/commit or direct URL)
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your vulnerability report within 48 hours.
- **Investigation**: We will investigate and validate the vulnerability.
- **Timeline**: We aim to provide an initial assessment within 7 days.
- **Fix**: If confirmed, we will work on a fix and aim to release a patch within 30 days.
- **Credit**: We will credit you in the release notes (unless you prefer to remain anonymous).

## Security Best Practices

When deploying Reviewer Roulette:

### 1. Secrets Management

- Never commit secrets to version control
- Use Kubernetes Secrets or external secret managers
- Rotate GitLab tokens and webhook secrets regularly

### 2. Network Security

- Use TLS/HTTPS for all external communications
- Implement network policies in Kubernetes
- Restrict access to metrics and health endpoints

### 3. GitLab Webhook Security

- Always configure webhook secret tokens
- Validate webhook signatures
- Use HTTPS URLs for webhook endpoints

### 4. Database Security

- Use strong passwords for PostgreSQL
- Enable SSL for database connections
- Restrict database access to application pods only

### 5. Redis Security

- Configure Redis authentication
- Use Redis ACLs if available
- Restrict Redis access to application pods only

### 6. RBAC and Authentication

- Configure OIDC authentication for admin endpoints (when implemented)
- Use Kubernetes RBAC for pod access control
- Follow the principle of least privilege

### 7. Container Security

- Regularly update base images
- Run security scans on container images
- Use non-root users in containers (already configured)

### 8. Monitoring and Auditing

- Enable audit logging for admin operations
- Monitor for suspicious activity
- Set up alerts for security-relevant events

## Security Updates

Security updates will be released as:

- Patch releases (e.g., 1.5.1) for minor security issues
- Minor releases (e.g., 1.6.0) for major security updates

Security advisories will be published via:

- GitHub Security Advisories
- Release notes
- Project README

## Disclosure Policy

- We follow responsible disclosure practices
- Security issues will be disclosed publicly after a fix is available
- We request that security researchers give us reasonable time to respond before any disclosure

## Known Security Considerations

### Webhook Secret Validation

The application validates GitLab webhook signatures to prevent unauthorized requests. Ensure `GITLAB_WEBHOOK_SECRET` is configured and kept secret.

### GitLab Token Permissions

The GitLab bot token should have minimal required permissions:

- `api` - For reading user data, MR information, and posting comments
- `read_repository` - For accessing CODEOWNERS files

Avoid using tokens with `sudo` or admin privileges.

### Database Access

PostgreSQL credentials should be rotated regularly and access should be restricted to the application only.

### Redis Cache

Redis cache contains temporary availability data. While not highly sensitive, it should still be protected with authentication and network policies.

## Contact

For security concerns:

- Use [GitHub Security Advisories](https://github.com/aimd54/gitlab-reviewer-roulette/security/advisories/new) (preferred)
- Email: **1679489+aimd54@users.noreply.github.com**

For general questions, use GitHub Issues or Discussions.
