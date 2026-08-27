# Responsible Disclosure Policy

**Effective Date:** August 2026

The Goy Company takes security seriously. If you discover a vulnerability in any Goy product or repository, we want to hear from you. This policy outlines how to report vulnerabilities, what you can expect from us, and what we ask of you in return.

---

## How to Report

Please send vulnerability reports to:

📧 **legal@goycompany.com**

Include as much detail as you can:
- Description of the vulnerability
- Steps to reproduce (or a proof of concept if you have one)
- Affected product(s) and version(s)
- Any potential impact you've identified

We accept reports in plaintext English. Do not include sensitive data (private keys, user data, exploit code that targets live systems) in your initial report.

---

## Our Commitments

### 1. Acknowledgment

We will acknowledge receipt of your report within **48 hours** of submission. If you don't receive a response within 48 hours, please follow up — your report may have been lost or filtered.

### 2. Evaluation

We will evaluate your report promptly and provide an initial assessment within **5 business days**. This assessment will include:
- Whether the report qualifies as a valid vulnerability
- Severity rating (Critical, High, Medium, Low)
- Estimated timeline for remediation

### 3. Remediation

We will work to remediate valid vulnerabilities according to the following severity-based targets:

| Severity | Target Remediation |
|---|---|
| Critical | 14 days |
| High | 30 days |
| Medium | 60 days |
| Low | 90 days |

These are targets, not guarantees. Complex vulnerabilities may require longer. We will keep you informed of our progress.

### 4. Disclosure Timeline

We request **90 days** from the date of our initial acknowledgment before you publicly disclose the vulnerability. This gives us time to:
- Reproduce the issue
- Develop and test a fix
- Deploy the fix to production
- Notify affected users if necessary

If we exceed the 90-day window without a fix, you may disclose the vulnerability at your discretion, provided you give us an additional **7 days** of notice before going public.

We will coordinate with you on the timing and content of any public disclosure, and we will credit you as the discoverer if you wish (see Hall of Fame below).

### 5. Hall of Fame

We maintain a public **Security Hall of Fame** at [goy.company/security/hall-of-fame](https://goy.company/security/hall-of-fame) (or equivalent URL). Researchers who report valid vulnerabilities may be recognized with:
- Name or handle (at your choice)
- Date of discovery
- Brief description of the vulnerability (with your consent)

You may choose to be listed anonymously or by pseudonym.

### 6. Rewards

In addition to Hall of Fame recognition, we may offer **discounts on Goy products** as a thank-you for significant findings. The value of the reward depends on the severity and impact of the vulnerability and is at The Goy Company's sole discretion.

We do not offer cash bounties at this time, but we reserve the right to introduce a bug bounty program in the future.

---

## Safe Harbor

The Goy Company will not initiate legal action against you — and will not support any legal action initiated by others — for security research conducted in accordance with this policy, including:

- Research that involves accessing our products in ways that go beyond the permissions granted under the GSAL, **provided** the research is conducted in good faith to identify and disclose vulnerabilities
- Publication of your research findings, provided you comply with the disclosure timeline above
- Any technical activity necessary to identify, reproduce, or demonstrate the vulnerability

This safe harbor applies as long as you:
- Do not compromise the security, availability, or integrity of any production system or user data
- Do not access, modify, or delete data that does not belong to you
- Do not maintain persistent access to any system after the vulnerability has been reported
- Report the vulnerability to us before disclosing it to any third party

If legal action is initiated against you by a third party for your research, we will take steps to make it known that your actions were conducted in accordance with this policy.

---

## Scope

This policy applies to all Goy products and repositories, including but not limited to:
- `goy-store` (persistence abstraction)
- `goy-node` (edge infrastructure)
- `goy-relay` (control plane)
- `goy-vpn` (WireGuard plane)
- Any other publicly accessible Goy product or service

Out of scope:
- Physical attacks against Goy offices or infrastructure
- Social engineering against Goy employees or users
- Denial-of-service attacks against production systems
- Vulnerabilities in third-party dependencies that are not specific to Goy's implementation (report those to the upstream project)

---

## We Ask That You

1. **Act in good faith.** This policy exists to make Goy products safer. Do not exploit vulnerabilities for personal gain, competitive advantage, or malicious purposes.
2. **Minimize harm.** If you discover user data during your research, stop and report it immediately. Do not access, download, or retain any data beyond what is necessary to demonstrate the vulnerability.
3. **Keep it confidential.** Do not disclose the vulnerability to anyone other than The Goy Company until we have acknowledged the report and agreed on a disclosure timeline.
4. **Follow the rules.** Stay within the scope defined above. If you're unsure whether your activity is permitted, ask us first at legal@goycompany.com.

---

## Questions?

If you have questions about this policy, or if you're unsure whether a vulnerability you've found is in scope, reach out to us at legal@goycompany.com before proceeding.

We appreciate your help in keeping Goy products safe.

---

**The Goy Company**
legal@goycompany.com
