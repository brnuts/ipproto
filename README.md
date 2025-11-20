# go-ipproto

Small Go library to translate between IP protocol **numbers** (0–255) and
their IANA-registered **names**, based on the official IANA CSV registry.

- Reads the IANA `protocol-numbers-1.csv`
- Handles single values (e.g. `6`) and ranges (e.g. `148-252`)
- Lets you look up:
  - number → short name (`Keyword`, e.g. `"TCP"`)
  - number → long name (`Protocol`, e.g. `"Transmission Control"`)
  - name → number (short or long)

## Install

```bash
go get github.com/youruser/go-ipproto/ipproto
