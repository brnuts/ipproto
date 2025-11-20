# ipproto

`ipproto` is a Go library that translates between **IP protocol numbers**
(0–255) and their official **IANA names**, using an embedded copy of
`protocol-numbers.csv`.

No configuration needed.  
No external files.  
Just import and use.

## Installation

```
go get github.com/brnuts/ipproto
```

## Basic Usage

### Lookup short name (Keyword) from protocol number

```go
name, ok := ipproto.LookupKeyword(6)
fmt.Println(name, ok)  // Output: TCP true
```

### Lookup long protocol name from number

```go
longName, ok := ipproto.LookupProtocolName(6)
fmt.Println(longName, ok)  // Output: Transmission Control true
```

### Lookup protocol number from short name (Keyword)

```go
num, ok := ipproto.LookupDecimal("UDP")
fmt.Println(num, ok)   // Output: 17 true
```

### Lookup number from long protocol name

```go
num, ok := ipproto.LookupDecimal("Internet Control Message")
fmt.Println(num, ok)   // Output: 1 true
```

### Flexible name matching

Keyword and Protocol names are matched case-insensitively and with whitespace normalized.

```go
num, ok := ipproto.LookupDecimal("transMission   control")
fmt.Println(num, ok)   // Output: 6 true
```

## Lookup full Entry

```go
entry, ok := ipproto.LookupByNumber(41)
if ok {
    fmt.Println("Keyword:", entry.Keyword)
    fmt.Println("Protocol:", entry.Protocol)
}
```

## Range Handling

Some IANA entries specify ranges, such as:

```
148-252   Unassigned
```

Calling:

```go
start, ok := ipproto.LookupDecimal("Unassigned")
fmt.Println(start)  // Output: 148
```

returns the start of the range.

To inspect the full range:

```go
entry, ok := ipproto.LookupByNumber(start)
fmt.Println(entry.DecimalStart, entry.DecimalEnd)
```

## Optional: Overriding the Embedded CSV

```go
err := ipproto.LoadFromFile("protocol-numbers-latest.csv")
```

or:

```go
ipproto.LoadFromReader(bytes.NewReader(customCSV))
```

## File Structure

```
protocols.go
protocol-numbers.csv
go.mod
README.md
```

## License

MIT
