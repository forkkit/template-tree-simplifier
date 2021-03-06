# template-tree-simplifier

[![GoDoc](https://godoc.org/github.com/mh-cbon/template-tree-simplifier/simplifier?status.svg)](https://godoc.org/github.com/mh-cbon/template-tree-simplifier/simplifier)

A package to simplify a template AST via a serie of transformations.

It transforms

```
{{"some" | split (("what" | lower) | up)}}
```

into
```
{{$var1 := lower "what"}}{{$var0 := up $var1}}{{split $var0 "some"}}
```

## Install

```sh
go get github.com/mh-cbon/template-tree-simplifier
glide get github.com/mh-cbon/template-tree-simplifier
```

# Usage

```go
package main

import (
	"fmt"
	"github.com/mh-cbon/template-tree-simplifier/simplifier"
	"text/template"
)

func main() {
	file := "cli/tpl/test.tpl"
	fmt.Println(file)

	data := nil
	funcs := template.FuncMap{}

	tpl, err := template.New("").Funcs(funcs).ParseFiles(file)
	if err != nil {
		panic(err)
	}

	simplifier.Transform(t, data, funcs)
}

```
