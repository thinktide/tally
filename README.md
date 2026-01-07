# tally

A fast, simple CLI time tracking utility.

## Features

- **Simple commands** — start, stop, pause, resume
- **Projects & tags** — organize with `@project` and `+tags`
- **Reports** — daily, weekly, monthly summaries
- **Offline-first** — all data stored locally in SQLite
- **Multiple output formats** — table, JSON, CSV

## Installation

### Homebrew

```bash
brew tap thinktide/tally
brew install tally
```

### From source

```bash
go install github.com/thinktide/tally/cmd/tally@latest
```

### Build manually

```bash
git clone https://github.com/thinktide/tally.git
cd tally
make build
./bin/tally
```

## Usage

### Start tracking

```bash
tally start @work "Implementing feature" +backend +api
```

- `@work` — project (required)
- `"Implementing feature"` — description (optional)
- `+backend +api` — tags (optional)

### Stop tracking

```bash
tally stop
```

### Check status

```bash
tally status
```

### Pause and resume

```bash
tally pause                      # Pause now
tally pause -f 09:00             # Record pause from 9am to now
tally pause -f 09:00 -t 10:30    # Record pause from 9am to 10:30am

tally resume                     # Resume paused timer, or reopen stopped entry
```

When resuming a stopped entry, tally shows the entry details and asks for confirmation. A pause is created for the gap between the stop time and now.

### View log

```bash
tally log                    # Last 10 entries
tally log -n 20              # Last 20 entries
tally log @work              # Filter by project
tally log +backend           # Filter by tag
tally log --from 2024-01-01  # Filter by date
```

### Edit an entry

```bash
tally edit                   # Edit most recent
tally edit 01ABC123...       # Edit by ID
```

Opens the entry as JSON in `$EDITOR` (defaults to vim). You can:
- Edit project, title, tags, start/end times
- Add new pauses (leave `id` empty)
- Modify existing pause times
- Remove pauses by deleting them from the array

Each pause has a `reason` field: "Manual", "Display off", or "System sleep".

### Delete an entry

```bash
tally delete                 # Delete most recent (with confirmation)
tally delete 01ABC123...     # Delete by ID
tally delete -f              # Skip confirmation
```

### Reports

```bash
tally report                 # Interactive period selection
tally report today
tally report yesterday
tally report week
tally report lastWeek
tally report month
tally report lastMonth
tally report year
tally report lastYear

# With filters
tally report week @work +backend

# Output formats
tally report today --format json
tally report today --format csv
```

### Configuration

```bash
tally config list                              # Show all settings
tally config get output.format                 # Get a value
tally config set output.format json            # Set a value
```

**Available settings:**

| Key | Values | Default | Description |
|-----|--------|---------|-------------|
| `output.format` | table, json, csv | table | Default report format |
| `data.location` | path | ~/.tally | Data directory |

## Data Storage

All data is stored locally in `~/.tally/tally.db` (SQLite).

To reset all data:

```bash
rm -rf ~/.tally
```

To backup:

```bash
cp ~/.tally/tally.db ~/tally-backup.db
```

## License

MIT
