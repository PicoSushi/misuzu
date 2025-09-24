# Misuzu

Automates canary deployments on Google App Engine with configurable traffic-shifting percentages and interval controls.

## Overview

Misuzu is a command-line tool that helps you configure canary deployments for your Google App Engine services. It allows you to gradually shift traffic from a source version to a destination version with customizable parameters including step rates and intervals.

## Features

- **Gradual Traffic Shifting**: Configure step-by-step traffic migration between service versions
- **Customizable Intervals**: Set time intervals between traffic shifting steps
- **Flexible Rate Control**: Define target and step rates for precise traffic management
- **Dry Run Mode**: Preview deployment configuration without executing actual changes
- **Short Flags**: Use convenient single-character flags for faster command execution

## Installation

### Prerequisites

- Go 1.25.1 or higher

### Build from Source

```bash
git clone https://github.com/PicoSushi/misuzu.git
cd misuzu
go build -o misuzu .
```

## Usage

### Basic Command

```bash
./misuzu -project=PROJECT_ID -service=SERVICE_NAME -source-version=v1 -destination-version=v2
```

### Command Line Options

| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `-project` | `-p` | ✅ | - | Google Cloud Project ID |
| `-service` | `-s` | ✅ | - | App Engine service name |
| `-source-version` | `-f` | ✅ | - | Current version to migrate from |
| `-destination-version` | `-t` | ✅ | - | Target version to migrate to |
| `-interval` | `-i` | ❌ | 300 | Interval in seconds between traffic shifts |
| `-target-rate` | `-r` | ❌ | 100 | Target percentage of traffic for destination version |
| `-step-rate` | `-e` | ❌ | 10 | Percentage of traffic to shift per step (1-100) |
| `-dry-run` | `-d` | ❌ | false | Show what would be done without actually performing deployment |

### Examples

#### Basic Canary Deployment

```bash
./misuzu -project=my-project -service=default -source-version=v1 -destination-version=v2
```

#### Custom Step Rate and Interval

```bash
./misuzu \
  -project=my-project \
  -service=api \
  -source-version=v1.0.0 \
  -destination-version=v1.1.0 \
  -step-rate=5 \
  -interval=600
```

#### Partial Traffic Migration

```bash
./misuzu \
  -project=my-project \
  -service=frontend \
  -source-version=stable \
  -destination-version=beta \
  -target-rate=50 \
  -step-rate=25
```

## Configuration Validation

Misuzu validates all input parameters and provides clear error messages:

- **Required fields**: project, service, source-version, destination-version
- **Step rate range**: Must be between 1 and 100 (inclusive)
- **Clear error reporting**: Specific validation errors with field names

## Output

The tool displays a formatted configuration summary:

```text
=== Misuzu Canary Deployment Configuration ===
Project ID: my-project
Service Name: default
Source Version: v1
Destination Version: v2
Interval (seconds): 300
Target Rate (%): 100
Step Rate (%): 10
Dry Run: false
=== Configuration Complete ===
```

When dry-run mode is enabled, additional warning is displayed:

```text
=== Misuzu Canary Deployment Configuration ===
Project ID: my-project
Service Name: default
Source Version: v1
Destination Version: v2
Interval (seconds): 300
Target Rate (%): 100
Step Rate (%): 10
Dry Run: true
*** DRY RUN MODE - No actual deployment will be performed ***
=== Configuration Complete ===
```

## Development

### Requirements

- Go 1.25.1+
- [mise](https://mise.jdx.dev/) (optional, for version management)

### Setup

```bash
# Using mise (recommended)
mise install

# Or install Go manually
go version  # Should be 1.25.1+
```

### Building

```bash
go build
```

### Testing

```bash
go test ./...
```

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.

## Author

PicoSushi
