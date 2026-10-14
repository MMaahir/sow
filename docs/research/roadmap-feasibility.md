# Roadmap feasibility checks

> **Historical.** This note reviewed the earlier layered-template plan. ADR 0009 (one template with components) and ADR 0010 (rules for running Copier) have since settled template sourcing, the update procedure and the task policy, so the sections on multi-template composition, update transactions and task trust no longer describe the current plan. The source checks on Copier, GitHub, AWS and the toolchain are still useful.

Checked 2026-10-14 against primary sources. These are findings and proposed checks for the roadmap review, not new project decisions. No generated project or release pipeline has been exercised. Claims about a possible implementation are conditional because the roadmap does not specify that implementation.

## Copier source tracking and composition

Copier documents applying multiple templates to one project with a separate answers file for each. Each template must render its answers file through `{{ _copier_conf.answers_file }}.jinja`. Copier recommends one template per Git repository; split mirrors fit that recommendation. Its `_subdirectory` option separates a template's metadata from its rendered content. It is not a general selector for independent templates in a monorepo. [Copier configuration](https://copier.readthedocs.io/en/stable/configuring/#applying-multiple-templates-to-the-same-subproject), [subdirectory setting](https://copier.readthedocs.io/en/stable/configuring/#subdirectory).

Passing a normal `templates/<layer>/` directory inside the monorepo directly to Copier 9.19.1 does not track the monorepo's commit. `get_repo()` recognizes a local Git repository only when the supplied directory is its root. `Template.commit` returns `None` for a non-Git source, and the answers writer omits that `_commit`. `run_update()` rejects a project with no old template reference. This follows the released source code, rather than a runtime reproduction. [Git source detection](https://github.com/copier-org/copier/blob/v9.19.1/copier/_vcs.py), [template commit detection](https://github.com/copier-org/copier/blob/v9.19.1/copier/_template.py), [answers writer and update validation](https://github.com/copier-org/copier/blob/v9.19.1/copier/_main.py).

The roadmap leaves the 0.1 source strategy open. It therefore does not establish that every 0.1 project will be unupdatable. If 0.1 uses ordinary local subdirectories or bundled files, the reported problem applies. A tagged split repository or another defined Git source strategy needs a proof before 0.1 is treated as a valid baseline for 0.2 updates. Suggested acceptance check: generate with the actual 0.1 delivery method, inspect each `_src_path` and `_commit`, then update that saved project through the actual 0.2 source path.

## Updates need a transaction policy

Copier 9.19.1 refuses to update a dirty destination repository. A first layer update normally leaves changes, so a simple loop over answers files cannot safely implement the stated command. Per-layer commits or isolated temporary Git history are possible approaches. Neither approach is selected by the roadmap. [Released update validation](https://github.com/copier-org/copier/blob/v9.19.1/copier/_main.py).

Copier describes conflict handling, deleted-path handling and broken-update recovery. It promises no universal clean merge. Updates can leave inline conflicts or reject files. The old template generation can also fail to replay. `git merge --abort` is not its rollback procedure. The roadmap's clean-merge test can cover unchanged fixtures, but it cannot establish preservation of all user edits. [Copier update behavior](https://copier.readthedocs.io/en/stable/updating/).

Suggested checks before 0.2: an edited generated file, a deleted file, an intentional conflict, failure in a later layer, repeat execution, and a skipped release. Specify how the CLI stops, reports partial work, restores the original state and prevents a successful result when conflicts remain. Update every layer to one explicit release ref; letting each independently choose its newest tag does not enforce lockstep during a partially published release.

## Tasks and trust affect update reproducibility

Copier disables tasks, migrations and Jinja extensions by default because they can run arbitrary code. Its configuration docs say skipping tasks does not skip migrations. During an update, Copier regenerates the old template, runs tasks, applies the update and replays the project's changes. Tasks that depend on a mutable network service can prevent replay of an older release. [Unsafe features and task controls](https://copier.readthedocs.io/en/stable/configuring/#unsafe), [update process](https://copier.readthedocs.io/en/stable/updating/#how-the-update-works).

Suggested policy to discuss: keep template rendering deterministic, run project setup after the complete composition, and define which template sources the CLI may trust. Pin Copier itself and verify its exact API behavior. There is no tested task or trust policy in the current roadmap.

## Tool choices mostly need compatibility gates

Copier 9.19.1 lists Python 3.14 support and requires Python 3.10 or newer. Python 3.14 is a supported choice. The absence of a 3.15 classifier does not prove that 3.15 cannot execute Copier; 3.15 compatibility was not established by this check. Prefer a dated supported-version statement over the roadmap's categorical explanation. [Copier 9.19.1 metadata](https://pypi.org/project/copier/9.19.1/).

TypeScript 7.0 shipped on 2026-07-08 through the `typescript` package with a `tsc` executable. Calling it alpha would be wrong. The release announcement says 7.0 does not have a programmatic API and documents TypeScript 6 compatibility aliases for tools that need one. This does not prove the selected Vite, Hono, Prisma and client-generator versions work together. [TypeScript 7.0 announcement](https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/).

ty's beta announcement permits production use for motivated users, and Oxfmt has reached beta. They do not conflict with the decision to allow pinned pre-1.0 tools while excluding alpha tools. Exact installed versions and the whole generated toolchain remain unverified. [Astral's ty announcement](https://astral.sh/blog/ty), [Oxfmt beta announcement](https://oxc.rs/blog/2026-02-24-oxfmt-beta).

## GitHub automation needs an explicit route

Protected-branch requirements also govern integrations with push access. If generated `main` requires PRs and successful checks without bypass, CI cannot simply push its new image-tag commit there. Suggested options for discussion are an image-update PR, a separate GitOps branch or repository, or an explicitly scoped exception. Define concurrency and ensure an image update does not cause an endless build loop. [GitHub protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches).

The release-please README still says resources created with `GITHUB_TOKEN` do not trigger workflows. Current GitHub docs are more precise: PR `opened`, `synchronize` and `reopened` events can create runs that need approval; pushes and release events still do not trigger follow-on runs. App tokens can run those PR checks automatically. A release workflow can also publish using release-please outputs in the same workflow. Specify the token and publishing route instead of assuming a tag-triggered publisher will run. [Release-please action](https://github.com/googleapis/release-please-action), [current GitHub trigger rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow).

## AWS identity and cost assumptions

An OIDC trust policy limited to `main` and tags does not authenticate a normal PR identity. Environments can change the subject format, and repository identity formats have also changed. The proposed PR Terraform plan therefore needs a defined credential route and permissions distinct from apply. Merely changing checkout to `main` does not change the workflow's identity. [AWS OIDC configuration](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-in-aws), [GitHub OIDC subject examples](https://docs.github.com/en/actions/reference/security/oidc#example-subject-claims).

A $1 budget alert is not a $1 spending limit. AWS notes billing and notification delays, and charges can continue after a threshold. The zero-cost intention needs an estimated service budget and a cleanup procedure, with the alert described as monitoring. [AWS Budgets behavior](https://docs.aws.amazon.com/cost-management/latest/userguide/budgets-managing-costs.html).

## Caveats for the additional review

Renovate config alone does not install or authorize the hosted GitHub App. An installation covering all repositories can cover a newly created repo. For a selected-repository installation, adding access can be automated with suitable credentials. Current GitHub docs list App user tokens, App installation tokens and fine-grained PATs with the required installation-repository-access write permission. They also require the authenticated user to have admin access. A blanket claim that only classic user tokens work is outdated. Do not assume an arbitrary `gh` token can do it. [Renovate installation](https://docs.renovatebot.com/getting-started/installing-onboarding/), [current GitHub installation-access endpoint](https://docs.github.com/en/rest/apps/installations#add-a-repository-to-an-app-installation).

The upstream skills README recommends `mattpocock-skills@claude-plugins-official` for Claude. Its own marketplace manifest names `mattpocock` and declares `mattpocock-skills`. These are different distribution routes; seeing different IDs does not establish that either is invalid. Select a route and verify the generated settings against that route. Claude's successful installation does not establish Codex's installation. [Upstream installation instructions](https://github.com/mattpocock/skills), [upstream marketplace manifest](https://raw.githubusercontent.com/mattpocock/skills/main/.claude-plugin/marketplace.json).

`pypa/gh-action-pypi-publish` generates and uploads signed attestations by default with Trusted Publishing. The 0.2 publishing implementation can therefore already include PyPI distribution attestations. This does not establish SLSA build provenance or signatures for generated container images. The roadmap should state which artifact and guarantee the later signing milestone adds. [PyPA action attestation behavior](https://github.com/pypa/gh-action-pypi-publish#generating-and-uploading-attestations), [PyPI attestation documentation](https://docs.pypi.org/attestations/).

## Unresolved evidence

No runtime proof was run for layered composition, update rollback, installed tool compatibility or remote GitHub settings. The exact Prisma major and support window, client generator, Zensical generator status, local ARM runner configuration and AWS service costs were not resolved. Recheck mutable sources when implementing the relevant release. Source-backed concerns above are reasons to add decisions and acceptance checks, not proof that the architecture cannot work.
