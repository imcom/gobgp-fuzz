# Purpose

This project is intended to be used as a `gobgpd` gRPC fuzzing test tool.

## Usage

Currently only `remote` command is available:

```bash
Usage:
  gobgp-fuzz [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  local       A brief description of your command
  remote      Add and remove prefixes to the specified GoBGPd instance while keep dumping the global RIB
```

Example:

```bash
-> % go run main.go remote --help
This command lanuches goroutines to concurrently add, remove and dump prefixes.

Usage:
  gobgp-fuzz remote [flags]

Flags:
  -a, --addr string      gobgpd grpc addr (default "127.0.0.1:50051")
  -s, --cidrs strings    CIDR pool to generate prefixes (default [10.0.0.0/8])
  -c, --concurrent int   concurrent callers (default 4)
  -h, --help             help for remote
```

```bash
git clone git@github.com:imcom/gobgp-fuzz.git
cd gobgp-fuzz
go mod download
go run main.go remote -a 127.0.0.1:50051 -s 10.0.0.0/8 -c 2
```

The command will take the CIDR and randomly remove a range from it in order to randomly generate some prefixes for the fuzz. It then starts `concurrent` goroutines to add, remove and dump the global rib.
