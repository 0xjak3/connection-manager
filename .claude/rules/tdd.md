# TDD workflow (applies to every feature and bug fix)

Work is driven by tests from the outside in. An e2e test, written from the user's point of view, defines "done". Unit tests drive the design inside.

## The loop

0. **Contract first.** Add or change the operations and schemas in `openapi.yaml` (`.claude/rules/openapi.md`). This is the spec that scenarios and code follow.
1. **Red: e2e first.** Write e2e scenarios in `test/e2e/` from the spec (`.claude/rules/e2e.md`). Run `make test-e2e` and confirm they fail **for the right reason**: a missing route (404/405), a wrong status, or a wrong body. A failure from a compile error, a container that didn't start, or a broken harness doesn't count. Fix the harness, then re-run. Keep the failing output for the report.
2. **Implement, inner TDD per layer.** Go in the order of `.claude/skills/new-feature/SKILL.md`. For domain and usecase, write the unit test first, watch it fail, then make it pass. Handler, router and repository tests are written alongside their code (`.claude/rules/testing.md`).
3. **Green.** `make test-e2e` passes, and so does `make check` (and `make test-integration` if the repository changed).
4. **Refactor** with everything green, then re-run steps 3's commands.

## Rules

- No production code for a behaviour until a failing e2e scenario for it exists. A bug fix starts with an e2e (or, if not observable through the API, a unit) test that reproduces it.
- Refactors and extra coverage of behaviour that already works: new scenarios may pass straight away (say so in the report). Existing e2e must stay green the whole time.
- While implementing, don't edit `openapi.yaml` or e2e scenarios to make them pass. If a scenario looks wrong (it contradicts the spec, or asserts something the spec never said), stop and report it. The spec owner decides, then the scenario changes first and goes red again.
- One behaviour at a time: you may write several scenarios up front, but implement until they're all green before adding unrelated work.
- The report must show both runs: the red output (step 1) and the green output (step 3).
