# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

lulu-ext is an extension library for the [lulu](https://github.com/trainking/lulu) game server framework. It provides common tools, functions, and patterns for game service development. Written in Go 1.17, module path `github.com/trainking/lulu-ext`.

## Build & Test Commands

```bash
# Build all packages
go build ./...

# Run all tests
go test ./...

# Run tests for a single package
go test ./matchx/
go test ./container/

# Run a single test
go test ./matchx/ -run TestName

# Vet for common issues
go vet ./...
```

## Architecture

### `matchx` — Matchmaking Queue

The core package. Implements an asynchronous, channel-based matchmaking system where both real players and AI bots (robots) can be grouped together.

- **`MatchQueue`** is the central component. It runs a single background goroutine (`run()`) that loops on a `select` over four channels: `matchAdd` (player joins), `matchDel` (player leaves), `closeChan` (shutdown), and a timeout ticker.
- **Real-player grouping**: When the queue length reaches `GetSuccessNum()`, `groupReal()` pulls players from the front of the queue, forms a `MatchGroup`, and sends it on `success`. If it can't fill a full group, players are pushed back onto the queue.
- **Timeout fallback**: When the `realTimeout` ticker fires, every waiting player is removed from the queue and matched with AI bots via `GroupAI()`, which calls `CallRobots(successNum - 1)` to fill the remaining slots.
- **`MatchCallback`** is the interface callers must implement — it provides the success group size (`GetSuccessNum`) and robot summoning (`CallRobots`).
- **Thread safety**: All mutations to `matching` (a `container/list`) and `matchValue` (a `map[uint64]*MatchPlayer`) happen exclusively on the `run()` goroutine. External callers interact only via `Add()`, `Del()`, `Success()`, and `Close()` which write to/read from channels.
- **Consuming results**: Call `Success()` to get the read-only success channel. The channel is buffered — drain it promptly to avoid backpressure. Sends on the success channel use `select` with `closeChan` so `Close()` can always shut down the queue even if the success buffer is full.

### `container` — Common Container Types

Currently a minimal package intended to hold reusable container data structures. No exported types yet.
