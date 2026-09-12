# Company control and technique matrix — version 1

Baseline, modes A–D, review state and company decisions D1–D10 are defined in the
[assessment](company-assessment.md) and [threat model](company-threat-model.md).
Evidence IDs E1–E10 resolve in [the inventory](company-evidence.md).
Mappings are this project's applicability analysis, not NIST/MITRE endorsement,
certification, proof of compliance or proof that an attack occurred.

## Reference editions and interpretation

References were checked on 2026-09-11:

- **NIST CSF 2.0**, CSWP 29, 2024-02-26: [official publication, Appendix A](https://nvlpubs.nist.gov/nistpubs/CSWP/NIST.CSWP.29.pdf).
  PR.AA and PR.DS are categories, not SP 800-53 control IDs.
- **NIST SP 800-53 Rev. 5, release 5.2.0**: [NIST publication/update record](https://csrc.nist.gov/pubs/sp/800/53/r5/upd1/final)
  and [official OSCAL catalog pinned at 78650f02ad9321bb7b817846f8fbd4f2bcd620de](https://github.com/usnistgov/oscal-content/blob/78650f02ad9321bb7b817846f8fbd4f2bcd620de/nist.gov/SP800-53/rev5/json/NIST_SP-800-53_rev5_catalog.json).
  Catalog metadata is version 5.2.0. Base controls below do not imply every
  enhancement or organisation-defined parameter is satisfied.
- **NIST AI RMF 1.0 (2023)**: [official Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/).
  A revision is in progress; this assessment uses the published 1.0 framework.
- **MITRE ATT&CK Enterprise v19**: version-pinned technique pages below.
- **MITRE ATLAS 2026.08, format 6.0.0**: [official release data pinned at 41d4f5ca4112f0e492ffaa3ebff07dc80a75afa5](https://github.com/mitre-atlas/atlas-data/blob/41d4f5ca4112f0e492ffaa3ebff07dc80a75afa5/dist/v6/ATLAS-2026.08.yaml).
  Exact technique IDs below can be searched in that file; no third-party mapping
  is needed. Poisoned data retrieved by a benign tool is distinguished from a
  poisoned tool's definition, implementation or response.

**SA-12 is withdrawn in Rev. 5 and incorporated into the SR family.** It is retained
as an explicitly dispositioned original finding below, not marked passed or silently
assessed as an active Rev. 5 control. If the company requires Rev. 4, D1 must record
that edition and a separate assessment; no equivalence is assumed.

Status vocabulary: **product evidence** means an implemented bounded behavior has
observed tests; **partial/shared** means external controls or wider evidence remain;
**external/open** means no product implementation can close that organisational
requirement; **not applicable to A runtime** does not exempt artifact supply-chain
or endpoint responsibilities. None means organisation acceptance.

## NIST findings

| Exact reference / relevant requirement | Applicability | Product evidence and current status | Product / company owner and remaining decision |
|---|---|---|---|
| CSF 2.0 PR.AA-01, -02, -03, -04: identity/credential lifecycle, binding, authentication and assertions | B/C/D inherit OS/provider identities; A runtime has none | **Partial/shared:** native owned stores, typed credential handling and Copilot identity gate; E2/E3 and [identity boundary](company-identity.md). No independent app identity, RBAC or tenant proof. | Credential maintainer / IAM and provider owners: D3. Verify approved accounts, provider grants, SSO/MFA and revocation externally. |
| CSF 2.0 PR.AA-05: permissions, least privilege and separation of duties | All deployed binaries; capability policy for B/C/D | **Partial/shared:** protected administrator authority and source/action approvals, E2/E5/E7. Local user can still invoke other tools; provider grants are not reduced. | Policy maintainer / endpoint and tenant owners: D2/D3/D6. Enforce approved installations and native least privilege. |
| CSF 2.0 PR.AA-06: physical access | All endpoint modes | **External/open:** no product physical-access control. | Endpoint owner: D2; device inventory, session lock and physical access evidence. |
| CSF 2.0 PR.DS-01, -02, -10: stored, transmitted and in-use data protection | A has no runtime identity store/network; B/C/D have sensitive flows | **Partial/shared:** OS stores, HTTPS endpoint checks, typed minimization and retention, E3/E4; [data inventory](privacy-and-retention.md). Plaintext process/child access and provider stores remain. | Credential/privacy maintainers / privacy, endpoint and provider owners: D3/D4/D5. Verify encryption, access, TLS trust/egress and data-use acceptance. |
| CSF 2.0 PR.DS-11: protected and tested backups | Endpoint/provider records in B/C/D; deployment inventory for all | **External/open:** local purge is not backup deletion or tested recovery. No product backup system. | Endpoint/privacy owners: D4; define backup scope, encryption, expiry and restore tests. |
| SP 800-53 Rev. 5 CM-7: least functionality | A removes capabilities at build time; B/C/D restrict them locally | **Product evidence, shared deployment:** restricted import/symbol/syscall audits E1; default provider/integration denial E2; exact executable/config approvals E5/E7. Full personal mode is intentionally broader. | Product maintainers / endpoint owner: D1/D2/D5/D6. Choose essential functions, approved apps, ports/protocols and fleet enforcement. |
| SP 800-53 Rev. 5 SA-11: developer testing and evaluation | All modes and delivery pipeline | **Partial/shared:** negative cases and native CI tied to SHA, candidate dependency scans and flaw fixes E1–E9; private reporting and review E10. No independent penetration test or complete organisational assessment plan. | Product/security maintainers / company assessor: D1/D6/D9. Select assessment depth/frequency and review evidence; CI is not independent assurance. |
| SP 800-53 Rev. 5 SA-12: Supply Chain Protection, withdrawn | Original finding remains relevant as supply-chain risk | **Dispositioned, not passed:** assess SR rows below. No active SA-12 implementation claim. | Company control assessor: D1 confirms edition/tailoring. |
| SP 800-53 Rev. 5 SI-7: integrity verification and response | Artifacts in all modes; policy/images/config in B/C/D | **Partial/shared:** verified provenance/receipts and changed-byte refusal E8/E9; protected inputs E2/E5/E7. Does not attest every library or inspect host firmware; quota/audit advisory policy is unrelated. | Release/policy maintainers / endpoint owner: D2/D5/D6. Approve verifier roots, deployed versions, change response and OS integrity scope. |
| SP 800-53 Rev. 5 SR-1, SR-2: policy/procedures and supply-chain plan | All suppliers, providers and deployment modes | **External/open:** E10 states existing release/support boundaries; this package is evidence, not the company's supply-chain plan. | Company procurement/security: D9; establish owners, plan, review cadence and supplier inventory. |
| SP 800-53 Rev. 5 SR-3: supply-chain controls/processes | Source, build, dependency and artifact chain | **Partial/shared:** source-bound release gate, pinned tools, vulnerability review and immutable-artifact handling E8/E9. Repository/build authorization can still be compromised. | Release maintainer / company release/security owners: D2/D9. Review repository and build access controls and downstream acquisition process. |
| SP 800-53 Rev. 5 SR-4: provenance | All distributed artifacts and bundled ranking data | **Product evidence, shared trust:** exact source/ref, signed subjects, per-binary SBOM and embedded input hashes E8/E9. A signature establishes origin under stated trust, not safe behavior or data rights. | Release/data maintainers / release and data owners: D2/D8. Archive evidence, approve roots and redistribution. |
| SP 800-53 Rev. 5 SR-5, SR-6: acquisition methods and supplier reviews | Personally maintained pre-release product, providers, CodexBar and tooling | **Partial/shared:** candidate inventories and support limits E8–E10; no contractual service or full supplier assessment supplied. | Product release maintainer / procurement/security: D5/D9. Assess suppliers and existing commercial arrangements; SLA expansion is out of scope. |
| SP 800-53 Rev. 5 SR-7, SR-8: supply-chain operations security and notification agreements | Build/release operations and upstream incident communication | **External/open:** SECURITY.md offers a private reporting route (E10), not a notification agreement or organisation OPSEC program. | Release/security maintainers / company procurement/security: D9. Record applicable agreements and incident escalation; none is invented here. |
| SP 800-53 Rev. 5 SR-9, SR-10, SR-11: tamper protection, inspection and authenticity | Software artifact/installations for all modes | **Partial/shared:** E2/E5/E7/E8/E9 show tamper refusal and authenticated source. No hardware anti-counterfeit program or physical inspection evidence. | Release/policy maintainers / endpoint/acquisition owners: D1/D2/D5. Tailor hardware scope and deployed software inspection. |
| SP 800-53 Rev. 5 SR-12: component disposal | Retired binary, data, credentials and endpoint | **Partial/shared:** owned cleanup/logout E3/E4/E5; not provider revocation, backup/media sanitization or complete endpoint disposal. | Product data maintainer / endpoint/IAM/privacy: D3/D4/D9. Exercise revocation, inventory removal and disposal. |

## Reported attack techniques and abuse cases

Legitimate credential access or command execution is a capability, not evidence of
an adversary. These mappings explain how misuse could cross the threat boundaries.

| Exact technique reference | Applicability / threats | Product evidence and current status | Remaining owner/action |
|---|---|---|---|
| [ATT&CK v19 T1552.001 — Credentials In Files](https://attack.mitre.org/versions/v19/techniques/T1552/001/) | T01/T06; B/C/D personal or provider-owned files | **Partial/shared:** company owned fallback prohibited, E3; A runtime excludes credential code E1 | Endpoint/provider owners D3/D4/D5 assess remaining native/delegated stores and backups. |
| [ATT&CK v19 T1555 — Credentials from Password Stores](https://attack.mitre.org/versions/v19/techniques/T1555/) | T01/T04; legitimate store access can be abused | **Partial/shared:** native stores improve storage, E3; they do not universally isolate the credential owner from same-user processes | Endpoint/IAM D3; approve OS access and token lifecycle. |
| [ATT&CK v19 T1059 — Command and Scripting Interpreter](https://attack.mitre.org/versions/v19/techniques/T1059/) | T03/T08; D's shell/runtime and later agent tools | **Partial/shared:** protected argv/images, custom-shell opt-in, E5; A has no execution E1 | Platform/security D6; approve real runtimes and downstream commands. |
| [ATT&CK v19 T1195.002 — Compromise Software Supply Chain](https://attack.mitre.org/versions/v19/techniques/T1195/002/) | T05; every distributed mode and external tool | **Partial/shared:** provenance/SBOM/scanning/refusal E8/E9 | Release/endpoint D2/D9; defend authorized source/build access and downstream trust. |
| ATLAS 2026.08 AML.T0053 — AI Agent Tool Invocation ([official data](https://github.com/mitre-atlas/atlas-data/blob/41d4f5ca4112f0e492ffaa3ebff07dc80a75afa5/dist/v6/ATLAS-2026.08.yaml)) | T03/T08; D can expose local/remote tool privileges | **Partial/shared:** which-model invocation guards E5/E6; no agent-level authorization proof | Harness/platform owner D6 validates native permissions and approval prompts. |
| ATLAS 2026.08 AML.T0110 — AI Agent Tool Poisoning; AML.T0099 — AI Agent Tool Data Poisoning (same official data) | T08; malicious definitions/implementations/responses versus poisoned retrieved data | **Partial/shared:** shipped integration ownership and typed outputs E5/E6; no semantic poisoning detector or end-to-end agent resistance demonstrated | Platform/security D6 reviews skills/tools and exercises untrusted data/definitions. |
| ATLAS 2026.08 AML.T0051 — LLM Prompt Injection; AML.T0051.001 — Indirect (same official data) | T08; D consumes project and tool content | **External/open for LLM resistance:** parameter validation E5 is not prompt-injection prevention. A's ranker invokes no LLM. | Harness/security D6 supplies synthetic end-to-end abuse evaluation and containment. |
| ATLAS 2026.08 AML.T0086 — Exfiltration via AI Agent Tool Invocation (same official data) | T08; D's legitimate write tool can send sensitive parameters externally | **External/open for agent exfiltration:** inherited-secret reduction E5 limits one input; harness may read its own credentials/files | Security/network/platform D6 validates tool grants, data access and egress. |

## AI RMF evidence

These are selected [AI RMF 1.0 Core](https://airc.nist.gov/airmf-resources/airmf/5-sec-core/)
references for the reported Map/Measure/Manage gap. They do not assert completeness
across every AI RMF outcome. which-model is a deterministic recommender, not a
model-training service; its downstream agent use still has AI-system risks.

| Reference | Existing product evidence | Missing evidence / owner |
|---|---|---|
| MAP 1.1, 2.1, 2.2, 4.1, 4.2 | Four modes, purpose, knowledge limits, external dependencies, data flow, input hashes and threat register E1–E10 | Company intended use, risk tolerance, provider/data rights and affected-user context D1/D3/D8; company AI/privacy owners. |
| MEASURE 1.1, 2.1, 2.7, 2.10 | Threat-linked negative tests, exact CI/artifacts, privacy tests and documented scope/limitations E1–E9 | Independent assessment, live deployment checks, end-to-end agent misuse tests and task-specific quality evaluation D4/D6/D8; security/AI assessors. |
| MANAGE 1.1–1.4, 2.4, 3.1, 4.1 | Prioritized residual register, opt-in capabilities, owned removal, source-bound updates, advisory decision, readiness and pending acceptance record | Actual go/no-go, monitoring, incident/recovery practice, accountable risk acceptance and review cadence D2/D7/D9/D10; company risk/security owners. |
| GOVERN 2.1 (supporting context) | Product/company roles are separated and human sign-off remains explicit | Company appoints accountable people and allocates resources; this repository cannot approve on their behalf. |

Failure-open quota/audit reporting is not credited as an authorization or immutable
audit control in any row. Personal-mode compatibility is intentional. A company
requiring stronger isolation or mandatory audit completeness must supply external
controls or reopen that product decision; this assessment does not override the
requester's non-blocking launch requirement.
