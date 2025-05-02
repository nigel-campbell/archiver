# Archiver

A simple web crawler and archive tool that can save web pages to disk with configurable depth limits. Written in haste in an attempt to capture a few pages that were victim to the purge of various online resources by the U.S. federal government earlier this year.

## Features

- Configurable crawl depth
- Concurrent crawling with multiple workers
- Option to save pages to disk
- Domain-specific crawling
- Progress tracking and statistics
- URL normalization and validation

## Installation

```bash
go install github.com/nigel-campbell/archiver/cmd/archiver@latest
```

## Usage

```bash
archiver -domain=https://example.com -depth=2 -workers=5 -persist=true -dir=archive
```

### Command Line Options

- `-domain`: Comma-separated list of domains to crawl (default: "https://example.com")
- `-depth`: Maximum depth to crawl (-1 for unlimited) (default: -1)
- `-workers`: Number of concurrent workers (default: 3)
- `-persist`: Whether to save pages to disk (default: false)
- `-dir`: Directory to save pages to (default: "archive")

### Examples

1. Crawl a single domain with depth limit:
```bash
archiver -domain=https://example.com -depth=2
```

2. Crawl multiple domains with persistence:
```bash
archiver -domain=https://example.com,https://example.org -persist=true -dir=my_archive
```

3. Crawl with more workers for faster processing:
```bash
archiver -domain=https://example.com -workers=10
```

## Output

When running the crawler, you'll see:
- Progress updates every 5 seconds showing queue size and pages crawled
- Current URL being crawled and its depth
- Final statistics including total pages crawled and saved

## Development

### Building

```bash
git clone https://github.com/nigel-campbell/archiver.git
cd archiver
go build -o archiver cmd/archiver/archiver.go
```

### Running Tests

```bash
go test ./...
```

## License

MIT License 