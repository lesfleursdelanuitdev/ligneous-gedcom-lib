# validate.ligneous.org: spec

Status: PROPOSED 2026-10-16, for momolig's review. Branch `validate-ready` (worktree `~/gedcom-lib-validate`).

## Goal

A free public page at https://validate.ligneous.org/ where any genealogist can check a GEDCOM file, whatever program made it. They get:

- a summary of the file;
- every problem in plain English, with the line number and the people involved;
- the file's own lines around each problem;
- a report they can download;
- a repaired file for the format problems that have one safe fix.

**The file never leaves the visitor's computer.** The Go library runs in the browser as WebAssembly. Nothing is uploaded.

Success:

1. Files from common programs check without a crash or a silent misread. That covers files with CR line endings, ANSEL, ANSI, UTF-16 and mislabelled encodings.
2. Every finding has a line number and a plain-English explanation.
3. The repaired file has exactly the same genealogical content as the original.
4. A test proves no request leaves the page after it has loaded.

## Decisions (momolig, 2026-10-16)

| # | Decision |
|---|---|
| D1 | Files are checked in the browser (WebAssembly). The Go API stays loopback-only for the Gonsalves admin and is not changed here. |
| D2 | Audience: any genealogist. Plain-English findings, a quiet link to Ligneous. |
| D3 | v1 has a file summary, show-the-lines, a downloadable report (text and CSV), and a repaired file. |
| D4 | WebAssembly is built with standard Go, not TinyGo. Measured with a throwaway probe: 1.2 MB gzipped; parse, validate, enrich and export of a 1.1 MB file took about 1 s under node. |
| D5 | Explanations (`explain`) and repairs (`repair`) live in the Go library, so the admin, the API and Ligneous can reuse them later. Repairs work on lines and never re-export from the enriched tables, because that drops tags. |
| D6 | The page is a new public repo, `ligneous-validate`: plain TypeScript and Vite, no framework, deployed to a new site folder `validate`. |
| D7 | Not in v1: program-specific advice ("In Family Tree Maker…"), GEDCOM 7, repairs that change data, and moving the API or admin to the new library. |

**One addition made while writing this spec** (not discussed yet, and open for review):

| # | Decision |
|---|---|
| D8 | A lenient parse mode. Today one malformed line, such as a level that isn't a number, stops the whole parse. With `Options.Lenient`, the parser skips the line, records it as an error finding and goes on. The default stays strict, so the API keeps returning 422 as before. The page uses lenient mode. |
| D9 | The page turns on the validator's `DateConsistency` option, which is off by default. Without it, "death before birth" and "age at death over 120" are never reported. |

## Part 1: the library (`ligneous-gedcom-lib`, branch `validate-ready`)

### 1.1 Reading lines

- A custom `bufio.SplitFunc` ends a line at `\r\n`, `\n` or a bare `\r`. Today a CR-only file is read as one line: the whole file becomes HEAD's value, and Validate says `valid: true` with 0 people.
- The scanner's buffer goes up to 1 MB per line, from the default 64 KB. Lines over 255 characters are still reported (`LINE_TOO_LONG`).
- Spaces or tabs before the level number are allowed, and reported as `LEADING_WHITESPACE`.

### 1.2 Splitting a line

Tokenise by position. Skip leading whitespace, then read the level digits and one or more spaces. Then read an optional `@xref@` followed by spaces, then the tag. The value is everything after **exactly one** space following the tag.

- Fixes `0 @NOTE1@ NOTE Hello`. Today it gives the value `1@ NOTE Hello`, because the code uses `strings.Index(text, tag)` and that matches inside the xref.
- Fixes `1 CONC  world`. Today the value is `world`, because all leading spaces are trimmed, so the joined text reads "thereworld".
- The enricher trims values where it reads names, places and dates, so there's no visible change for ordinary lines. The existing enricher and exporter tests must still pass unchanged.

### 1.3 Encodings

The encoding is decided before any line is split. Results go in a new `parser.Encoding` result:

```go
type EncodingInfo struct {
    Detected  string // "UTF-8", "UTF-16LE", "UTF-16BE", "ANSEL", "Windows-1252", "CP437", "MacRoman", "ASCII"
    Declared  string // HEAD.CHAR as written, "" when missing
    Reason    string // "byte order mark", "header CHAR", "bytes are not valid UTF-8", ...
}
```

Order:

1. **BOM:** UTF-8 `EF BB BF`, UTF-16LE `FF FE`, UTF-16BE `FE FF`.
2. **UTF-16 without a BOM:** the first bytes look like `0\0 \0` (LE) or `\00\0 ` (BE).
3. **HEAD.CHAR**, read from the raw bytes of the first 50 lines, which are ASCII in every supported encoding:

   | CHAR | Encoding |
   |---|---|
   | `UTF-8` | UTF-8 |
   | `UNICODE` | UTF-16, with the byte order from step 2 |
   | `ANSEL` | ANSEL |
   | `ASCII` | ASCII (read as UTF-8, which is a superset) |
   | `ANSI` | Windows-1252 |
   | `IBMPC`, `IBM WINDOWS` | CP437 |
   | `MACINTOSH` | MacRoman |
   | missing | UTF-8, finding `ENCODING_UNDECLARED` |

4. **Check against the bytes:**
   - declared UTF-8 or ASCII (or missing, or a UTF-8 BOM), but not valid UTF-8 → the mixed rule below decides between UTF-8 with stray bytes and Windows-1252 throughout (finding `ENCODING_MISLABELED`);
   - declared ANSEL, ANSI, IBMPC or MACINTOSH, but valid UTF-8 containing at least one multi-byte sequence → UTF-8, finding `ENCODING_MISLABELED`;
   - declared ANSI (or an alias such as `ISO-8859-1` or `LATIN1`), not valid UTF-8, but passing the mixed rule (strict majority) → UTF-8 with stray bytes, finding `ENCODING_MISLABELED` (declared ANSI, detected UTF-8).
   - Whenever a file whose header names a legacy encoding (ANSI and its aliases, ANSEL, IBMPC, MACINTOSH) is read as UTF-8 or mixed, without a BOM, the repair records `read {n} characters as UTF-8 (the header said {declared})` on every line that holds at least one valid multi-byte character, and `ENCODING_MISLABELED` stays in `Left`. The STRAY Changes for converted bytes are recorded as usual.
5. **Not UTF-8 after all that** → hint `ENCODING_NOT_UTF8`, which the repair fixes.

**The mixed rule.** Count the valid multi-byte UTF-8 sequences (*multi*) and the bytes that are not valid UTF-8 (*invalid*). What wins depends on the declaration:
- **Declared UTF-8, or a UTF-8 BOM:** multi > 0 and multi ≥ invalid → UTF-8 with a few stray bytes. A tie counts as UTF-8 here.
- **Declared legacy (ANSI and its aliases, ANSEL, IBMPC, MACINTOSH), undeclared, or any other CHAR:** UTF-8 (or mixed) needs a strict majority, multi > invalid. A tie keeps the declared or legacy decoding, so `CHAR ANSI` with `Ren\xE9e` and `CAF\xC9\x94` reads `Renée` and `CAFÉ”`, not `CAFɔ`. Only ANSI files go mixed; ANSEL, IBMPC and MACINTOSH files are read as UTF-8 only when they are entirely valid UTF-8. An undeclared file that wins is treated like declared UTF-8, with no per-line record, because `ENCODING_UNDECLARED` is already reported.
- Otherwise → Windows-1252 throughout (or the declared encoding). After a UTF-8 BOM, each run of invalid bytes becomes U+FFFD instead.

In the mixed path each stray byte is read as Windows-1252, and the file gets one `ENCODING_STRAY_BYTES` finding (warning; `count`, `encoding`; on the first affected line). The five bytes Windows-1252 leaves undefined (0x81, 0x8D, 0x8F, 0x90, 0x9D) are unreadable, not stray: they become U+FFFD and count towards `ENCODING_BAD_BYTES` instead.

**UTF-8 split across CONC.** Some exporters cut CONC lines by bytes, in the middle of a character: `1 NOTE Caf\xC3` then `2 CONC \xA9 au lait`. Before counting, invalid bytes at the end of a line that become a valid character when joined with the start of the next line's value count as one valid multi-byte sequence, not as stray bytes, provided the next line is a real continuation: `CONC` one level below the line, or `CONC` at the same level when the line is itself `CONC` or `CONT` (the same rule as the ANSEL carry-over). The character is joined by moving its continuation bytes **back** onto the first line, which then ends with the whole character: `1 NOTE Café` and `2 CONC  au lait`. That way a space before the character is not left at the end of a line, where readers trim it (`1 NOTE Prix 10 €`, `2 CONC  ok` reads `Prix 10 € ok`). If moving the bytes back would push the first line past 255 bytes, the lead bytes move forward to the start of the CONC value instead, when the byte before them is not whitespace. Otherwise the character is not joined, and its bytes are read as stray or unreadable under the normal rules and recorded as such. The repair records `joined a character split across lines {n} and {m}` on line n.

**Two codes for bytes:**
- `ENCODING_STRAY_BYTES`: bytes in a UTF-8 file that were in another encoding and were read as Windows-1252. The repaired file keeps the letters as read, with one `converted {n} stray Windows-1252 characters to UTF-8` Change per line.
- `ENCODING_BAD_BYTES`: bytes that could not be read at all and became U+FFFD (undefined in the file's encoding, undefined Windows-1252 bytes, ANSEL marks with no letter, lone UTF-16 surrogates). Each line gets a `replaced {n} unreadable characters with �` Change, and the code stays in `Left`.

**ANSEL decoding:**
- Use a hand-written table (ANSI/NISO Z39.47 plus the GEDCOM extensions).
- Combining marks (bytes 0xE0–0xFE) come *before* their base letter in ANSEL. The decoder moves them after the letter and then applies Unicode NFC. That needs `golang.org/x/text/unicode/norm`.
- Bytes that aren't in the table become U+FFFD, with finding `ENCODING_BAD_BYTES`. The finding gives the line and a count.
- A mark at the end of a line carries over to the first letter of the next line only when that line is a real CONC continuation (see above). Otherwise it becomes U+FFFD (`ENCODING_BAD_BYTES`).

**Code pages:** use `golang.org/x/text/encoding/charmap` for Windows-1252, CP437 and MacRoman. This is the library's first new dependency. **Check:** it must add under 100 KB gzipped to the WebAssembly build. If it adds more, hand-write the three 128-entry tables instead.

### 1.4 Structure the parser drops silently today

| Today | Now |
|---|---|
| Lines before `0 HEAD` are dropped | still dropped, finding `LINES_BEFORE_HEAD` (first line, count) |
| `1 BIRT` followed by `3 DATE` is attached anyway | still attached, finding `LEVEL_JUMP` |
| Unknown level-0 records (`0 @X1@ _EVDEF`) are dropped | still dropped from the tree, finding `UNKNOWN_RECORD` (hint). The repair keeps these lines. |
| An empty line is a warning | finding `EMPTY_LINE` |

### 1.5 Parse codes and lenient mode

`ParseError` gains `Code string` and `Severity`. Today's fatal errors get codes:

- `BAD_LINE`: no level or no tag;
- `BAD_LEVEL`: the level is not a number, or is negative;
- `XREF_WITHOUT_TAG`;
- `TOO_DEEP`: past `MaxNestingDepth`.

With `Options.Lenient = true` (D8), these become error findings and the parser goes on. The JSON of `ParseError` today has no tags (`Line`, `Message`, `Context`, `Err`), so the new fields are added the same way: `Code`, `Severity`.

### 1.6 Line numbers

- `GedcomRecord` gains `Line int \`json:"line,omitempty"\`` on every node.
- `ValidationError` gains `Line int \`json:"Line,omitempty"\``.
- **Exact node where the check has it:**
  - date checks → the `DATE` line;
  - orphaned pointers → the pointer line (`FAMC`, `HUSB`, …);
  - name checks → the `NAME` line;
  - `INVALID_SEX` → the `SEX` line;
  - ASSO checks → the `ASSO` line.
- **Everything else** gets its record's level-0 line from a final pass over an xref → line index. `MISSING_HEADER` gets line 1. `MISSING_TRAILER` gets the last line.
- **Test:** every finding on the sample files has `Line > 0`, and the line it points at contains the expected tag.

### 1.7 `explain` package

```go
type Entry struct {
    Code     string
    Group    string // "format", "header", "links", "dates", "names"
    Title    string
    Meaning  string // may contain {placeholders} filled from Details or by the page
    HowToFix string
}
func Lookup(code string) (Entry, bool)
func All() []Entry
```

- Placeholders: `{person}`, `{child}`, `{father}`, `{mother}`, `{target}` (filled with names by the page from the xrefs), plus any Details key (`{age}`, `{marriage_year}`, `{date}`, …).
- **Missing details:** findings gain the Details the catalog needs that aren't there today:
  - `death_year` and `birth_year` (DEATH_BEFORE_BIRTH);
  - `target_tag` (INVALID_ASSOCIATE_TARGET);
  - `sex` (INVALID_SEX);
  - `encoding`, `declared` and `count` (encoding findings);
  - `count` (LINES_BEFORE_HEAD);
  - `tag` (UNKNOWN_RECORD);
  - `family` wherever a family is involved.
- **Coverage test:** every `validator.Code*` constant, `LINE_EXCEEDS_MAX_PHYSICAL_LENGTH` and every parse code has an entry. A test fails on a placeholder that no finding can fill.
- The catalog wording is in the appendix, for review.

### 1.8 `repair` package

```go
type Change struct { Line int; What string; Fix Fix }  // What: plain English, e.g. "split a 312-character note into 2 lines"
type Result struct {
    Data      []byte    // UTF-8 without a BOM when every fix is applied; see "Choosing fixes"
    Changes   []Change
    Left      []string  // codes still present after the repair (including those of fixes not chosen)
    Available []FixInfo // every kind the file could use; Lines counted with every fix applied
    Applied   []FixInfo // the kinds that changed something, with the lines they really changed
    Skipped   []FixInfo // chosen and available, but nothing changed, with the reason
}
type FixInfo struct { Fix Fix; Lines int; Required bool; Reason string }
type Options struct { Fixes []Fix `json:"fixes"` } // nil or omitted: every format fix; []: none
func Repair(data []byte) (*Result, error)       // = RepairWithOptions(data, Options{})
func RepairWithOptions(data []byte, opt Options) (*Result, error)
```

**Fixes:**
- re-encode to UTF-8 and set `1 CHAR UTF-8`, adding it after `1 GEDC` if missing;
- turn bare `\r` line endings into `\r\n`. Otherwise keep the file's own line-ending style;
- remove blank lines;
- remove whitespace before the level number;
- split lines over 255 bytes with `CONC`, on text tags only: `NOTE`, `TEXT`, `PAGE`, `CONT`, `CONC`, `TITL`, `PUBL`, `AUTH`, `ABBR`. Never split inside a UTF-8 sequence, and never at a space;
- add `0 TRLR` if it's missing.

**Choosing fixes (v0.3.0).** `RepairWithOptions(data, Options{Fixes: []Fix{...}})` applies only the chosen kinds: `encoding` (re-encode to UTF-8 and set `CHAR UTF-8`, with the stray, unreadable, read-as-UTF-8 and joined-character changes), `bom`, `line-endings`, `blank-lines`, `leading-space`, `long-lines` and `trailer`. A nil `Fixes` means all of them, and `Repair(data)` is exactly that. Every `Change` carries its `Fix`. `Result.Available` lists every kind the file could use, in that order, with its line count, the same whatever is chosen. The page shows it as a checklist with "Repair everything" and "Repair selected". There is no choice per line. Without `encoding` the file keeps its own encoding, and the other fixes edit its original bytes. A UTF-16 file is then left unchanged (`encoding` is `Required`: "this file is UTF-16, so it has to be converted first"; the other kinds say "needs the conversion to UTF-8"). `long-lines`, and a `bom` that overrules a legacy CHAR, are skipped with the reason "needs the conversion to UTF-8". Edits that could change how the file is read are skipped too: those next to a line ending in a non-ASCII byte (the decoder may join it with the next CONC line), and line removals or splits in HEAD before the CHAR line the decoder finds. The repaired file is then read again, and it must read like the original. If it doesn't, it is made again without `blank-lines` and `leading-space`, and as a last resort it is left unchanged. A chosen kind that changed nothing is listed in `Skipped` with its reason ("needs the conversion to UTF-8" or "skipped: it would change how the file is read"), so a choice is never ignored silently. A UTF-8 byte order mark stays unless `bom` is chosen, even when `encoding` is on: the result is then UTF-8 with a BOM and `CHAR UTF-8` (accepted). When only some bare CRs could be changed, the change reads "changed N line endings to CRLF". `Left` still lists the codes of the kinds that were not chosen. The safety tests below hold for each single kind and for all-but-each-kind.

**Never changes:** links, dates, names, the order of lines, xrefs, level jumps, lines before HEAD, or unknown records. Each of these stays in `Left`.

**Safety tests** run on every sample file and on a broken fixture for each repair:
1. `Enrich(Parse(original))` and `Enrich(Parse(repaired))` give identical people, families, events, notes, sources and media. The only difference allowed is the header's CHAR and an added trailer.
2. `Repair(Repair(x).Data)` returns no changes.
3. Every line that wasn't changed is byte-identical after decoding.

### 1.9 Release

After review and momolig's OK: merge `validate-ready` into `master`, push, and tag `v0.2.0`. The Go API and the Gonsalves admin keep their vendored copy until a separate step (D7).

## Part 2: `ligneous-validate` (new repo, `/srv/apps/ligneous-validate`)

- **The GitHub repo:** public, `lesfleursdelanuitdev/ligneous-validate`. Create it only after momolig's OK.
- **The library:** depend on `ligneous-gedcom-lib v0.2.0` by tag. During development, use a `replace` to the worktree, which is removed before the first push.

### 2.1 WebAssembly entry point (`wasm/main.go`)

Build: `GOOS=js GOARCH=wasm go build -ldflags='-s -w'`. Two functions are set on `globalThis.ligneous`.

**`check(bytes: Uint8Array) → string (JSON)`:**

```jsonc
{
  "encoding": { "detected": "ANSEL", "declared": "ANSEL", "reason": "header CHAR" },
  "summary": {
    "people": 2322, "families": 1100, "sources": 40, "notes": 12, "media": 0,
    "years": [1732, 2020],                 // earliest and latest event years, null when none
    "program": "Family Tree Maker", "programVersion": "24.0",   // HEAD.SOUR.NAME (or SOUR value) and VERS
    "gedcomVersion": "5.5.1", "lines": 98431
  },
  "findings": [{
    "code": "MOTHER_TOO_YOUNG_AT_CHILD_BIRTH", "severity": "warning", "line": 1204,
    "group": "dates", "title": "...", "meaning": "...", "howToFix": "...",   // from explain, placeholders not filled
    "details": { "age": "11" }, "xrefs": ["@I12@", "@I40@"], "roles": { "child": "@I12@", "mother": "@I40@" }
  }],
  "names": { "@I12@": "Mary Smith (1850–1921)", "@F3@": "John Smith & Ann Lee" },
  "text": "the whole file decoded to UTF-8"      // for show-the-lines
}
```

- The lenient parse (D8) and `DateConsistency` (D9) are on.
- `roles` maps placeholder names to xrefs, so the page can fill `{mother}` and similar.

**`repair(bytes: Uint8Array) → { data: Uint8Array, changes: [...], before: counts, after: counts }`:**
- `before` and `after` are finding counts by severity, from running `check` on each version.

**Tests:** Go tests on the JSON shape for the sample files, using a non-wasm build of the same functions.

### 2.2 The page

Plain TypeScript and Vite, no framework. It matches the ligneous.org landing page (same type and colours, light and dark), works from phone width up, and meets WCAG 2.2 AA.

1. **Start.**
   - Headline: "Check your GEDCOM file".
   - One line on what it does.
   - A drop zone with a **Choose a file** button.
   - Under it: "Your file stays on your computer. Nothing is uploaded."
   - The WebAssembly file starts downloading as soon as the page opens.
2. **Working.** A Web Worker loads the WebAssembly and runs `check`, so the page never freezes. The page shows "Reading your file…", then "Checking…". Files over 50 MB get a note that it may take a while. There's no hard limit.
3. **Report.**
   - **The summary card:** file name, people, families, sources, years, the program that made it, GEDCOM version, encoding. For any encoding but UTF-8, it shows "(we can convert this)".
   - **Counts:** errors, warnings and hints.
   - **A severity filter.**
   - **Groups, in this order:**
     - File format and encoding
     - Header
     - Broken links
     - Dates that don't add up
     - Names and sex

     Within each group, errors come first. The first 50 are shown, then a **Show more** button.
   - **Each finding** shows its title, the meaning with names filled in, and "Line 1,204". When opened, it shows lines 1,201–1,207 in a monospace box with line 1,204 highlighted, and then **How to fix it**.
   - **No problems:** "No problems found in 2,322 people", with the summary.
4. **Download the report:**
   - **Text:** a summary, then each finding with its line, people, meaning and how to fix it.
   - **CSV:** severity, code, line, title, people, meaning.
5. **Fix what can be fixed.**
   - Shown only when `repair` has changes: "We can fix N format problems automatically."
   - It lists the changes, then the counts before and after, then **Download repaired file** (`<name>-repaired.ged`).
   - Then: "These need fixing in your family tree program:", with the count of findings it leaves.

- **Privacy:** a CSP meta tag: `default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'; style-src 'self'; img-src 'self' data:`. No third-party fonts, scripts or analytics.
- **Footer:** "Made by Ligneous", linking to https://ligneous.org/, plus a link to the source on GitHub.

### 2.3 Tests

- Vitest: filling placeholders, the report text, CSV escaping, grouping and sorting.
- Playwright, on chromium only, using the cached `chromium_headless_shell` on the server:
  - a clean file, `royal92.ged`, an ANSEL file, a UTF-16 file, a CR-only file, and a broken file with one finding of each kind;
  - the report and its counts, show-the-lines, both downloads, and the repaired file. The repaired download is re-checked in the page, and must have fewer findings and no format findings;
  - phone width (390 px) and desktop;
  - **no request after load**: a request listener fails the test on any request that isn't one of the page's own static files.

## Part 3: deploy

1. **momolig, in the panel (once):**
   1. **Add a static folder** `validate`, with **Show it at its own host** unticked.
   2. On the **validate.ligneous.org** card, delete the route `/` → `ligneous` (the route only, never the folder). Then add the route `/` → static folder `validate`, switched on, with **cache forever** on `/assets/`.
   3. Run once: `sudo setfacl -R -m u:momolig:rwX -m d:u:momolig:rwX /var/www/lesfleursdelanuit-sub/validate`. (This step is no longer needed once pammy-setup#2's ACL fix is built.)
2. **`~/lesfleursdelanuit-setup/deploy-ligneous-validate.sh`:**
   - builds;
   - runs `rsync -rt --omit-dir-times --no-links --delete dist/` into the folder;
   - probes https://validate.ligneous.org/. It expects 200, the CSP meta tag present, and the `.wasm` asset served with `Content-Type: application/wasm` and status 200.
3. **ligneous.org landing:** add a "Check your GEDCOM file" link to https://validate.ligneous.org/.
4. **Certificate:** validate.ligneous.org is already on ligneous.org's certificate. Switching renewal to webroot (the open item from the ligneous.org session) is separate from this work.

## Steps

Each step is test-first and reviewed. Nothing is merged, pushed, tagged, created on GitHub or deployed without momolig's OK.

1. Library:
   1. lines, values, line endings (1.1, 1.2);
   2. encodings (1.3), including the size check;
   3. structure findings, parse codes, lenient mode (1.4, 1.5);
   4. line numbers (1.6);
   5. `explain` (1.7);
   6. `repair` (1.8);
   7. release (1.9).
2. `ligneous-validate`:
   1. the entry point (2.1);
   2. the page (2.2);
   3. the tests (2.3).
3. Deploy (Part 3).

## Appendix: explanation catalog (draft wording)

`{person}` and the other role names become "Mary Smith (1850–1921)" on the page. Severity is the validator's own and is not changed here.

### File format and encoding (group `format`)

| Code | Title | Meaning | How to fix it |
|---|---|---|---|
| ENCODING_NOT_UTF8 | This file uses an older character encoding ({encoding}) | Most programs today expect UTF-8. Some will show accented letters wrongly when they import this file. | Download the repaired file below, which is converted to UTF-8. Or, when you export from your program, pick "UTF-8" as the character set. |
| ENCODING_MISLABELED | The file says it's {declared}, but it's really {encoding} | The header names one character set, but the letters in the file are written in another. A program that trusts the header will show accented letters as gibberish. | Download the repaired file below. Its header matches its contents. |
| ENCODING_UNDECLARED | The file doesn't say which character set it uses | We read it as UTF-8. Another program might guess differently. | Download the repaired file below, which says UTF-8 in its header. |
| ENCODING_BAD_BYTES | {count} characters couldn't be read | These bytes don't match any letter in the file's character set. They show as � here. The letters were probably damaged before this file was made. | Find these people in your program and retype the names or places shown with �. |
| ENCODING_STRAY_BYTES | Some characters were in a different encoding | {count} characters were in a different encoding from the rest of the file. We've read them as best we can — check the names and places on these lines. | Look at the names and places on these lines in your program and correct any letter that looks wrong. The repaired file keeps the letters as we read them. |
| LINE_TOO_LONG | A line is longer than GEDCOM allows | GEDCOM lines may be at most 255 characters. Some programs cut longer lines short and lose the end of the text. | The repaired file splits long notes into continuation lines. For other kinds of line, shorten the text in your program. |
| LINE_EXCEEDS_MAX_PHYSICAL_LENGTH | A line is longer than GEDCOM allows | (same as LINE_TOO_LONG) | (same) |
| EMPTY_LINE | Blank line | GEDCOM files shouldn't have blank lines. Most programs skip them, but a few stop reading. | The repaired file removes them. |
| LEADING_WHITESPACE | A line starts with spaces | Every line should start with its level number. Some programs reject lines that start with a space. | The repaired file removes the spaces. |
| BAD_LINE | A line we couldn't read | Every line needs a level number and a tag, like `1 NAME`. This one doesn't, so we skipped it. Anything it held is missing from the check. | Open the file in a text editor at this line and see what it should be. The most common cause is a note whose text runs onto a new line without a `CONT` tag. |
| BAD_LEVEL | A line's level number isn't a number | Every line starts with a number that shows how it nests under the line above. We skipped this line. | Usually a note whose text broke onto a new line. Fix it in a text editor, or re-export from your program. |
| XREF_WITHOUT_TAG | A record has an id but no type | This line has an id like `@I1@` but doesn't say whether it is a person, a family or something else. We skipped it. | Re-export from your program. If it happens again, tell the program's makers. |
| TOO_DEEP | Lines nest too deeply | This line is nested more than 100 levels deep, which no real file needs. The file is probably damaged. | Re-export from your program. |
| LEVEL_JUMP | A line skips a level | This line is nested two or more levels below the line above it. Programs may attach it to the wrong thing or drop it. | Find this person or family in your program and check the event or fact near this line. If it looks right there, re-export. |
| LINES_BEFORE_HEAD | There's text before the file's header | A GEDCOM file must begin with `0 HEAD`. We ignored {count} lines before it. | Open the file in a text editor and delete everything above `0 HEAD`, or re-export from your program. |
| UNKNOWN_RECORD | A kind of record we don't read | Your program saved some of its own records here (`{tag}`). Other programs usually ignore them. Nothing is wrong with your tree. | Nothing to do, unless you need this information in another program. |
| MISSING_HEADER | The file has no header | A GEDCOM file must start with `0 HEAD`. Without it, most programs won't open the file. | Re-export from your program. |
| MISSING_TRAILER | The file has no end marker | A GEDCOM file should end with `0 TRLR`. Without it, the file may have been cut short while it was copied or downloaded. | Check that the file's last people are all there. The repaired file adds the end marker. |

### Header (group `header`)

| Code | Title | Meaning | How to fix it |
|---|---|---|---|
| HEADER_MISSING_SUBM | The file doesn't say who made it | GEDCOM {gedcom_version} expects the header to name a submitter, meaning the person who made the file. Most programs don't mind. | In your program, fill in your name as the researcher or submitter, then export again. |
| XREF_EXCEEDS_VERSION_LIMIT | An id is longer than this GEDCOM version allows | Ids like `@I123@` may be at most 20 characters in GEDCOM before 7.0. Some programs reject longer ones. | Usually nothing you can change in your program. If an import fails, tell the program's makers. |

### Broken links (group `links`)

| Code | Title | Meaning | How to fix it |
|---|---|---|---|
| MISSING_XREF_INDIVIDUAL | A person has no id | Every person needs an id such as `@I1@` so families can point to them. Nothing can link to this person. | Re-export from your program. |
| MISSING_XREF_FAMILY | A family has no id | Every family needs an id such as `@F1@` so people can point to it. | Re-export from your program. |
| ORPHANED_FAMC | {person} is a child of a family that doesn't exist | The file links {person} to family {target} as a child, but there's no such family in the file. The link is lost on import. | Open {person} in your program and check their parents. Remove the broken link and add the parents again. |
| ORPHANED_FAMS | {person} is a spouse in a family that doesn't exist | The file links {person} to family {target} as a partner, but there's no such family. | Open {person} in your program and check their spouses or partners. |
| ORPHANED_HUSB | A family's husband doesn't exist | Family {family} names {target} as husband, but there's no such person in the file. | Open the family in your program and choose the husband again. |
| ORPHANED_WIFE | A family's wife doesn't exist | Family {family} names {target} as wife, but there's no such person in the file. | Open the family in your program and choose the wife again. |
| ORPHANED_CHIL | A family's child doesn't exist | Family {family} lists {target} as a child, but there's no such person in the file. | Open the family in your program and check its children. |
| ORPHANED_SOUR | A source that doesn't exist | {person} cites source {target}, but there's no such source in the file. The citation is lost on import. | Open the citation in your program and choose the source again. |
| ORPHANED_NOTE | A note that doesn't exist | {person} points to note {target}, but there's no such note in the file. | Open {person} in your program and check their notes. |
| ORPHANED_XREF_OTHER | A link to something that doesn't exist | A `{pointer_tag}` line points to {target}, but there's no such record in the file. | Find the record at this line in your program and remove or redo the link. |
| MISSING_ASSOCIATE_XREF | An association with no person | {person} has an association (godparent, witness and so on) that doesn't say who with. | Open {person} in your program and fix or remove the association. |
| ORPHANED_ASSOCIATE_XREF | An association with someone who doesn't exist | {person} is associated with {target}, but there's no such person in the file. | Open {person} in your program and fix or remove the association. |
| INVALID_ASSOCIATE_TARGET | An association that doesn't point to a person | {person}'s association points to {target}, which is a {target_tag}, not a person. | Open {person} in your program and choose the associated person again. |
| MISSING_ASSOCIATE_RELA | An association that doesn't say what the relationship is | {person} is associated with {target} but the file doesn't say how, for example godparent or witness. | Open the association in your program and fill in the relationship. |
| EMPTY_FAMILY | A family with nobody in it | This family has no husband, wife or children. It's usually left over from deleting people. | Find the empty family in your program and delete it. |

### Dates that don't add up (group `dates`)

| Code | Title | Meaning | How to fix it |
|---|---|---|---|
| DEATH_BEFORE_BIRTH | {person} died before they were born | Death year {death_year} is before birth year {birth_year}. One of the dates is probably mistyped. | Open {person} in your program and check both dates against your sources. |
| AGE_AT_DEATH_EXCEEDS_120 | {person} lived to {age} | Possible, but rare. Often a typo in a year, or two people with the same name combined into one. | Check {person}'s birth and death dates against your sources. |
| BAPTISM_BEFORE_BIRTH | {person} was baptised before they were born | Baptism ({bapm}) is before birth ({birt}). | Check both dates. Sometimes the baptism date was entered as the birth date. |
| BURIAL_BEFORE_DEATH | {person} was buried before they died | Burial ({buri}) is before death ({deat}). | Check both dates against your sources. |
| CHILD_BORN_BEFORE_FATHER | {child} was born before their father | {child} was born before {father}. Usually a mistyped year, or the child linked to the wrong family. | Check both birth dates, and that {child} is in the right family. |
| CHILD_BORN_BEFORE_MOTHER | {child} was born before their mother | {child} was born before {mother}. Usually a mistyped year, or the child linked to the wrong family. | Check both birth dates, and that {child} is in the right family. |
| FATHER_TOO_YOUNG_AT_CHILD_BIRTH | {father} was {age} when {child} was born | That's younger than usual for a father. It may be right, but it's often a typo or the wrong family. | Check both birth dates and {child}'s parents. |
| MOTHER_TOO_YOUNG_AT_CHILD_BIRTH | {mother} was {age} when {child} was born | That's younger than usual for a mother. It may be right, but it's often a typo or the wrong family. | Check both birth dates and {child}'s parents. |
| FATHER_TOO_OLD_AT_CHILD_BIRTH | {father} was {age} when {child} was born | That's older than usual for a father. It's often two men with the same name combined into one. | Check both birth dates and {child}'s parents. |
| MOTHER_TOO_OLD_AT_CHILD_BIRTH | {mother} was {age} when {child} was born | That's well past the age women usually have children. It's often a typo or the wrong family. | Check both birth dates and {child}'s parents. |
| MARRIAGE_BEFORE_SPOUSE_BIRTH | {person} married before they were born | The marriage in {marriage_year} is before {person}'s birth. | Check the marriage date and {person}'s birth date. |
| MARRIED_BELOW_MINIMUM_AGE | {person} married at {age} | That's younger than usual. It may be right for the time and place, or a mistyped year. | Check the marriage date and {person}'s birth date. |
| EVENT_DATE_UNPARSEABLE | A date we couldn't understand | {person}'s {event} date reads "{date}". Programs may ignore it or read it differently. | Rewrite it as a GEDCOM date, such as `12 MAR 1850`, `ABT 1850`, `BEF 1850` or `BET 1850 AND 1855`. Free text belongs in a note. |
| SWAPPED_OPPOSITE_SEX_SPOUSE_TAGS | The husband and wife may be swapped | In this family, the person entered as husband is female and the person entered as wife is male. | Open the family in your program and check who is in which place, and each person's sex. |

### Names and sex (group `names`)

| Code | Title | Meaning | How to fix it |
|---|---|---|---|
| MISSING_NAME | A person with no name | Person {person} has no name in the file. They'll show as "unknown" in most programs. | Open the person in your program and add a name, even a partial one. |
| NAME_TAG_EMPTY | A name that's blank | {person} has a name line with nothing in it. | Open {person} in your program and fill in or remove the empty name. |
| NAME_MISSING_SURNAME_SLASH | We can't tell which part of the name is the surname | GEDCOM marks the surname with slashes, as in `John /Smith/`. This name has none, so other programs may sort it wrongly. | In your program, put the surname in the surname field. If the person has no known surname, this is fine. |
| INVALID_SEX | A sex value other programs won't understand | {person}'s sex is "{sex}". GEDCOM allows M, F, U (unknown), X (intersex) and N (not recorded). | Open {person} in your program and choose their sex again. |

[TODO: momolig] Check the tone. Check the BAD_LINE "most common cause". Say if "Ligneous" should appear anywhere in the findings. I've kept it out of them on purpose.
