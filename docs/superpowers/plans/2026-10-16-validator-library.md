# Validator library (Part 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `ligneous-gedcom-lib` read real-world GEDCOM files correctly, with line numbers on everything, plain-English explanations and a safe line-level repair, ready to be compiled to WebAssembly for validate.ligneous.org.

**Architecture:**
- The parser gains a decoding front end that works out the encoding and converts to UTF-8.
- It also gains a line splitter that handles every line-ending style and a positional tokeniser.
- Findings get stable codes and severities.
- Line numbers ride on every `GedcomRecord`.
- Two new packages, `explain` (the copy catalog) and `repair` (line-level fixes), sit beside `validator`.

**Tech Stack:** Go 1.23 (`go.mod` stays `go 1.23.5`), standard library, `github.com/google/uuid`, and new `golang.org/x/text v0.28.0`. That's the newest x/text that keeps `go 1.23`; v0.29+ needs 1.24.

**Spec:** `docs/validator-site-spec.md`, Part 1 (sections 1.1–1.9) and the Appendix catalog.

## Global Constraints

- Work only in the worktree `~/gedcom-lib-validate`, branch `validate-ready`. Never touch `/srv/apps/gonsalves/ligneous-gedcom-lib`, which is the main checkout on `master`.
- `go.mod` keeps `go 1.23.5`. Run `go mod tidy` after adding x/text, and check that the `go` line didn't change.
- Every existing test must keep passing unchanged. If an assertion has to change, the commit message says why.
- The default `parser.Parse` behaviour for well-formed UTF-8 files is unchanged:
  - same tree;
  - same warnings, except that each now carries a Code and a Severity;
  - fatal errors still return `err != nil` when `Lenient` is false.
- JSON changes are additions only: `line` on `GedcomRecord`, `Line` on `ValidationError`, `Code`/`Severity` on `ParseError`, and `encoding`/`lineCount` on `GedcomDocument`. No existing key is renamed or removed.
- Severity strings in `ParseError`: `"error"`, `"warning"`, `"hint"`.
- Test command for every task: `go test ./...` (from the worktree root). Also `go vet ./...` must be clean.
- Commit after each task on `validate-ready`. Never push, merge or tag (Task 7 is momolig's call).
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Missing CHAR and non-UTF-8 bytes.** A file with no `CHAR` line that is really Windows-1252 must be read as Windows-1252 with `ENCODING_UNDECLARED` + `ENCODING_MISLABELED`, not read as broken UTF-8. Test in Task 2: `TestDecodeUndeclaredLatin1`.
2. **Mixed line endings.** Mostly `\r\n` with a few bare `\r` (files edited on two systems) must split at every one. Test in Task 1: `TestSplitMixedLineEndings`.
3. **BOM disagreeing with CHAR.** A UTF-8 BOM with `1 CHAR ANSEL`: the BOM wins, with finding `ENCODING_MISLABELED`. Test in Task 2: `TestDecodeBOMBeatsChar`.
4. **Empty values.**
   - `1 CONC`, `2 DATE` and `1 NAME ` (trailing space) must give `Value == ""`. They must not panic or give " ".
   - `1 CONC  ` (two spaces) must give `" "`.

   Test in Task 1: `TestTokeniseEmptyValues`.
5. **Speed on real files.** `BenchmarkParseValidateEnrich` (existing, validator/benchmark_test.go) on `royal92.ged` must stay within 1.5× of the `master` figure. Each task that touches the parser records before/after in its commit message. Check in Tasks 1, 2 and 4.

---

### Task 1: Codes on parse findings, line splitting and positional tokenising

**Files:**
- Modify: `parser/errors.go`: add `Code`, `Severity`; add the code constants.
- Modify: `parser/parser.go`: `parseLines`, `parseSingleLine`.
- Create: `parser/split.go`: line splitter.
- Modify: `enricher/enricher.go:232,353,483,831`: trim values where names are read, and leave the CONC join untouched.
- Test: `parser/split_test.go`, `parser/parser_test.go`, `enricher/enricher_test.go`.

**Interfaces:**
- Produces (in `parser`):
  ```go
  type ParseError struct { Line int; Message, Context string; Err error; Code string; Severity string }
  const (
      CodeEmptyLine = "EMPTY_LINE"; CodeLineTooLong = "LINE_TOO_LONG"; CodeLeadingWhitespace = "LEADING_WHITESPACE"
      CodeBadLine = "BAD_LINE"; CodeBadLevel = "BAD_LEVEL"; CodeXrefWithoutTag = "XREF_WITHOUT_TAG"; CodeTooDeep = "TOO_DEEP"
      CodeReadError = "READ_ERROR"
  )
  func splitGedcomLines(data []byte, atEOF bool) (advance int, token []byte, err error) // bufio.SplitFunc
  ```
- Severities:
  - `EMPTY_LINE`: warning
  - `LINE_TOO_LONG`: warning
  - `LEADING_WHITESPACE`: warning
  - `BAD_LINE`: error
  - `BAD_LEVEL`: error
  - `XREF_WITHOUT_TAG`: error
  - `TOO_DEEP`: error
  - `READ_ERROR`: error

- [ ] **Step 1: Write the failing tests**
  - `TestSplitLineEndings`: the input `"0 HEAD\r1 GEDC\r\n2 VERS 5.5.1\n0 TRLR"` gives 4 tokens with no `\r` or `\n` in them.
  - `TestSplitMixedLineEndings` (Review Focus 2): 1000 lines ending in `\r\n`, with every 10th ending in a bare `\r`, give 1000 tokens.
  - `TestParseCROnlyFile`: `"0 HEAD\r1 GEDC\r2 VERS 5.5.1\r0 @I1@ INDI\r1 NAME A /B/\r0 TRLR\r"` gives `len(doc.Individuals)==1`, `doc.Trailer.Tag=="TRLR"`, and `doc.Header.Value==""`.
  - `TestTokeniseXrefContainingTag`: `0 @NOTE1@ NOTE Hello there` gives `Notes[0].Value=="Hello there"`, `Xref=="@NOTE1@"`.
  - `TestTokeniseConcKeepsLeadingSpace`: `1 CONC  world` gives the child value `" world"`.
  - `TestTokeniseEmptyValues` (Review Focus 4): `1 CONC` → `""`, `1 NAME ` → `""`, `1 CONC  ` → `" "`, `2 DATE` → `""`.
  - `TestLeadingWhitespaceWarning`: `"  1 NAME A /B/"` parses, and gives exactly one warning with `Code=="LEADING_WHITESPACE"`, `Line==N`.
  - `TestLongLineOver64KB`: a 70,000-byte NOTE value parses (no `token too long`), with one `LINE_TOO_LONG` warning.
  - Existing `TestParseEmptyLine`, `TestParseInvalidLevel` and `TestParseMissingTag` get extra assertions: `Code` is `EMPTY_LINE`, `BAD_LEVEL` and `BAD_LINE`, and the `Severity` is right.
  - `TestEnrichNameWithDoubleSpace` (enricher): `1 NAME  John /Doe/` enriches to the given name `John`, surname `Doe`, full name with no leading space.
- [ ] **Step 2: Run `go test ./parser/ ./enricher/` and check the new tests fail.** Expected: wrong values and counts, `token too long`, and Code `""`.
- [ ] **Step 3: Implement.**
  - `splitGedcomLines` treats `\r\n`, `\n` and a lone `\r` as one terminator each. At EOF it returns the remaining bytes.
  - `parseLines` uses it with `scanner.Buffer(make([]byte, 64*1024), 1<<20)`.
  - Tokenise by position:
    1. skip leading spaces and tabs, and warn if there were any;
    2. read the level digits, then one or more spaces;
    3. if the next token starts and ends with `@`, it's the xref, followed by one or more spaces;
    4. read the tag (up to the next space or the end);
    5. if one more space follows, the value is the rest of the line, verbatim.
  - Fill `Code`/`Severity` on every `newParseError`/`wrapParseError` call.
  - In the enricher, `strings.TrimSpace` the NAME values at the listed lines. Do not trim CONC/CONT joins (line 831ff).
- [ ] **Step 4: Run `go test ./... && go vet ./...`.** Expected: everything passes, old and new.
- [ ] **Step 5: Benchmark** with `go test -bench=ParseValidateEnrich -run=^$ ./validator/`. Record the before (on `master`) and after figures.
- [ ] **Step 6: Commit** `fix(parser): split on every line ending, tokenise by position, codes on parse findings`, with the benchmark numbers in the body.

### Task 2: Encodings

**Files:**
- Create: `gedcom/encoding.go`: the `EncodingInfo` type.
- Modify: `gedcom/types.go`: add `Encoding EncodingInfo \`json:"encoding"\``, `LineCount int \`json:"lineCount"\`` to `GedcomDocument`.
- Create: `parser/decode.go`: detection and conversion.
- Create: `parser/ansel.go`: the ANSEL table and decoder.
- Modify: `parser/parser.go`: `ParseWithOptions` decodes first; `stripBOM` is folded into decode.
- Modify: `go.mod`/`go.sum`: `golang.org/x/text v0.28.0`.
- Create fixtures `testdata/encodings/`, made with `printf`/`iconv` and committed as bytes:
  - `ansel.ged`: `CHAR ANSEL`, the name `Ren\xE2e /Dupr\xE2e/`, a place `Z\xE8urich`;
  - `ansi.ged`: `CHAR ANSI`, `Ren\xE9`;
  - `utf16le-bom.ged`;
  - `utf16be-nobom.ged`;
  - `mislabeled-utf8-as-ansel.ged`;
  - `undeclared-latin1.ged`;
  - `bom-utf8-char-ansel.ged`.
- Test: `parser/decode_test.go`, `parser/ansel_test.go`.

**Interfaces:**
- Produces:
  ```go
  // gedcom
  type EncodingInfo struct { Detected string `json:"detected"`; Declared string `json:"declared"`; Reason string `json:"reason"` }
  // parser
  const (CodeEncodingNotUTF8 = "ENCODING_NOT_UTF8"; CodeEncodingMislabeled = "ENCODING_MISLABELED"
         CodeEncodingUndeclared = "ENCODING_UNDECLARED"; CodeEncodingBadBytes = "ENCODING_BAD_BYTES")
  func Decode(raw []byte) (utf8 []byte, info gedcom.EncodingInfo, findings []*ParseError)
  func decodeANSEL(b []byte) (s string, bad int)
  ```
- Detected names, exactly: `"UTF-8"`, `"UTF-16LE"`, `"UTF-16BE"`, `"ANSEL"`, `"Windows-1252"`, `"CP437"`, `"MacRoman"`, `"ASCII"`.
- Severities:
  - `ENCODING_NOT_UTF8`: hint
  - `ENCODING_MISLABELED`: warning
  - `ENCODING_UNDECLARED`: hint
  - `ENCODING_BAD_BYTES`: warning
- Details needed later by `explain`: the encoding findings need `encoding`, `declared` and `count`. ParseError has no Details map, so add `Details map[string]string \`json:"Details,omitempty"\`` to `ParseError` here.

- [ ] **Step 1: Write the failing tests**
  - `TestANSELCombining`:
    - `E2 65` → `é`;
    - `E8 75` → `ü`;
    - `F0 63` → `ç`;
    - two marks `E3 E2 61` → `a` plus both marks, NFC-composed where it can be (assert `norm.NFC.IsNormalString`);
    - a trailing mark at the end of the input → dropped and counted in `bad`.
  - `TestANSELSpecials`:
    - `A5` → `Æ`, `B5` → `æ`, `A2` → `Ø`, `B2` → `ø`;
    - `A1` → `Ł`, `B1` → `ł`;
    - `CF` → `ß`, `C3` → `©`, `B9` → `£`;
    - an undefined byte `0xFF` → `�`, with `bad==1`.
  - `TestDecodeTable`, table-driven over the fixtures. It asserts `info.Detected`, `info.Reason`, the decoded name (`René /Dupré/`, `Zürich`) and the finding codes:
    - `ansel.ged`: ANSEL, `header CHAR`, [ENCODING_NOT_UTF8];
    - `ansi.ged`: Windows-1252, [ENCODING_NOT_UTF8];
    - `utf16le-bom.ged`: UTF-16LE, `byte order mark`, [ENCODING_NOT_UTF8];
    - `utf16be-nobom.ged`: UTF-16BE, `byte pattern`, [ENCODING_NOT_UTF8];
    - `mislabeled-utf8-as-ansel.ged`: UTF-8, [ENCODING_MISLABELED];
    - plain UTF-8 with `CHAR UTF-8`: no findings.
  - `TestDecodeUndeclaredLatin1` (Review Focus 1): Windows-1252, with [ENCODING_UNDECLARED, ENCODING_MISLABELED, ENCODING_NOT_UTF8].
  - `TestDecodeBOMBeatsChar` (Review Focus 3): UTF-8, [ENCODING_MISLABELED].
  - `TestParseSetsEncodingAndLineCount`: `Parse` on `ansel.ged` gives `doc.Encoding.Detected=="ANSEL"` and `doc.LineCount` = the number of lines, with the decode findings included in `warnings`.
- [ ] **Step 2: Run `go test ./parser/`.** Expected: FAIL, because nothing is defined.
- [ ] **Step 3: Implement.** Follow spec 1.3 in order: BOM, UTF-16 byte pattern, HEAD.CHAR scan over the first 50 raw lines, the check against the bytes, then convert.
  - The ANSEL table follows ANSI/NISO Z39.47 plus the GEDCOM 5.5.1 extensions (`BE`, `BF`, `CD`, `CE`, `CF`ß); look up the exact characters in the GEDCOM 5.5.1 spec, Appendix D. The tests above pin the subset that matters.
  - Use `golang.org/x/text/encoding/charmap` (Windows1252, CodePage437, Macintosh), `golang.org/x/text/encoding/unicode` for UTF-16, and `golang.org/x/text/unicode/norm`.
  - Count U+FFFD from invalid input as `ENCODING_BAD_BYTES`, with Details `count`, and `Line` set to the first line where it happened.
- [ ] **Step 4: Run `go test ./... && go vet ./...`.** Expected: PASS. Then run `go mod tidy && grep '^go ' go.mod`. Expected: `go 1.23.5`.
- [ ] **Step 5: WebAssembly size check.**
  - Build `GOOS=js GOARCH=wasm go build -ldflags='-s -w' -o /tmp/claude-1000/-home-momolig/11c1c6b1-3f9f-4cf2-9576-83cd1966f71f/scratchpad/wasm-size/{before,after}.wasm ./cmd/gedvalidate` before and after the change.
  - If `cmd/gedvalidate` doesn't build for wasm, use the probe at `/tmp/claude-1000/-home-momolig/11c1c6b1-3f9f-4cf2-9576-83cd1966f71f/scratchpad/wasmprobe`. Point its `replace` at this worktree.
  - Compare `gzip -9c | wc -c`. **Pass: under 100,000 bytes added.** If it's over, replace charmap with three hand-written 128-entry tables in `parser/codepages.go` and measure again.
- [ ] **Step 6: Benchmark** as in Task 1 and record the numbers.
- [ ] **Step 7: Commit** `feat(parser): detect and decode GEDCOM encodings (ANSEL, ANSI, UTF-16, code pages)`, with the size and benchmark numbers in the body.

### Task 3: Structure findings and lenient mode

**Files:**
- Modify: `parser/options.go`: `Lenient bool`.
- Modify: `parser/parser.go`: lines before HEAD, level jumps, unknown records, lenient skip.
- Test: `parser/parser_test.go`; a new fixture `testdata/malformed/structure.ged`.

**Interfaces:**
- Produces:
  ```go
  const (CodeLinesBeforeHead = "LINES_BEFORE_HEAD"; CodeLevelJump = "LEVEL_JUMP"; CodeUnknownRecord = "UNKNOWN_RECORD")
  type Options struct { Context context.Context; StrictMode bool; MaxNestingDepth int; Lenient bool }
  ```
- Severities and Details:
  - `LINES_BEFORE_HEAD`: warning. Details: `count`. Line: the first such line.
  - `LEVEL_JUMP`: warning. Details: `from`, `to`.
  - `UNKNOWN_RECORD`: hint. Details: `tag`.

- [ ] **Step 1: Write the failing tests**
  - `TestLinesBeforeHead`: two junk lines, then a valid file. One finding, `Line==1`, `Details["count"]=="2"`. The tree is unchanged.
  - `TestLevelJump`: `1 BIRT` then `3 DATE 1900`. One `LEVEL_JUMP`, on the DATE line, with `from=1,to=3`. The DATE is still attached under BIRT, as today.
  - `TestUnknownTopLevelRecord`: `0 @X1@ _EVDEF` with children. One `UNKNOWN_RECORD`, `tag=_EVDEF`. Not in any slice.
  - `TestLenientSkipsBadLines`:
    - with `Lenient:true`, a file whose line 5 is `ABC NAME x` and line 9 is `1` gives `err==nil`, both lines reported as `BAD_LEVEL`/`BAD_LINE` with severity error, and every other record parsed;
    - with `Lenient:false`, the same file still returns `err` at line 5 (the existing behaviour).
- [ ] **Step 2: Run `go test ./parser/`.** Expected: FAIL.
- [ ] **Step 3: Implement.**
  - In `parseLines`, a line error under `Lenient` is appended to `warnings` and the line is skipped.
  - Level jumps are detected while building blocks, using the previous line's level.
  - `classifyRecord`'s default branch reports `UNKNOWN_RECORD`. That needs the warnings slice, which `buildDocument` already receives.
- [ ] **Step 4: Run `go test ./... && go vet ./...`.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(parser): report lines before HEAD, level jumps, unknown records; lenient mode`.

### Task 4: Line numbers on nodes and findings

**Files:**
- Modify: `gedcom/types.go`: `Line int \`json:"line,omitempty"\`` on `GedcomRecord`.
- Modify: `parser/parser.go`: `buildRecord` copies `lineNumber`.
- Modify: `validator/validator.go`, `validator/date_checks.go`: set `Line` from the exact node; add the missing Details; run a final fill-in pass.
- Test: `validator/validator_test.go`, `parser/parser_test.go`.

**Interfaces:**
- Produces:
  ```go
  type ValidationError struct { /* existing */ Line int `json:"Line,omitempty"` }
  ```
- New Details:
  - `death_year`, `birth_year`: DEATH_BEFORE_BIRTH;
  - `target_tag`: INVALID_ASSOCIATE_TARGET;
  - `sex`: INVALID_SEX;
  - `family`: the family's xref, on every finding raised while looping over families (ORPHANED_HUSB/WIFE/CHIL, the child/parent date checks, MARRIAGE_*, SWAPPED_*, EMPTY_FAMILY).
- Exact lines:
  - date findings → the DATE child of the event (birth, or the event named in `event`);
  - ORPHANED_* → the pointer line;
  - NAME_* → the NAME line;
  - INVALID_SEX → the SEX line;
  - ASSO findings → the ASSO line.
- Fill-in pass at the end of `ValidateWithOptions`:
  - `Line==0` with `Xref` set → that record's level-0 line;
  - MISSING_HEADER → 1;
  - MISSING_TRAILER → `doc.LineCount`.

- [ ] **Step 1: Write the failing tests**
  - `TestRecordLineNumbers`: on a small file, `Individuals[0].Line==4`, its NAME child `.Line==5`, and its BIRT/DATE grandchild has the right line.
  - `TestEveryFindingHasLine`: over `testdata/*.ged` and `testdata/malformed/*.ged`, with `DateConsistency:true`, every finding has `Line>0`.
  - `TestFindingLinePointsAtTag`: on a fixture with one of each, the line content at `Line` (from the raw file) contains the expected tag:
    - DATE for MOTHER_TOO_YOUNG and DEATH_BEFORE_BIRTH;
    - FAMC for ORPHANED_FAMC;
    - NAME for NAME_MISSING_SURNAME_SLASH;
    - SEX for INVALID_SEX;
    - ASSO for ORPHANED_ASSOCIATE_XREF.
  - `TestNewDetails`: DEATH_BEFORE_BIRTH has `death_year`/`birth_year`, INVALID_SEX has `sex`, INVALID_ASSOCIATE_TARGET has `target_tag`, and ORPHANED_CHIL has `family`.
- [ ] **Step 2: Run `go test ./...`.** Expected: FAIL.
- [ ] **Step 3: Implement** as described in the Interfaces block. Build an `xref → Line` map once, in the fill-in pass.
- [ ] **Step 4: Run `go test ./... && go vet ./...`.** Expected: PASS. Then benchmark as in Task 1.
- [ ] **Step 5: Commit** `feat: line numbers on every node and finding; details the explanations need`.

### Task 5: `explain` catalog

**Files:**
- Create: `explain/explain.go`: types, `Lookup`, `All`, `Placeholders`.
- Create: `explain/catalog.go`: the entries, with the wording copied **verbatim** from the spec Appendix.
- Test: `explain/explain_test.go`.

**Interfaces:**
- Produces:
  ```go
  type Entry struct { Code, Group, Title, Meaning, HowToFix string }
  func Lookup(code string) (Entry, bool)
  func All() []Entry                          // in catalog order
  func Placeholders(e Entry) []string         // names inside {…} across Title, Meaning, HowToFix, deduplicated
  ```
- Groups, exactly: `"format"`, `"header"`, `"links"`, `"dates"`, `"names"`.
- The Appendix says "(same as LINE_TOO_LONG)" for LINE_EXCEEDS_MAX_PHYSICAL_LENGTH. Copy that entry's text rather than writing "(same)".
- Role placeholders the page fills: `person`, `child`, `father`, `mother`, `target`, `family`.

- [ ] **Step 1: Write the failing tests**
  - `TestCatalogCoversEveryCode`: every `validator.Code*` constant (use a list in the test that mirrors `codes.go`, plus a check that `codes.go` has the same count via `go/parser` on the file), `LINE_EXCEEDS_MAX_PHYSICAL_LENGTH`, and every `parser.Code*` constant has an entry.
  - `TestPlaceholdersAreFillable`: every placeholder of every entry is either a role, or a Details key that the code's findings carry. Use a hand-kept map `code → []detailKey` in the test, checked against the fixtures from Task 4: run the validator on them and confirm that each listed key appears.
  - `TestNoEmptyFields`: every entry has a non-empty Group, Title, Meaning and HowToFix, and the Group is one of the five.
- [ ] **Step 2: Run `go test ./explain/`.** Expected: FAIL.
- [ ] **Step 3: Implement**, copying the Appendix tables row by row.
- [ ] **Step 4: Run `go test ./... && go vet ./...`.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(explain): plain-English catalog for every finding code`.

### Task 6: `repair`

**Files:**
- Create: `repair/repair.go`
- Create: `repair/repair_test.go`, `repair/safety_test.go`
- Fixtures: reuse `testdata/encodings/*`. Add `testdata/repair/`:
  - `blank-lines.ged`;
  - `leading-space.ged`;
  - `long-note.ged`: a 600-byte NOTE with multi-byte characters at the 255-byte boundary;
  - `no-trailer.ged`;
  - `cr-only.ged`;
  - `no-char.ged`;
  - `custom-tags.ged`: `_UID`, `0 @X1@ _EVDEF`, `2 _MILT`.

**Interfaces:**
- Consumes: `parser.Decode` (Task 2), `parser.Parse` with `Lenient` (Task 3), `enricher.Enrich`.
- Produces:
  ```go
  type Change struct { Line int `json:"line"`; What string `json:"what"` }
  type Result struct { Data []byte `json:"-"`; Changes []Change `json:"changes"`; Left []string `json:"left"` }
  func Repair(data []byte) (*Result, error)
  ```
- `Line` is the line number in the **original** file.
- `Left` is the codes found in the original that repair doesn't fix: the validator's findings (with `DateConsistency:true`) plus the non-repairable parse codes. It's deduplicated and sorted.
- Text tags that may be split with CONC: `NOTE TEXT PAGE CONT CONC TITL PUBL AUTH ABBR`.
- `What` strings, exactly:
  - `"converted from {encoding} to UTF-8"` (line 0, once);
  - `"set the character set to UTF-8"`;
  - `"changed line endings to CRLF"` (line 0, once);
  - `"removed a blank line"`;
  - `"removed spaces before the level number"`;
  - `"split a {n}-character line into {k} lines"`;
  - `"added the end-of-file marker (0 TRLR)"`.

- [ ] **Step 1: Write the failing tests** (`repair_test.go`), one per fixture, asserting the exact `Changes` and the bytes of a few key lines:
  - after a split, every line is ≤255 bytes;
  - no line ends or starts with a space at a split point;
  - each split falls on a UTF-8 rune boundary;
  - the `CHAR` line reads `1 CHAR UTF-8`, and comes right after `GEDC`'s subtree when it was added.
- [ ] **Step 2: Write the safety tests** (`safety_test.go`). They run over every `.ged` in `testdata/`, `testdata/encodings/` and `testdata/repair/`:
  - `TestRepairPreservesContent`: compare `json.Marshal` of `Enrich(Parse(original))` and `Enrich(Parse(repaired))`. Set `Document` to nil first and strip every `"line"` key. They must be equal, apart from the header's CHAR and the trailer.
  - `TestRepairIsIdempotent`: `Repair(Repair(x).Data).Changes` is empty.
  - `TestRepairKeepsUnchangedLines`: every original line that isn't touched by a Change appears byte-identical (after decoding) and in the same order.
  - `TestRepairKeepsCustomRecords`: `custom-tags.ged`, repaired, still contains `0 @X1@ _EVDEF` and its children, and `_UID` and `_MILT`.
- [ ] **Step 3: Run `go test ./repair/`.** Expected: FAIL.
- [ ] **Step 4: Implement.**
  - Decode with `parser.Decode`, then walk the lines, keeping the original line numbers.
  - Apply the fixes in spec 1.8 in one pass.
  - Then run `Parse` (lenient) and `ValidateWithOptions` on the original to fill `Left`.
- [ ] **Step 5: Run `go test ./... && go vet ./...`.** Expected: PASS.
- [ ] **Step 6: Commit** `feat(repair): line-level repair of format problems, with content-preservation tests`.

### Task 7: Release (momolig's call; not done by a builder)

- [ ] Run a whole-branch review (`superpowers:requesting-code-review`) and fix what it finds.
- [ ] README: a "Validating real-world files" section covering encodings, lenient mode, line numbers, `explain` and `repair`. Update `docs/INTERMEDIATE_FORMAT.md` for `line`, `encoding`, `lineCount`, `Line`, `Code`, `Severity`, `Details`.
- [ ] **Ask momolig.** Then merge `validate-ready` into `master`, push, and tag `v0.2.0`.
