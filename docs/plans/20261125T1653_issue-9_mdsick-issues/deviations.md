# Deviations

- Task 1, Step 1: the plan's test expected warnings ordered `9.md`,
  `43.md`, `13.md`, which contradicts both the plan's own "ascending
  natural order" note and the goal's "sort by File (natural)". Fixed the
  expectation to `9.md`, `13.md`, `43.md` (ascending natural); the
  implementation keeps `natural.Compare` ascending for both columns.
- Task 2, Step 1 (beyond plan): `TestBuild` in `run_test.go` counted the
  index anchors and now sees a fifth one — the warnings table's
  `<a href="43.html">#43</a>` row. Raised the expected count from 4 to 5
  and updated the message; otherwise the plan's assertions were applied
  as written.
