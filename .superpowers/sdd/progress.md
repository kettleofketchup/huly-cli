# huly-cli SDD progress ledger

Plan: docs/superpowers/plans/2026-06-25-huly-cli.md
Branch: master  | GitHub repo: public, created+pushed at end (Task 2 last)

Task 1: complete (scaffold commit 861603f, builds+vets clean, inherited cmds present)
Task 3: complete (commit 847f6f0, review clean; MINOR: ids.go cap hint 28→16, defer to final)
Task 4: complete (commit cfc7ada, review clean; MINOR: Issue.Priority int, String() untested — defer)
Task 5: complete (commit dd1dfbf, review clean; MINOR: map mutation in NewCreateIssueTx, NewUpdateDocTx no nil-guard on ops, test == "" type-assert — defer)
Task 6: complete (commit 7364910, verified inline: exact errors.go, builds)
Task 7: complete (commits b1af69d+daad34a, review clean after fix; IMPORTANT fixed: request-construction error handling; MINOR: LoadServerConfig untested, bearer/kind untested — defer)
Task 8: complete (commits 4b6f310+e583c55, review clean after fix; IMPORTANT fixed: 201/204 success; MINOR: json.Marshal err ignored, Retry-After date form, test gaps — defer). Phase 2 (client) done.
Task 9: complete (commit b7b3c34, review clean; MINOR: cred-file TOCTOU umask window before chmod [security, consider hardening], os.IsNotExist vs errors.Is, test discards Save err — defer to final)
Task 10: complete (commit 0bebe6f, review clean; MINOR: whoami dup newClient, bare err wrap in runLogin — defer)
Task 11: complete (commit ee24d21, verified inline: exact auth.go, builds+vets+tests). Phase 3 (auth) done.
Task 12: complete (commit 109fa55, verified inline: exact output.go, tests+vet pass)
Task 13: complete (commit 135356f, review clean; MINOR: Log field no mapstructure tag, rootCmd PersistentPreRunE skipped if subcmd defines own [none do] — defer). Phase 4 done.
Task 14: complete (commit 87e2423, review clean; MINOR: tmp leak on rename failure, os.IsNotExist — defer)
Task 15: complete (commit 0659796, verified inline: filterPrefix safe, tests+vet pass). Phase 5 done.
Task 16: complete (commit 450c1f1, review clean; MINOR: cache.Load err discarded, prune branch untested — defer)
Task 17: complete (commit 1351aa2, verified inline: exact project.go, tests+vet pass, completion resolves cleanly)
Task 18: complete (commits b04f09b+b6ecd48; IMPORTANT fixed: component cache write-through now stores human project identifier, consistent with sync)
Task 19: complete (commit 6de3b34, verified inline: createMilestone correct, identifier write-through, target-date parse, --output, tests+vet pass)
Task 20: complete (commit ca78b29, review clean; MINOR: issue write-through omits Project [if fixed, store pr.Identifier NOT projectRef to match components/sync], test doesn't assert attachedToClass — defer to final). Phase 6 done.
Task 21: complete (commit 421447a, review clean; MINOR: projectFilter branch untested, non-deterministic sync slice order — defer). All code tasks (3-21) done.
Task 22: complete (commit 7c5f9bd, README accurate; verified: full suite+vet pass, completion smoke clean exit 0, --help lists 13 commands)
Task 23: complete (commit 1f831ca, Zensical build OK, public/ gitignored, Go green; NOTE: docs/superpowers specs render into site - minor, exclude later). Phase 7 done. All code+docs tasks complete.
