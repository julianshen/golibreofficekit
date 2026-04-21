# testdata

Fixtures used by integration tests (behind the `lok_integration`
build tag).

- `hello.odt` — trivial Writer document, "Hello from Phase 3."

Regenerate only if the file corrupts. LO version drift may change
bytes, but the tests only check type and non-empty output, so exact
byte-identity is not required.

Regeneration recipe:

```bash
tmpdir=$(mktemp -d)
echo "Hello from Phase 3." > "$tmpdir/hello.txt"
soffice --headless --convert-to odt --outdir "$tmpdir" "$tmpdir/hello.txt"
cp "$tmpdir/hello.odt" testdata/
```
