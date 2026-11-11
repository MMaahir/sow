# sow

A Go CLI that generates and updates project repos from one template. It's a personal internal tool, built to learn SRE, DevOps and platform engineering.

## Language

### Templates

**Component**:
An optional part of the template that a project can turn on, such as `backend`, `frontend`, `database`, `observability` or `deploy`.
_Avoid_: layer, module, plugin, feature

**Stack**:
The implementation a component uses, such as `ts` or `go` for `backend`, `react` for `frontend`, or `postgres` for `database`. Every component has at least one.
_Avoid_: flavour, language (on its own)

**Stack interface**:
The rules every backend or frontend stack must meet so one can replace another: shared task names, a Dockerfile, health endpoints, and an OpenAPI document or client.
_Avoid_: layer interface, layer contract

**Preset**:
A named set of answers that `sow new` fills in, such as `minimal` or `web-ts`. It applies only when a project is created.
_Avoid_: profile, stack, flavour

**Answers file**:
The file in a generated project that records the sow version and the answers it was rendered with. There's one per project.
_Avoid_: config, state file

**Settle**:
To turn the answers that flags, a preset, the personal defaults, the prompts or the answers file give into one answer per question: the first source that has a key wins, a question no source answers takes its default, and every rule on the answers is checked. The result is the settled answers.
_Avoid_: resolve, merge, validate (for the whole step)

**Personal defaults**:
The answers that describe the author and stay the same across projects: name, email and GitHub username.
_Avoid_: settings, profile, user config

**Follow-up step**:
A command the CLI runs in a project after it writes files, such as installing dependencies.
_Avoid_: task, hook, post-step

**Old render** and **new render**:
The two renders an update compares. The old render is the template at the project's recorded version, and the new render is the template at the CLI's version, both with the project's answers.
_Avoid_: base, upstream

### Projects

**Generated project**:
A repo created by `sow new` from the template.
_Avoid_: output, scaffold, child project

**Platform**:
The separate repo that runs the local cluster and the shared services every generated project deploys into.
_Avoid_: infra repo, cluster repo

**Platform contract**:
The conventions a generated project can rely on when it deploys to the platform, such as namespace, image naming and telemetry endpoint.
_Avoid_: platform API, interface

**Skills contract**:
What sow needs from the agent skills repo: a marketplace name and the plugins to enable for each component.
_Avoid_: skills config

### Ownership

**Infrastructure file**:
A file the author writes by hand and agents never touch, such as a workflow, a Dockerfile or a Helm chart.
_Avoid_: ops file, DevOps file

**Author**:
The person who owns sow and writes its infrastructure files.
_Avoid_: user, maintainer, human
