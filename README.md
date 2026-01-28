# .vibe — A DSL for AI Code Generation

`.vibe` is a domain-specific language designed to orchestrate  
Claude Code CLI builds without manual prompting.

### Why `.vibe`?

Traditional AI coding workflows require:

- repeated prompt writing
- re-explaining context
- no batch execution
- no reproducibility

`.vibe` provides:

- persistent build context
- deterministic execution
- loop & condition control flow
- shell hooks via MCP
- shareable build scripts

---

### Minimal Example

```vibe
project = "calculator"
victim = web
frontend = react

task = "Create a simple calculator UI."

ask "scaffold react project"
ask "implement calculator buttons and state"
ask "style it"
```

---

### Install

```bash
go install github.com/codecravings/.vibe@latest
```

---

### Usage

```bash
.vibe run calculator.vibe
```

---

### License

MIT
