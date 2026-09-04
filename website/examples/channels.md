---
title: Send and receive through a channel
description: Use a buffered Go channel, checked receive, close, and the zero value after close.
---

# Send and receive through a channel

This deterministic recipe shows the fundamental channel states without relying on scheduler order.

## Project tree

```text
channels/
└── main.km
```

## Source

<<< ../snippets/channels.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
21 true 42 0 false
```

The buffered sends complete before a receiver starts. The first checked receive returns `21, true`; the next receive returns `42`. After closing and draining the channel, checked receive returns the `int` zero value and `false`.

## Boundary behavior

- Sending or closing a closed channel panics as in Go.
- Receiving from a closed, drained channel returns the zero value immediately.
- A nil channel blocks for send/receive; closing it panics.
- Only a bidirectional or send-capable channel can be closed.
- `select` preserves Go readiness and nondeterministic selection among ready cases.

Use `GoSendChannel<T>` and `GoReceiveChannel<T>` in signatures when direction is part of the API contract.
