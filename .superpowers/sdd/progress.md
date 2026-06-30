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
