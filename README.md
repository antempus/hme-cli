# hme-cli

This tool automates creation and management of iCloud "Hide My Email" addresses. It’s designed for power users who want to generate, list, deactivate aliases the UNIX way.

> Note: You need an active iCloud+ subscription to create iCloud emails.

---

## Highlights

- Designed to integrate with standard UNIX utilities (`fzf`, `awk`, `sort`) for fast interactive workflows.
- Stores session cookie and cache in predictable locations.
- Deactivate/Reactivate feature.
- Written in Go, single static binary, no issues with dependencies.

**Related project:** if you want to compare implementations, see [hidemyemail-generator](https://github.com/rtunazzz/hidemyemail-generator).

---

## Getting your iCloud session cookie string

1. Install the browser extension: **Cookie-Editor** (https://cookie-editor.com/) or another cookie export tool.
2. Login to [https://www.icloud.com](https://www.icloud.com) and open the extension.
3. Use **Export > Header String** (this produces the value used in the `Cookie:` request header).
4. Paste that exact single-line header string into `~/.config/hme-cli/cookies.txt`.

**Important:** keep that file readable only by you. If the session expires, repeat the export.

## Examples

Since the CLI outputs data in a tab-separated format (`\t`), it integrates perfectly with standard UNIX utilities like `fzf`, `awk`, and `sort`.

### Interactive bulk deactivation

```bash
hme-cli l \
  | sort -t $'\t' -k 5,5 -r \
  | fzf -m \
  | awk '{print $2}' \
  | hme-cli d
```

### Generate 5 addresses with 5s interval

```bash
hme-cli n -c 5 -i 5000
```

### Create with label and note

```bash
hme-cli n -l "newsletter" -n "personal newsletter" -c 1
```

### List with cache

```bash
hme-cli l -c
```

---

## File Structure

- ~/.config/hme-cli/cookies.txt - Stores your iCloud session
- ~/.cache/hme-cli/latest - Stores the output of the last successful list command
