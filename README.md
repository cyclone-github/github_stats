[![Readme Card](https://github-readme-stats.vercel.app/api/pin/?username=cyclone-github&repo=github_stats&theme=gruvbox)](https://github.com/cyclone-github/github_stats/)

[![Go Report Card](https://goreportcard.com/badge/github.com/cyclone-github/github_stats)](https://goreportcard.com/report/github.com/cyclone-github/github_stats)
[![GitHub issues](https://img.shields.io/github/issues/cyclone-github/github_stats.svg)](https://github.com/cyclone-github/github_stats/issues)
[![License](https://img.shields.io/github/license/cyclone-github/github_stats.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/release/cyclone-github/github_stats.svg)](https://github.com/cyclone-github/github_stats/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/cyclone-github/github_stats.svg)](https://pkg.go.dev/github.com/cyclone-github/github_stats)

# Cyclone's GitHub Stats
![image](https://i.imgur.com/4pBNN6K.png)
### Features:
- Tool to retrieve GitHub stats for your favorite user repositories
- Add users for quickly recalling favorite GitHub repositories 
- Option to remove users you no longer wish to keep track of
- Stores user configuration in 'config.json'
### Example Usage:
- ./github_stats.bin
- Follow Prompts

### Version:
- v0.3.0-2023-10-13; fixed watchers; added cache and ratelimiting support

### Compile from source:
- If you want the latest features, compiling from source is the best option since the release version may run several revisions behind the source code.
- This assumes you have Go and Git installed
  - `git clone https://github.com/cyclone-github/github_stats.git`
  - `cd github_stats`
  - `go mod init github_stats`
  - `go mod tidy`
  - `go build -ldflags="-s -w" .`
- Compile from source code how-to:
  - https://github.com/cyclone-github/scripts/blob/main/intro_to_go.txt
