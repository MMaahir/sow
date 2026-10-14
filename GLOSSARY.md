# sow

A CLI that generates and updates project repos from one Copier template. It's a personal internal tool, built to learn SRE, DevOps and platform engineering.

## Language

### Templates

**Component**:
An optional part of the template that a project can turn on, such as `backend`, `frontend`, `database`, `observability` or `deploy`.
_Avoid_: layer, module, plugin, feature

**Stack**:
The implementation a component uses, such as `python` or `ts` for `backend`, or `react` for `frontend`.
_Avoid_: flavour, language (on its own)

**Stack interface**:
The rules every backend or frontend stack must meet so one can replace another: shared task names, a Dockerfile, health endpoints, and an OpenAPI document or client.
_Avoid_: layer interface, layer contract

**Preset**:
A named set of answers that `sow new` fills in, such as `minimal` or `web-python`. It applies only when a project is created.
_Avoid_: profile, stack, flavour

**Answers file**:
The file in a generated project that records the template version and the answers it was rendered with. There's one per project.
_Avoid_: config, state file

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
