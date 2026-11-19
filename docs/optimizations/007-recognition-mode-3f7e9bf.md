# 7. Recognition Mode (3F7e9bf)

- `Recognize` (`zp parse -recognize`) runs a separate program derived from the same grammar that builds no values
  and skips the actions that no predicate needs, and returns the same syntax errors. Rules whose values predicates read still run normally.
- Effect: 1.5–1.8× faster than a full parse (JSON 78 → 44 ms). Compare it with `json.Valid`, not with parsers that
  build trees.
