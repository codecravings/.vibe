# .vibe

> Because typing code is so 2024.

A DSL that mass instructions Claude to build your entire project while you pretend to be productive.

## Install

```bash
go install github.com/codecravings/.vibe@latest
```

Or download a binary from [Releases](https://github.com/codecravings/.vibe/releases).

## Usage

```bash
.vibe myproject.vibe
```

## Example

```vibe
project = "startup-idea-69"
victim = web-fullstack
frontend = react
backend = express
db = mongodb

task = "Build me a billion dollar app idk"

ask "scaffold everything"
ask "make it work"
ask "make it pretty"

if chaos-level > 9000 {
    ask "add blockchain"
}
```

## What it does

1. You write vibes
2. It yells at Claude
3. Code appears
4. You mass ship it

## Options

```
--dry-run      See what would happen (for cowards)
--model haiku  Faster but dumber
--interactive  Enable permission prompts (why tho)
```

## License

MIT - do whatever, just mass ship

---

*"I mass coded the code that codes the code"* - mass shipping person
