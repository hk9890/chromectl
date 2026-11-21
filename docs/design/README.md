# Design

Forward-looking specification for chromectl: what it is intended to be, and
why. How this tree relates to `docs/` is [../DOCUMENTING.md](../DOCUMENTING.md)'s.

| Document | Owns |
|---|---|
| [product.md](product.md) | What chromectl is, the job it does, and what it deliberately leaves out |
| [architecture.md](architecture.md) | The three processes, the two protocols, one host per profile |

Each fact lives in exactly one of these, or in a record under
[../decisions/](../decisions/) that they link. A document that needs a fact
another owns links to it.
