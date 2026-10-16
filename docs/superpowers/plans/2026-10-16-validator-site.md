# validate.ligneous.org site (Part 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the public page validate.ligneous.org. It checks a GEDCOM file in the visitor's browser with the Go library compiled to WebAssembly, shows each finding in plain English with its line, people and surrounding lines, offers a text and CSV report, and offers a repaired file.

**Architecture:**
- **Repo.** A new repo, `ligneous-validate`, at `/srv/apps/ligneous-validate`.
- **`check/`** is a Go package that turns library output into one JSON shape the page needs. It's plain Go, so it's testable with `go test`.
- **`wasm/`** is a tiny `js && wasm` entry that exposes that package to JavaScript.
- **`web/`** is a Vite + TypeScript page with no framework. A Web Worker loads the WebAssembly and runs it, so the page never freezes.

**Tech Stack:**
- Go 1.23 (`go 1.23.5`), with `ligneous-gedcom-lib` at the `validate-ready` work (tag `v0.2.0` once released).
- Node 22, Vite 8.3.0, TypeScript 7.0.2 and `@playwright/test` 1.63.0. These match the versions in `~/data-viewer-filters`.
- Vitest: the newest 3.x that works with Vite 8.3.0. The builder checks this with `npm view` and records the version.

**Spec:** `docs/validator-site-spec.md` in the library repo: Part 2, Part 3, D1–D9. Part 1 is already built on `validate-ready`.

## Global Constraints

- **Privacy.** Nothing leaves the visitor's computer:
  - no `fetch`/XHR/beacon of file data;
  - no third-party fonts, scripts or analytics.
  - Every page includes exactly this CSP meta tag: `<meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self'; style-src 'self'; img-src 'self' data:; worker-src 'self'">`.
- **Look.** Use the ligneous.org landing tokens verbatim:
  - Light: `--bg: #f8f6f1; --text: #1f2a22; --muted: #55605a; --accent: #3f6b4f; --rule: #d9d3c6`.
  - Dark: `--bg: #141815; --text: #e9ebe6; --muted: #a7b0a9; --accent: #8fc0a0; --rule: #2e3530`.
  - Font: `--serif: "Iowan Old Style", "Palatino Linotype", Palatino, Georgia, serif`. Code and lines use `ui-monospace, Menlo, Consolas, monospace`.
  - Severity colours are new tokens, each checked to AA contrast on both backgrounds:
    - `--error: #9b2c2c` (light) and `#f19999` (dark);
    - `--warning: #8a5a00` (light) and `#e8c170` (dark);
    - `--hint: var(--muted)`.
    - Severity is never shown by colour alone: each finding also shows the word "Error", "Warning" or "Hint".
- **Layout and accessibility.** Phone first. It must work at 360 px width with a 16 px side gutter and no horizontal page scroll. The line viewer scrolls sideways inside its own box. It must meet WCAG 2.2 AA:
  - keyboard reachable;
  - visible focus;
  - the drop zone is also a real `<button>`;
  - live-region status messages.
- **Copy.** Exact strings, used verbatim:
  - Title: `Check your GEDCOM file`
  - Lead: `See every problem in your family tree file in plain English, with the line it's on.`
  - Privacy line: `Your file stays on your computer. Nothing is uploaded.`
  - Button: `Choose a file`
  - Drop hint: `or drop it here`
  - Status: `Reading your file…` / `Checking…`
  - Big-file note (over 50 MB): `This is a big file. Checking may take a minute.`
  - No problems: `No problems found in {people} people.`
  - Not GEDCOM: `This doesn't look like a GEDCOM file. GEDCOM files usually end in .ged and start with "0 HEAD".`
  - WebAssembly failed: `This browser couldn't start the checker. Try a current version of Firefox, Chrome, Safari or Edge.`
  - Repair heading: `We can fix {n} format problems automatically` (singular `1 format problem`).
  - Repair button: `Download repaired file`
  - Left heading: `These need fixing in your family tree program:`
  - Footer: `Made by Ligneous` (links to https://ligneous.org/) · `Source code` (links to https://github.com/lesfleursdelanuitdev/ligneous-validate).
- **Group order and headings:**
  - `format` → "File format and encoding"
  - `header` → "Header"
  - `links` → "Broken links"
  - `dates` → "Dates that don't add up"
  - `names` → "Names and sex"
- **Severity order:** error, then warning, then hint.
- **Placeholders:** filled per spec 1.7 and Task 1's role rules. If a placeholder can't be filled, the page shows the bare value from the finding, or for a role, "someone". It never shows `{…}`.
- **No new runtime dependencies** in the page. Dev dependencies only: vite, typescript, vitest, @playwright/test.
- **Installs.** **Ask momolig before the first `npm install`** in the new repo (house rule). The controller asks once, before Task 3.
- **Git.**
  - `git init` locally only. Creating the GitHub repo and pushing need momolig's OK.
  - Commit after each task. Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
  - The `replace` directive points at `/home/momolig/gedcom-lib-validate` until the library is tagged. Then it's swapped for `v0.2.0` (Task 6).
- **Playwright.** Use the cached Chromium only: `launchOptions.executablePath = ~/.cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-linux64/chrome-headless-shell`. Never `npx playwright install`.

## Review Focus

1. **Not a GEDCOM file** (a JPEG, PDF, zip or empty file) shows the Not-GEDCOM message. It must not show an empty report, and it must not crash. Rule: no `0 HEAD` found and zero records, or a fatal error. Test in Task 4: `e2e: not a gedcom file`.
2. **A second file after the first** replaces everything: summary, findings, repair, downloads. Nothing from the first file stays, including an open line viewer. Test in Task 4: `e2e: second file resets the report`.
3. **Thousands of findings.** A generated file with 5,000 findings renders its first screen in under 3 s. "Show more" adds 50 at a time without re-rendering the rest. Test in Task 5: `e2e: 5000 findings`.
4. **Large file.** A generated 20 MB file finishes checking, and the page stays responsive while it's checked (the status line updates and the button is focusable). Test in Task 5: `e2e: 20 MB file`.
5. **Download names.**
   - `Family Tree.GED` → `Family Tree-repaired.ged` and `Family Tree-report.txt`/`.csv`.
   - A name with no extension gets `.ged` added.
   - Path separators and control characters are stripped.

   Test in Task 3: `downloadName` unit tests.

---

### Task 1: Repo, and the `check` package (Go)

**Files:**
- Create `/srv/apps/ligneous-validate/`: run `git init`, create `go.mod` (module `github.com/lesfleursdelanuitdev/ligneous-validate`, `go 1.23.5`, require the lib `v0.0.0` with `replace => /home/momolig/gedcom-lib-validate`), `.gitignore` (`node_modules/`, `web/dist/`, `web/src/wasm/ligneous.wasm`, `web/public/wasm_exec.js`, `test-results/`, `playwright-report/`) and a `README.md` stub.
- Create: `check/check.go`, `check/roles.go`, `check/summary.go`, `check/repair.go`
- Test: `check/check_test.go`, `check/roles_test.go`, using the library's `testdata/` files (read via the replace path, or copied to `check/testdata/`).

**Interfaces:**
- Consumes (library):
  - `parser.ParseWithOptions` with `Lenient: true`;
  - `doc.Encoding`, `doc.LineCount`;
  - `validator.ValidateWithOptions` with `DateConsistency: true`;
  - `explain.Lookup`;
  - `enricher.Enrich`;
  - `repair.Repair`.
- Produces:
  ```go
  type Result struct {
      Encoding gedcom.EncodingInfo `json:"encoding"`
      Summary  Summary             `json:"summary"`
      Findings []Finding           `json:"findings"`
      Names    map[string]string   `json:"names"`   // xref → "Mary Smith (1850–1921)" / family → "John Smith & Ann Lee"
      Text     string              `json:"text"`    // the whole file decoded to UTF-8
      IsGedcom bool                `json:"isGedcom"`
  }
  type Summary struct {
      People, Families, Sources, Notes, Media int; Years *[2]int `json:"years"`
      Program, ProgramVersion, GedcomVersion string; Lines int
  } // json keys: people, families, sources, notes, media, years, program, programVersion, gedcomVersion, lines
  type Finding struct {
      Code, Severity, Group, Title, Meaning, HowToFix string
      Line int; Details map[string]string; Xrefs []string; Roles map[string]string
  } // json keys lowerCamel: code, severity ("error"|"warning"|"hint"), group, title, meaning, howToFix, line, details, xrefs, roles
  func Check(raw []byte) Result
  type RepairOut struct { Data []byte `json:"-"`; Changes []repair.Change `json:"changes"`; Left []string `json:"left"`; Before Counts `json:"before"`; After Counts `json:"after"` }
  type Counts struct { Errors, Warnings, Hints int } // json errors, warnings, hints
  func Repair(raw []byte) (RepairOut, error)
  ```
- **Role rules** (`roles.go`). Use `Xref`, `RelatedXref` and Details, and classify each xref as a person or a family by the document's index:
  - `person`: the first individual among Xref and RelatedXref.
  - `child`: `Details["child"]`, else the individual RelatedXref, on `CHILD_*`, `FATHER_*` and `MOTHER_*` codes.
  - `father` / `mother`: `Details["parent"]`, on codes containing `FATHER` / `MOTHER`.
  - `family`: `Details["family"]`, else the family among Xref and RelatedXref.
  - `target`: `Details["target"]`, else `Details["pointer"]`, else the Value of the pointer line.
- **Not GEDCOM:** `IsGedcom` is false when the parse returns a fatal error, or when no `0 HEAD` exists and the document has no records. Findings are still returned.
- **Program** is the `HEAD.SOUR.NAME` value, else the `HEAD.SOUR` value. **ProgramVersion** is `HEAD.SOUR.VERS`.
- **Years** come from every parsed date year in the enriched document, as `[min, max]`, or `nil` when there are none.

- [ ] **Step 1: Write the failing tests:**
  - `TestCheckRoyal92`: People 3010, findings non-empty, every finding has Line > 0, Title non-empty, and Severity one of the three.
  - `TestCheckEncodings`: `testdata/encodings/ansel.ged` gives Encoding.Detected `ANSEL`, and `Text` contains `René /Dupré/`.
  - `TestCheckNotGedcom`:
    - 4 KB of bytes from a PNG header and random data gives IsGedcom false;
    - an empty input gives IsGedcom false;
    - a valid minimal file gives true.
  - `TestRoles`: table-driven over fixtures that raise MOTHER_TOO_YOUNG_AT_CHILD_BIRTH, CHILD_BORN_BEFORE_FATHER, ORPHANED_FAMC, ORPHANED_HUSB, DEATH_BEFORE_BIRTH and INVALID_ASSOCIATE_TARGET. Each one's Roles is pinned exactly, e.g. `{child:@I2@, mother:@I3@, family:@F1@}`.
  - `TestNames`: an individual with birth 1850 and death 1921 gives `Mary Smith (1850–1921)` (the en dash is U+2013).
    - With only a birth: `Mary Smith (b. 1850)`.
    - With only a death: `Mary Smith (d. 1921)`.
    - With neither: `Mary Smith`.
    - With no name: `Unnamed person (@I9@)`.
    - A family: `John Smith & Ann Lee`, or one name alone, or `Family @F3@`.
    - The name is the enriched `full_name` with the slashes removed and the spaces collapsed.
  - `TestRepairCounts`: on `testdata/repair/cr-only.ged`, `Before` has more findings than `After`, `Changes` is non-empty, and running `Check(out.Data)` gives `After`'s counts.
- [ ] **Step 2: Run `go test ./...`.** Expected: FAIL (undefined).
- [ ] **Step 3: Implement** with the signatures above.
- [ ] **Step 4: Run `go test ./... && go vet ./...`.** Expected: PASS.
- [ ] **Step 5: Commit** `feat(check): one JSON result for the page (findings, roles, names, summary, repair)`.

### Task 2: WebAssembly entry and build

**Files:**
- Create: `wasm/main.go` (`//go:build js && wasm`).
- Create: `scripts/build-wasm.sh`. It runs `GOOS=js GOARCH=wasm go build -ldflags='-s -w' -o web/src/wasm/ligneous.wasm ./wasm`, then copies `$(go env GOROOT)/lib/wasm/wasm_exec.js` (falling back to `misc/wasm/`) to `web/public/wasm_exec.js`.
- Create: `scripts/smoke-wasm.mjs`. It runs under node: it loads wasm_exec.js and the .wasm, calls `check` on a file path given as an argument, and prints the people count, the number of findings and the time taken.

**Interfaces:**
- Produces `globalThis.ligneous = { check(bytes: Uint8Array): string, repair(bytes: Uint8Array): { data: Uint8Array, json: string } }`.
  - `check` returns `json.Marshal(check.Check(raw))`.
  - `repair`'s `json` is `RepairOut` and `data` holds the repaired bytes.
  - It sets `globalThis.ligneousReady = true` after registering.
  - Panics are recovered, and the function returns `{"error":"…"}` instead.

- [ ] **Step 1:** Write the smoke script first, then run it. Expected: it fails because there's no .wasm file.
- [ ] **Step 2:** Implement `wasm/main.go` and the build script.
- [ ] **Step 3:** Run `bash scripts/build-wasm.sh && node scripts/smoke-wasm.mjs /home/momolig/gedcom-lib-validate/testdata/royal92.ged`. Expected: `people 3010`, findings > 0, and a time under 3000 ms. Then run it on `testdata/encodings/utf16le-bom.ged` (expected: no error) and on a PNG (expected: `isGedcom false`).
- [ ] **Step 4:** Record the size (raw and `gzip -9`) in the commit body. **Budget: 1,500,000 bytes gzipped.** If it's over, stop and report.
- [ ] **Step 5: Commit** `feat(wasm): check and repair exposed to JavaScript`.

### Task 3: Page shell, worker and report rendering

**Before this task, the controller asks momolig to approve `npm install`.**

**Files:**
- Create: `package.json`, with these scripts:
  - `build`: `bash scripts/build-wasm.sh && vite build web`
  - `dev`
  - `test`: `vitest run`
  - `e2e`: `playwright test`
  - `check`: `go test ./... && npm test && npm run build && npm run e2e`
- Create: `web/vite.config.ts`, with `base: '/'`, `build.outDir: 'dist'`, assets hashed (the default), and the worker as an ES module.
- Create: `web/index.html` (the CSP meta, a static shell, a `<noscript>` line), `web/src/styles.css`, `web/src/main.ts`, `web/src/worker.ts`, `web/src/fill.ts`, `web/src/render.ts`, `web/src/names.ts` (`downloadName`), `web/src/types.ts` (mirrors Task 1's JSON).
- Test: `web/src/fill.test.ts`, `web/src/names.test.ts`, `web/src/render.test.ts`. `render.ts` is split into pure functions that return a view model (groups, ordered findings, visible slice, headings), which vitest tests, plus a thin DOM builder covered by the e2e tests. No DOM library in unit tests.

**Interfaces:**
- **Worker messages:**
  - in: `{type:'check', bytes:ArrayBuffer}` and `{type:'repair', bytes:ArrayBuffer}`;
  - out: `{type:'status', text}`, `{type:'result', result:Result}`, `{type:'repaired', data:ArrayBuffer, info:RepairOut}` and `{type:'error', message}`.
  - The worker imports the wasm with `new URL('./wasm/ligneous.wasm', import.meta.url)` so the file name is hashed, and loads `/wasm_exec.js` with `importScripts`. It starts loading as soon as it's created.
- **Fill functions:**
  - `fill(template: string, f: Finding, names: Record<string,string>): string` replaces `{role}` with `names[roles[role]] ?? 'someone'`, and `{key}` with `details[key] ?? ''`. Missing keys collapse the space around them.
  - `fillLine(n:number): string` formats as `Line 1,204` (en-US grouping).
- **Download names:** `downloadName(original: string, kind: 'repaired'|'report-txt'|'report-csv'): string`.

- [ ] **Step 1: Write the failing unit tests:**
  - `fill`: a role, a detail key, a missing role ("someone"), a missing key (no `{` left in the output), and two placeholders in one string.
  - `downloadName`: the Review Focus 5 cases.
  - `render`: grouping order, severity order inside a group, the first 50 then "Show more", and the "No problems" text with the people count.
- [ ] **Step 2: Run `npm test`.** Expected: FAIL.
- [ ] **Step 3: Implement.**
  - **Start screen:** title, lead, drop zone with the button, and the privacy line.
  - **While working:** a status line in an `aria-live="polite"` region.
  - **Report:**
    - a summary card (file name, people, families, sources, years, program and version, GEDCOM version, encoding, "(we can convert this)" when not UTF-8);
    - counts;
    - a severity filter (three checkboxes);
    - groups as `<section>`s with `<h2>`;
    - each finding as `<details>`/`<summary>`.
  - **Opening a finding** shows lines `line-3 … line+3` from `result.text`, in a `<pre>` with line numbers. The finding's line is marked with `<mark>`. Then "How to fix it".
- [ ] **Step 4: Run `npm test && npm run build`.** Expected: PASS, and `web/dist/` contains `index.html`, a hashed `.wasm` and `wasm_exec.js`.
- [ ] **Step 5: Commit** `feat(web): page, worker and report`.

### Task 4: Downloads and repair

**Files:**
- Create: `web/src/export.ts`.
- Modify: `web/src/render.ts`, `web/src/main.ts`.
- Test: `web/src/export.test.ts`.

**Interfaces:**
- `reportText(file: string, r: Result): string`. The text is:
  1. a header (file name, the date in ISO form, and the summary lines);
  2. the findings, grouped as on the page, each one as:
     ```
     [Error] Line 1,204: <title>
       <meaning filled>
       How to fix it: <howToFix filled>
     ```
- `reportCsv(r: Result): string` gives the columns `severity,code,line,title,people,meaning`.
  - `people` is the role names joined with `; `.
  - Escaping follows RFC 4180: quote every field, and double any quote inside it.
  - Line endings are CRLF, and the file starts with a UTF-8 BOM so Excel opens it correctly.
- **Repair section:**
  - Shown only when `changes.length > 0`.
  - The list shows "Line N: what" for each change, and just "what" when N is 0.
  - Then the counts before and after.
  - Then the button. It makes a `Blob` with type `text/plain;charset=utf-8` and saves it under `downloadName(…,'repaired')`.
  - Then the "These need fixing…" list: the `left` codes mapped to catalog titles, with their counts from the current result.

- [ ] **Step 1: Write the failing tests:**
  - `reportText`: the exact output for a 2-finding fixture.
  - `reportCsv`: quoting of `"`, commas and newlines, CRLF line endings and the BOM.
  - The repair-section render: singular and plural headings, and hidden when there are no changes.
- [ ] **Step 2: Run `npm test`.** Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Write the e2e tests** for Review Focus 1 and 2:
  - `e2e: not a gedcom file` (a PNG fixture);
  - `e2e: second file resets the report`.

  This task creates the `playwright.config.ts`:
  - it serves `vite preview` on port 5199;
  - `executablePath` is set per Global Constraints;
  - it runs one project at desktop size and one at 360×740 with `isMobile` and `hasTouch`.
- [ ] **Step 5: Run `npm test && npm run build && npm run e2e`.** Expected: PASS.
- [ ] **Step 6: Commit** `feat(web): text and CSV report, repaired file`.

### Task 5: End-to-end coverage and the privacy check

**Files:**
- Create: `e2e/validate.spec.ts` and `e2e/fixtures/`. The fixtures are copies of the library's royal92, ansel, utf16le-bom, cr-only and a broken file with one finding of each kind, plus generators for the 5,000-finding and 20 MB files, written at test time to the OS temp dir.

- [ ] **Step 1: Write these tests** (they run against the build; each should fail until the behaviour is right):
  - **clean file:** "No problems found in N people", or the count of hints only.
  - **royal92:**
    - the counts match `check` from the smoke script;
    - opening the first finding shows 7 lines with the marked line equal to its line number.
  - **ansel:** the encoding row says ANSEL and "(we can convert this)", and the names show accents correctly.
  - **utf16 / cr-only:** reports appear and the repair section shows.
  - **The repaired download:**
    - capture the download;
    - load it back through the page;
    - no `format`-group findings except LINE_TOO_LONG on non-note tags, and the error and warning totals are not higher.
  - **The report downloads:** the txt contains the first finding's title; the csv's first line is the header.
  - **Phone (360 px):** no horizontal scroll (`document.documentElement.scrollWidth <= innerWidth`), and the line viewer scrolls inside itself.
  - **Keyboard only:** Tab to "Choose a file", then open a finding with Enter, with focus visible.
  - **Privacy:** add a `page.on('request')` listener after the first load completes. Load and check royal92, run the repair, and download both reports. Every request URL must be same-origin and in the build's file list. Any other request fails the test.
  - **5,000 findings:** Review Focus 3.
  - **20 MB file:** Review Focus 4.
- [ ] **Step 2: Run `npm run e2e`.** Fix the page until all pass. Never weaken a test to pass. If a test can't pass, report it.
- [ ] **Step 3: Run `npm run check`.** Expected: PASS.
- [ ] **Step 4: Commit** `test(e2e): reports, downloads, phone, keyboard, privacy, big files`.

### Task 6: Deploy (momolig's steps are marked; nothing goes live without momolig's OK)

- [ ] Swap the `replace` for `github.com/lesfleursdelanuitdev/ligneous-gedcom-lib v0.2.0` once the library is tagged. Run `go mod tidy && npm run check`, then commit.
- [ ] Write `~/lesfleursdelanuit-setup/deploy-ligneous-validate.sh`. Use `deploy-ligneous-site.sh` as the template:
  - it runs `npm run build` in `/srv/apps/ligneous-validate`;
  - it rsyncs with `rsync -rt --omit-dir-times --no-links --delete web/dist/` into `/var/www/lesfleursdelanuit-sub/validate/`;
  - it probes:
    - `https://validate.ligneous.org/` returns 200 and the body contains the CSP meta;
    - the hashed `.wasm` returns 200 with `content-type: application/wasm`;
    - `/wasm_exec.js` returns 200.
- [ ] **momolig:**
  1. Panel → Hosts → **Add a static folder** `validate`, with **Show it at its own host** unticked.
  2. On the **validate.ligneous.org** card, delete the *route* `/` → `ligneous`. Delete only the route, never the folder.
  3. Add the route `/` → static folder `validate`, switched on, with **cache forever** on `/assets/`.
  4. Run `sudo setfacl -R -m u:momolig:rwX -m d:u:momolig:rwX /var/www/lesfleursdelanuit-sub/validate 2>&1 | tee ~/install-logs/ligneous-validate-acl.log`.
- [ ] Run the deploy script. Then load the live page in the cached Chromium, check royal92 there, and take desktop and phone screenshots.
- [ ] Add a `Check your GEDCOM file` link to `~/lesfleursdelanuit-setup/ligneous-site/index.html` under the tagline, then run `deploy-ligneous-site.sh`.
- [ ] **Ask momolig**, then create the public GitHub repo `lesfleursdelanuitdev/ligneous-validate` and push.
