# Troubleshooting

**"command not found"** — `cf` is not installed. Install via any of:
```bash
npm install -g confluence-cf                       # npm
pip install confluence-cf                          # pip
brew install sofq/tap/cf                           # Homebrew (macOS/Linux)
scoop bucket add sofq https://github.com/sofq/scoop-bucket && scoop install cf  # Scoop (Windows)
go install github.com/sofq/confluence-cli@latest   # Go
```
Prebuilt binaries are also available at https://github.com/sofq/confluence-cli/releases.

**Exit code 2 (auth)** — Token expired or misconfigured. Test with:
```bash
cf configure --test
```

**Unknown command** — Command names are auto-generated from Confluence's API. Use `cf schema` to find the right name, or use `cf raw` as an escape hatch.

**Large responses filling context** — Always use `--preset` or `--fields` + `--jq` to minimize output.

**Content format** — Confluence uses Atlassian Storage Format (XHTML-based, not Markdown). Pass storage format through as-is. Example: `<h1>Title</h1><p>Content</p>`

**"--dry-run"** — Use this to preview what `cf` will send without making the API call.
